package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// insertVL plays VictoriaLogs' insert endpoint: it answers `status` (0 = the
// connection fails) and keeps the lines it accepted.
type insertVL struct {
	mu     sync.Mutex
	status int
	lines  []string
}

func (v *insertVL) set(status int) {
	v.mu.Lock()
	v.status = status
	v.mu.Unlock()
}

func (v *insertVL) got() []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return append([]string(nil), v.lines...)
}

func (v *insertVL) RoundTrip(r *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(r.Body)
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.status == 0 {
		return nil, errors.New("connection refused")
	}
	if v.status == http.StatusOK {
		for _, l := range strings.Split(strings.TrimSpace(string(body)), "\n") {
			v.lines = append(v.lines, l)
		}
	}
	return &http.Response{StatusCode: v.status, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
}

func testEntry(msg string, done *atomic.Int64) *Entry {
	e := &Entry{Received: time.Now(), Time: time.Now(), Host: "h", App: "a", Message: msg}
	if done != nil {
		e.Done = func() { done.Add(1) }
	}
	return e
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestSpoolFiles(t *testing.T) {
	dir := t.TempDir()
	sp, err := OpenSpool(dir, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := sp.Write([]byte("a\nb\n"), 2); err != nil {
		t.Fatal(err)
	}
	if err := sp.Write([]byte("c\n"), 1); err != nil {
		t.Fatal(err)
	}
	if err := sp.Write(bytes.Repeat([]byte("x"), 100), 1); !errors.Is(err, errSpoolFull) {
		t.Fatalf("write over the limit: %v, want errSpoolFull", err)
	}
	_ = os.WriteFile(filepath.Join(dir, "broken.ndjson.tmp"), []byte("z"), 0o644)

	// A new run finds the batches left on disk and drops unfinished writes.
	sp, err = OpenSpool(dir, 100)
	if err != nil {
		t.Fatal(err)
	}
	if n, size := sp.Pending(); n != 3 || size != 6 {
		t.Fatalf("pending %d lines %d bytes, want 3 and 6", n, size)
	}
	f, body, ok, err := sp.Oldest()
	if !ok || err != nil || string(body) != "a\nb\n" || f.lines != 2 {
		t.Fatalf("oldest: %q %+v %v %v", body, f, ok, err)
	}
	sp.Remove(f)
	if _, body, _, _ := sp.Oldest(); string(body) != "c\n" {
		t.Fatalf("next oldest %q", body)
	}
	if _, err := os.Stat(filepath.Join(dir, "broken.ndjson.tmp")); !os.IsNotExist(err) {
		t.Error("unfinished write left in the spool")
	}
}

func TestStoreSpoolsWhileVictoriaLogsIsDown(t *testing.T) {
	sp, err := OpenSpool(t.TempDir(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	vl := &insertVL{status: 0}
	s := NewStore("http://vl", 2, 100, 20*time.Millisecond, sp)
	s.client.Transport = vl
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)

	var done atomic.Int64
	for _, m := range []string{"one", "two", "three"} {
		s.Enqueue(testEntry(m, &done))
	}
	// VictoriaLogs is down: the messages are kept on disk, and acknowledged.
	waitFor(t, "3 spooled messages", func() bool { n, _ := sp.Pending(); return n == 3 })
	if done.Load() != 3 || s.dropped.Load() != 0 {
		t.Fatalf("done %d dropped %d, want 3 and 0", done.Load(), s.dropped.Load())
	}
	// While batches wait on disk, new ones queue behind them.
	vl.set(http.StatusServiceUnavailable)
	s.Enqueue(testEntry("four", &done))
	waitFor(t, "4 spooled messages", func() bool { n, _ := sp.Pending(); return n == 4 })

	vl.set(http.StatusOK)
	waitFor(t, "the spool to drain", func() bool { n, _ := sp.Pending(); return n == 0 })
	got := strings.Join(vl.got(), "\n")
	for i, m := range []string{"one", "two", "three", "four"} {
		if !strings.Contains(got, `"_msg":"`+m+`"`) {
			t.Errorf("message %d %q not sent: %s", i, m, got)
		}
	}
	if strings.Index(got, `"one"`) > strings.Index(got, `"four"`) {
		t.Error("spooled batches sent out of order")
	}
	if s.ingested.Load() != 4 || s.lastErr.Load() != "" {
		t.Errorf("ingested %d lastErr %q", s.ingested.Load(), s.lastErr.Load())
	}

	// Once the spool is empty, batches go straight to VictoriaLogs again.
	s.Enqueue(testEntry("five", &done))
	waitFor(t, "a direct insert", func() bool { return s.ingested.Load() == 5 })
}

func TestStoreDropsRefusedSpooledBatch(t *testing.T) {
	sp, _ := OpenSpool(t.TempDir(), 1<<20)
	_ = sp.Write([]byte("{bad json\n"), 1)
	vl := &insertVL{status: http.StatusBadRequest}
	s := NewStore("http://vl", 10, 10, time.Second, sp)
	s.client.Transport = vl
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)
	waitFor(t, "the refused batch to be dropped", func() bool { return s.dropped.Load() == 1 })
	if n, _ := sp.Pending(); n != 0 {
		t.Errorf("%d messages left in the spool", n)
	}
}

func TestStoreWithoutSpoolDrops(t *testing.T) {
	vl := &insertVL{status: 0}
	s := NewStore("http://vl", 10, 10, time.Hour, nil)
	s.client.Transport = vl
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // shutdown: one attempt, no retry pauses
	var done atomic.Int64
	s.store(ctx, []*Entry{testEntry("x", &done)})
	if done.Load() != 0 || s.dropped.Load() != 1 {
		t.Errorf("done %d dropped %d, want 0 and 1", done.Load(), s.dropped.Load())
	}
}

func TestEnqueueWait(t *testing.T) {
	s := NewStore("http://vl", 10, 1, time.Hour, nil)
	s.Enqueue(testEntry("fills the queue", nil))
	s.Enqueue(testEntry("syslog: dropped", nil))
	if s.dropped.Load() != 1 {
		t.Fatalf("dropped %d, want 1", s.dropped.Load())
	}
	// A producer that can wait blocks until there is room…
	queued := make(chan struct{})
	go func() {
		e := testEntry("docker: waits", nil)
		e.Wait = true
		s.Enqueue(e)
		close(queued)
	}()
	select {
	case <-queued:
		t.Fatal("did not wait for room in the queue")
	case <-time.After(50 * time.Millisecond):
	}
	<-s.in
	<-queued
	if s.dropped.Load() != 1 {
		t.Errorf("dropped %d, want 1", s.dropped.Load())
	}
	// …or until shutdown.
	close(s.quit)
	e := testEntry("docker: shutdown", nil)
	e.Wait = true
	s.Enqueue(e)
	if s.dropped.Load() != 2 {
		t.Errorf("dropped %d after shutdown, want 2", s.dropped.Load())
	}
}
