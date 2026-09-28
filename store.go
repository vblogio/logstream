package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

// Store sends messages to VictoriaLogs in batches and queries it with LogsQL.
type Store struct {
	base         string
	client       *http.Client
	streamClient *http.Client
	in         chan *Entry
	batchSize  int
	flushEvery time.Duration

	received atomic.Int64
	ingested atomic.Int64
	dropped  atomic.Int64
	lastErr  atomic.Value // string
}

func NewStore(base string, batchSize, queueSize int, flushEvery time.Duration) *Store {
	s := &Store{
		base:       strings.TrimRight(base, "/"),
		client:     &http.Client{Timeout: 60 * time.Second},
		// No global timeout: a large export can take longer than a minute. The
		// request context still cancels it when the browser goes away.
		streamClient: &http.Client{},
		in:         make(chan *Entry, queueSize),
		batchSize:  batchSize,
		flushEvery: flushEvery,
	}
	s.lastErr.Store("")
	return s
}

// Enqueue never blocks: when the queue is full, the message is counted as dropped.
func (s *Store) Enqueue(e *Entry) {
	s.received.Add(1)
	select {
	case s.in <- e:
	default:
		s.dropped.Add(1)
	}
}

// Run drains the queue into VictoriaLogs until the context is cancelled,
// then flushes whatever is left.
func (s *Store) Run(ctx context.Context) {
	ticker := time.NewTicker(s.flushEvery)
	defer ticker.Stop()
	batch := make([]*Entry, 0, s.batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := s.insert(batch); err != nil {
			s.dropped.Add(int64(len(batch)))
			s.lastErr.Store(err.Error())
			log.Printf("victorialogs: %d messages dropped: %v", len(batch), err)
		} else {
			s.ingested.Add(int64(len(batch)))
			s.lastErr.Store("")
		}
		batch = batch[:0]
	}

	for {
		select {
		case e := <-s.in:
			batch = append(batch, e)
			if len(batch) >= s.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-ctx.Done():
			for {
				select {
				case e := <-s.in:
					batch = append(batch, e)
					if len(batch) >= s.batchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}
		}
	}
}

func (s *Store) insert(batch []*Entry) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	for _, e := range batch {
		if err := enc.Encode(e.Record()); err != nil {
			return err
		}
	}
	u := s.base + "/insert/jsonline?_stream_fields=host,app&_msg_field=_msg&_time_field=_time"

	var err error
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<attempt) * 500 * time.Millisecond) // 1s, 2s, 4s, 8s
		}
		if err = s.post(u, buf.Bytes()); err == nil {
			return nil
		}
	}
	return err
}

func (s *Store) post(u string, body []byte) error {
	resp, err := s.client.Post(u, "application/stream+json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// Query runs a LogsQL query and returns one row per result.
func (s *Store) Query(ctx context.Context, query string) ([]map[string]any, error) {
	rows := []map[string]any{}
	err := s.QueryStream(ctx, query, func(row map[string]any) error {
		rows = append(rows, row)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// QueryStream runs a LogsQL query and calls fn for each result as it arrives,
// without keeping the results in memory. fn is only called once VictoriaLogs
// has accepted the query; an error returned by fn stops the stream.
func (s *Store) QueryStream(ctx context.Context, query string, fn func(row map[string]any) error) error {
	form := url.Values{"query": {query}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.base+"/select/logsql/query", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.streamClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return &queryError{status: resp.StatusCode, msg: strings.TrimSpace(string(msg))}
	}

	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64*1024), 16<<20)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal(line, &row); err != nil {
			return err
		}
		if err := fn(row); err != nil {
			return err
		}
	}
	return sc.Err()
}

// Purge starts a VictoriaLogs task that deletes every stored log, and resets
// the counters. VictoriaLogs must run with -delete.enable.
func (s *Store) Purge(ctx context.Context) (string, error) {
	body, err := s.deleteCall(ctx, "/delete/run_task?filter="+url.QueryEscape("*"))
	if err != nil {
		return "", err
	}
	var res struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", fmt.Errorf("unexpected VictoriaLogs answer: %s", strings.TrimSpace(string(body)))
	}
	s.received.Store(0)
	s.ingested.Store(0)
	s.dropped.Store(0)
	s.lastErr.Store("")
	return res.TaskID, nil
}

// PurgeRunning returns the number of deletion tasks still running in VictoriaLogs.
func (s *Store) PurgeRunning(ctx context.Context) (int, error) {
	body, err := s.deleteCall(ctx, "/delete/active_tasks")
	if err != nil {
		return 0, err
	}
	var tasks []json.RawMessage
	if err := json.Unmarshal(body, &tasks); err != nil {
		return 0, fmt.Errorf("unexpected VictoriaLogs answer: %s", strings.TrimSpace(string(body)))
	}
	return len(tasks), nil
}

func (s *Store) deleteCall(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.base+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(body))
		if resp.StatusCode == http.StatusNotFound || strings.Contains(msg, "delete.enable") {
			return nil, &codedError{code: "purge_unavailable", msg: "VictoriaLogs refuses deletions: start it with -delete.enable (see docker-compose.yml)", detail: msg}
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, msg)
	}
	return body, nil
}

// queryError is an error answer of VictoriaLogs to a query.
type queryError struct {
	status int
	msg    string
}

func (e *queryError) Error() string { return e.msg }

// queryStatus is the HTTP status to return for a failed query: 400 when
// VictoriaLogs rejected the query itself (bad LogsQL), 502 otherwise.
func queryStatus(err error) int {
	var qe *queryError
	if errors.As(err, &qe) && qe.status >= 400 && qe.status < 500 {
		return http.StatusBadRequest
	}
	return http.StatusBadGateway
}

func (s *Store) Stats() map[string]any {
	return map[string]any{
		"received":  s.received.Load(),
		"ingested":  s.ingested.Load(),
		"dropped":   s.dropped.Load(),
		"queue":     len(s.in),
		"lastError": s.lastErr.Load(),
	}
}
