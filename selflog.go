package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// selfLogConfig is saved in /data/selflog.json and edited in Settings > Sources.
type selfLogConfig struct {
	Enabled  bool   `json:"enabled"`
	MinLevel string `json:"min_level"` // severity name: "err", "warning", "notice", "info"…
}

// SelfLog turns the internal events of LogStream (storage errors, listener
// failures, sources that stop…) into log entries, with host=logstream and
// app=component. Events go through a bounded queue and a per-message
// throttle: an event about a failing Store is itself stored through that
// Store, so it must never be able to loop or pile up.
type SelfLog struct {
	cfgPath  string
	sink     func(*Entry)
	queue    chan *Entry
	interval time.Duration // one event per component and message format per interval

	mu         sync.Mutex
	cfg        selfLogConfig
	last       map[string]time.Time
	suppressed map[string]int
	emitted    uint64
	dropped    uint64
	throttled  uint64
}

// selfLog receives the events of logEvent; nil (tests) only writes them to
// the process log.
var selfLog *SelfLog

func NewSelfLog(dataDir string, sink func(*Entry)) *SelfLog {
	s := &SelfLog{
		cfgPath:    filepath.Join(dataDir, "selflog.json"),
		sink:       sink,
		queue:      make(chan *Entry, 256),
		interval:   30 * time.Second,
		cfg:        selfLogConfig{Enabled: true, MinLevel: "warning"},
		last:       map[string]time.Time{},
		suppressed: map[string]int{},
	}
	if b, err := os.ReadFile(s.cfgPath); err == nil {
		var cfg selfLogConfig
		if json.Unmarshal(b, &cfg) == nil {
			s.cfg = normSelfLogConfig(cfg)
		}
	}
	return s
}

func normSelfLogConfig(cfg selfLogConfig) selfLogConfig {
	if sevNum(cfg.MinLevel) < 0 {
		cfg.MinLevel = "warning"
	}
	return cfg
}

func sevNum(name string) int {
	for i, n := range severityNames {
		if n == name {
			return i
		}
	}
	return -1
}

// logEvent writes an internal event to the process log and, when the
// self-logs source is on and the level is high enough, emits it as an entry.
// sev is a syslog severity (3 err, 4 warning, 5 notice, 6 info).
func logEvent(component string, sev int, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("%s: %s", component, msg)
	if s := selfLog; s != nil {
		s.Emit(component, sev, format, msg, time.Now())
	}
}

// Emit queues an event without ever blocking. The same component and message
// format are let through once per interval; the next one says how many were
// left out in between.
func (s *SelfLog) Emit(component string, sev int, format, msg string, now time.Time) {
	sev = min(max(sev, 0), 7)
	key := component + "\x00" + format
	s.mu.Lock()
	if !s.cfg.Enabled || sev > sevNum(s.cfg.MinLevel) {
		s.mu.Unlock()
		return
	}
	if t, ok := s.last[key]; ok && now.Sub(t) < s.interval {
		s.suppressed[key]++
		s.throttled++
		s.mu.Unlock()
		return
	}
	s.last[key] = now
	if n := s.suppressed[key]; n > 0 {
		msg += fmt.Sprintf(" (%d similar events not shown)", n)
		delete(s.suppressed, key)
	}
	if len(s.last) > 1000 { // formats are constants: only a bug could get here
		clear(s.last)
	}
	s.mu.Unlock()

	e := &Entry{
		Time:       now,
		Received:   now,
		Host:       "logstream",
		App:        component,
		Facility:   "syslog",
		Severity:   severityNames[sev],
		SevNum:     sev,
		Message:    msg,
		Proto:      "internal",
		SourceType: "logstream",
		// Wait stays false: when the Store queue is full, the event is dropped
		// instead of blocking the component that reports it.
	}
	select {
	case s.queue <- e:
	default:
		s.mu.Lock()
		s.dropped++
		s.mu.Unlock()
	}
}

// Run hands the queued events to the sink until the context ends, then
// delivers what is left (the stop event among them).
func (s *SelfLog) Run(ctx context.Context) {
	for {
		select {
		case e := <-s.queue:
			s.deliver(e)
		case <-ctx.Done():
			for {
				select {
				case e := <-s.queue:
					s.deliver(e)
				default:
					return
				}
			}
		}
	}
}

func (s *SelfLog) deliver(e *Entry) {
	s.sink(e)
	s.mu.Lock()
	s.emitted++
	s.mu.Unlock()
}

// Configure saves and applies the configuration.
func (s *SelfLog) Configure(cfg selfLogConfig) error {
	cfg = normSelfLogConfig(cfg)
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	return writeJSONFile(s.cfgPath, cfg)
}

// Status is the state shown in Settings > Sources.
func (s *SelfLog) Status() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]any{
		"enabled":   s.cfg.Enabled,
		"min_level": s.cfg.MinLevel,
		"emitted":   s.emitted,
		"dropped":   s.dropped,
		"throttled": s.throttled,
	}
}
