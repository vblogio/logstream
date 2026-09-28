package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

// Store envoie les messages à VictoriaLogs par lots et l'interroge en LogsQL.
type Store struct {
	base       string
	client     *http.Client
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
		in:         make(chan *Entry, queueSize),
		batchSize:  batchSize,
		flushEvery: flushEvery,
	}
	s.lastErr.Store("")
	return s
}

// Enqueue ne bloque jamais : si la file est pleine, le message est compté comme perdu.
func (s *Store) Enqueue(e *Entry) {
	s.received.Add(1)
	select {
	case s.in <- e:
	default:
		s.dropped.Add(1)
	}
}

// Run vide la file vers VictoriaLogs jusqu'à l'annulation du contexte,
// puis envoie ce qui reste.
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
			log.Printf("victorialogs : %d messages perdus : %v", len(batch), err)
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
		return fmt.Errorf("HTTP %d : %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// Query exécute une requête LogsQL et renvoie une ligne par résultat.
func (s *Store) Query(ctx context.Context, query string) ([]map[string]any, error) {
	form := url.Values{"query": {query}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.base+"/select/logsql/query", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("%s", strings.TrimSpace(string(msg)))
	}

	rows := []map[string]any{}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64*1024), 16<<20)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal(line, &row); err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, sc.Err()
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
