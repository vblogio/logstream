package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

// Store sends messages to VictoriaLogs in batches and queries it with LogsQL.
// Batches VictoriaLogs cannot take go to the disk spool (when enabled) and
// are sent again, oldest first, once it answers.
type Store struct {
	base         string
	client       *http.Client
	streamClient *http.Client
	in           chan *Entry
	quit         chan struct{} // closed on shutdown: unblocks waiting producers
	batchSize    int
	flushEvery   time.Duration
	spool        *Spool        // nil: no disk buffer
	spooled      chan struct{} // wakes the replay loop up after a write to the spool

	received atomic.Int64
	ingested atomic.Int64
	dropped  atomic.Int64
	lastErr  atomic.Value // string
	failing  atomic.Bool  // VictoriaLogs refused or missed the last send
}

func NewStore(base string, batchSize, queueSize int, flushEvery time.Duration, spool *Spool) *Store {
	s := &Store{
		base:   strings.TrimRight(base, "/"),
		client: &http.Client{Timeout: 60 * time.Second},
		// No global timeout: a large export can take longer than a minute. The
		// request context still cancels it when the browser goes away.
		streamClient: &http.Client{},
		in:           make(chan *Entry, queueSize),
		quit:         make(chan struct{}),
		batchSize:    batchSize,
		flushEvery:   flushEvery,
		spool:        spool,
		spooled:      make(chan struct{}, 1),
	}
	s.lastErr.Store("")
	return s
}

// Enqueue adds a message to the queue. When the queue is full, a message
// whose producer can wait (Entry.Wait: Docker, host logs) blocks until there
// is room; any other one (syslog) is counted as dropped.
func (s *Store) Enqueue(e *Entry) {
	s.received.Add(1)
	select {
	case s.in <- e:
		return
	default:
	}
	if e.Wait {
		select {
		case s.in <- e:
			return
		case <-s.quit:
		}
	}
	s.dropped.Add(1)
}

// Run drains the queue into VictoriaLogs until the context is cancelled,
// then flushes whatever is left.
func (s *Store) Run(ctx context.Context) {
	ticker := time.NewTicker(s.flushEvery)
	defer ticker.Stop()
	batch := make([]*Entry, 0, s.batchSize)
	if s.spool != nil {
		go s.replay(ctx)
	}

	flush := func(ctx context.Context) {
		if len(batch) > 0 {
			s.store(ctx, batch)
			clear(batch)
			batch = batch[:0]
		}
	}

	for {
		select {
		case e := <-s.in:
			batch = append(batch, e)
			if len(batch) >= s.batchSize {
				flush(ctx)
			}
		case <-ticker.C:
			flush(ctx)
		case <-ctx.Done():
			close(s.quit)
			// The last batches get one short attempt, then go to the spool.
			end, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			for {
				select {
				case e := <-s.in:
					batch = append(batch, e)
					if len(batch) >= s.batchSize {
						flush(end)
					}
				default:
					flush(end)
					return
				}
			}
		}
	}
}

// store sends one batch to VictoriaLogs, or to the spool when VictoriaLogs
// fails or older batches are still waiting there (to keep their order).
// Entry.Done is called once the batch is stored or spooled.
func (s *Store) store(ctx context.Context, batch []*Entry) {
	body, err := encodeBatch(batch)
	if err == nil {
		err = s.save(ctx, body, len(batch))
	}
	if err != nil {
		s.dropped.Add(int64(len(batch)))
		s.lastErr.Store(err.Error())
		logEvent("store", 3, "%d messages lost: %v", len(batch), err)
		return
	}
	for _, e := range batch {
		if e.Done != nil {
			e.Done()
		}
	}
}

func (s *Store) save(ctx context.Context, body []byte, lines int) error {
	if s.spool == nil {
		// No disk buffer: retry a few times, then give up.
		var err error
		for attempt := 0; attempt < 5; attempt++ {
			if attempt > 0 && !sleepCtx(ctx, time.Duration(1<<attempt)*500*time.Millisecond) { // 1s, 2s, 4s, 8s
				break
			}
			if err = s.post(ctx, body); err == nil {
				s.ingested.Add(int64(lines))
				s.sent()
				return nil
			}
			s.sendFailed(err)
		}
		return err
	}

	var postErr error
	if waiting, _ := s.spool.Pending(); waiting == 0 {
		if postErr = s.post(ctx, body); postErr == nil {
			s.ingested.Add(int64(lines))
			s.sent()
			return nil
		}
		s.sendFailed(postErr)
	}
	err := s.spool.Write(body, lines)
	if err == nil {
		select {
		case s.spooled <- struct{}{}:
		default:
		}
		return nil
	}
	if postErr != nil {
		return fmt.Errorf("%v; %v", postErr, err)
	}
	// Spool full while older batches wait: one direct attempt before dropping.
	if postErr = s.post(ctx, body); postErr == nil {
		s.ingested.Add(int64(lines))
		return nil
	}
	return fmt.Errorf("%v; %v", err, postErr)
}

// replay sends the spooled batches back to VictoriaLogs, oldest first, with
// an increasing pause (up to 30 s) while it keeps failing.
func (s *Store) replay(ctx context.Context) {
	if n, size := s.spool.Pending(); n > 0 {
		logEvent("store", 5, "disk spool: %d messages (%d bytes) waiting from a previous run", n, size)
	}
	backoff := time.Second
	for {
		f, body, ok, err := s.spool.Oldest()
		if err != nil {
			logEvent("store", 4, "disk spool: %v", err)
		}
		if !ok {
			select {
			case <-ctx.Done():
				return
			case <-s.spooled:
			case <-time.After(time.Minute):
			}
			continue
		}
		err = s.post(ctx, body)
		switch {
		case err == nil:
			s.spool.Remove(f)
			s.ingested.Add(f.lines)
			s.sent()
			backoff = time.Second
			continue
		case isRejected(err):
			// VictoriaLogs refuses the data itself: sending it again would not help.
			s.spool.Remove(f)
			s.dropped.Add(f.lines)
			logEvent("store", 3, "disk spool: %d messages refused by VictoriaLogs and lost: %v", f.lines, err)
			continue
		}
		s.sendFailed(err)
		if !sleepCtx(ctx, backoff) {
			return
		}
		backoff = min(2*backoff, 30*time.Second)
	}
}

// sendFailed and sent report when VictoriaLogs stops and starts taking the
// batches again, once per change.
func (s *Store) sendFailed(err error) {
	s.lastErr.Store(err.Error())
	if !s.failing.Swap(true) {
		if s.spool != nil {
			logEvent("store", 4, "VictoriaLogs unreachable, batches are kept in the disk spool: %v", err)
		} else {
			logEvent("store", 4, "VictoriaLogs unreachable, retrying: %v", err)
		}
	}
}

func (s *Store) sent() {
	s.lastErr.Store("")
	if s.failing.Swap(false) {
		logEvent("store", 5, "VictoriaLogs reachable again")
	}
}

func encodeBatch(batch []*Entry) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	for _, e := range batch {
		if err := enc.Encode(e.Record()); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// insertError is an error status of VictoriaLogs to an insert.
type insertError struct {
	status int
	msg    string
}

func (e *insertError) Error() string { return fmt.Sprintf("HTTP %d: %s", e.status, e.msg) }

// isRejected tells whether VictoriaLogs refused the data itself (4xx other
// than 429), as opposed to being unreachable or overloaded.
func isRejected(err error) bool {
	var ie *insertError
	return errors.As(err, &ie) && ie.status >= 400 && ie.status < 500 && ie.status != http.StatusTooManyRequests
}

func (s *Store) post(ctx context.Context, body []byte) error {
	u := s.base + "/insert/jsonline?_stream_fields=host,app&_msg_field=_msg&_time_field=_time"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/stream+json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return &insertError{status: resp.StatusCode, msg: strings.TrimSpace(string(msg))}
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
	st := map[string]any{
		"received":  s.received.Load(),
		"ingested":  s.ingested.Load(),
		"dropped":   s.dropped.Load(),
		"queue":     len(s.in),
		"lastError": s.lastErr.Load(),
	}
	if s.spool != nil {
		st["spooled"], st["spoolBytes"] = s.spool.Pending()
	}
	return st
}
