package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSelfLogEmit(t *testing.T) {
	now := time.Unix(1759572000, 0)
	tests := []struct {
		name   string
		cfg    selfLogConfig
		events []int // severities, one second apart, same message format
		want   int   // entries queued
		suffix string
	}{
		{"warning and above", selfLogConfig{Enabled: true, MinLevel: "warning"}, []int{3}, 1, ""},
		{"below the level", selfLogConfig{Enabled: true, MinLevel: "warning"}, []int{5, 6}, 0, ""},
		{"info level", selfLogConfig{Enabled: true, MinLevel: "info"}, []int{6}, 1, ""},
		{"disabled", selfLogConfig{Enabled: false, MinLevel: "debug"}, []int{0}, 0, ""},
		{"bad level falls back to warning", normSelfLogConfig(selfLogConfig{Enabled: true, MinLevel: "loud"}), []int{4, 5}, 1, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSelfLog(t.TempDir(), func(*Entry) {})
			s.cfg = tt.cfg
			for i, sev := range tt.events {
				s.Emit("store", sev, "fmt %d", "msg", now.Add(time.Duration(i)*time.Second))
			}
			if len(s.queue) != tt.want {
				t.Fatalf("%d queued, want %d", len(s.queue), tt.want)
			}
		})
	}

	// Same component and format: once per interval, then a count of the others.
	s := NewSelfLog(t.TempDir(), func(*Entry) {})
	for i := 0; i < 5; i++ {
		s.Emit("store", 3, "%d messages lost", "x", now.Add(time.Duration(i)*time.Second))
	}
	s.Emit("syslog", 3, "%d messages lost", "y", now) // another component
	s.Emit("store", 3, "%d messages lost", "z", now.Add(s.interval))
	if len(s.queue) != 3 {
		t.Fatalf("%d queued", len(s.queue))
	}
	<-s.queue
	if e := <-s.queue; e.App != "syslog" || e.Host != "logstream" || e.SourceType != "logstream" || e.Wait {
		t.Fatalf("entry %+v", e)
	}
	if e := <-s.queue; e.Message != "z (4 similar events not shown)" || e.Severity != "err" {
		t.Fatalf("message %q", e.Message)
	}
	if st := s.Status(); st["throttled"] != uint64(4) {
		t.Fatalf("status %v", st)
	}

	// A full queue drops events instead of blocking.
	s = NewSelfLog(t.TempDir(), func(*Entry) {})
	for i := 0; i < cap(s.queue)+10; i++ {
		s.Emit("store", 3, "event %d", "x", now.Add(time.Duration(i)*s.interval))
	}
	if st := s.Status(); st["dropped"] != uint64(10) {
		t.Fatalf("status %v", st)
	}
}

func TestSelfLogRunAndConfigure(t *testing.T) {
	dir := t.TempDir()
	sink := &entrySink{}
	s := NewSelfLog(dir, sink.add)
	if err := s.Configure(selfLogConfig{Enabled: true, MinLevel: "notice"}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "selflog.json")); !strings.Contains(string(b), `"notice"`) {
		t.Fatalf("saved %s", b)
	}
	if s2 := NewSelfLog(dir, nil); s2.cfg.MinLevel != "notice" {
		t.Fatalf("reloaded %+v", s2.cfg)
	}
	old := selfLog
	selfLog = s
	defer func() { selfLog = old }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	logEvent("main", 5, "LogStream started")
	logEvent("main", 6, "too verbose")
	cancel()
	<-done
	logEvent("main", 5, "LogStream stopping") // queued after Run: not lost if Run drains it
	sink.mu.Lock()
	n := len(sink.es)
	sink.mu.Unlock()
	if n != 1 || s.Status()["emitted"] != uint64(1) {
		t.Fatalf("%d entries, status %v", n, s.Status())
	}
}
