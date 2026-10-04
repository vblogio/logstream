package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
)

// dmesgConfig is saved in /data/dmesg.json and edited in Settings > Sources.
type dmesgConfig struct {
	Enabled bool `json:"enabled"`
}

// dmesgPos is where reading resumes: the last record sent, valid for one boot
// only (the kernel numbers its records from 0 at each boot).
type dmesgPos struct {
	BootID string `json:"boot_id"`
	Seq    uint64 `json:"seq"`
}

// Dmesg collects the kernel ring buffer of the machine hosting the stack by
// reading /dev/kmsg, the device dmesg itself reads.
type Dmesg struct {
	device     string // /dev/kmsg
	bootIDPath string // identifies the current boot
	uptimePath string // gives the boot time, to date the records
	sink       func(*Entry)
	backfill   time.Duration
	cfgPath    string
	statePath  string
	retry      time.Duration // delay before opening the device again after a failure
	wake       chan struct{}

	mu        sync.Mutex
	cfg       dmesgConfig
	reset     bool
	pos       dmesgPos
	dirty     bool
	open      bool
	read      uint64
	lost      uint64
	errCode   string
	errDetail string
}

func NewDmesg(device, dataDir string, backfill time.Duration, sink func(*Entry)) *Dmesg {
	d := &Dmesg{
		device:     device,
		bootIDPath: "/proc/sys/kernel/random/boot_id",
		uptimePath: "/proc/uptime",
		sink:       sink,
		backfill:   backfill,
		cfgPath:    filepath.Join(dataDir, "dmesg.json"),
		statePath:  filepath.Join(dataDir, "dmesg-state.json"),
		retry:      10 * time.Second,
		wake:       make(chan struct{}, 1),
	}
	if b, err := os.ReadFile(d.cfgPath); err == nil {
		_ = json.Unmarshal(b, &d.cfg)
	}
	if b, err := os.ReadFile(d.statePath); err == nil {
		_ = json.Unmarshal(b, &d.pos)
	}
	return d
}

// kmsgSession is one opening of the device, read by its own goroutine.
type kmsgSession struct {
	f       *os.File
	done    chan struct{}
	err     error
	stopped bool // closed on purpose: err is not a failure
}

func (s *kmsgSession) stop() {
	s.stopped = true
	s.f.Close() // ends a pending read (/dev/kmsg goes through the poller)
	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
	}
}

// Run keeps the device open while the source is enabled.
func (d *Dmesg) Run(ctx context.Context) {
	save := time.NewTicker(5 * time.Second)
	defer save.Stop()
	var cur *kmsgSession
	var retryAt time.Time
	for {
		d.mu.Lock()
		enabled, reset := d.cfg.Enabled, d.reset
		d.reset = false
		d.mu.Unlock()
		if reset {
			// Disabled then enabled again: start over from DMESG_BACKFILL ago
			// rather than reading everything logged in between.
			if cur != nil {
				cur.stop()
				cur = nil
			}
			d.mu.Lock()
			d.pos, d.dirty = dmesgPos{}, true
			d.mu.Unlock()
			d.saveState()
			retryAt = time.Time{}
		}
		if !enabled && cur != nil {
			cur.stop()
			cur = nil
			d.mu.Lock()
			d.open = false
			d.mu.Unlock()
		}
		if enabled && cur == nil && !time.Now().Before(retryAt) {
			var err error
			if cur, err = d.start(); err != nil {
				d.fail(err)
				retryAt = time.Now().Add(d.retry)
			}
		}
		var done chan struct{}
		if cur != nil {
			done = cur.done
		}
		wait := d.retry
		if cur == nil && enabled {
			wait = max(time.Until(retryAt), 10*time.Millisecond)
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			if cur != nil {
				cur.stop()
			}
			d.saveState()
			return
		case <-d.wake:
		case <-save.C:
			d.saveState()
		case <-timer.C:
		case <-done:
			if !cur.stopped {
				d.fail(cur.err)
			}
			cur = nil
			retryAt = time.Now().Add(d.retry)
		}
		timer.Stop()
	}
}

func (d *Dmesg) saveState() {
	d.mu.Lock()
	if !d.dirty {
		d.mu.Unlock()
		return
	}
	d.dirty = false
	pos := d.pos
	d.mu.Unlock()
	if err := writeJSONFile(d.statePath, pos); err != nil {
		log.Printf("dmesg: saving position: %v", err)
	}
}

// fail records why the device is not being read (nil error: end of a
// regular file, only seen in tests).
func (d *Dmesg) fail(err error) {
	code, detail := "", ""
	if err != nil && !errors.Is(err, io.EOF) {
		detail = err.Error()
		switch {
		case errors.Is(err, fs.ErrNotExist):
			code = "not_found"
		case errors.Is(err, fs.ErrPermission):
			code = "permission"
		default:
			code = "read"
		}
	}
	d.mu.Lock()
	changed := code != d.errCode
	d.open, d.errCode, d.errDetail = false, code, detail
	d.mu.Unlock()
	if changed && code != "" {
		log.Printf("dmesg: %v", err)
	}
}

// start opens the device and starts reading it in the background.
func (d *Dmesg) start() (*kmsgSession, error) {
	f, err := os.Open(d.device)
	if err != nil {
		return nil, err
	}
	bootID := readTrim(d.bootIDPath)
	boot := d.bootTime(time.Now())
	d.mu.Lock()
	pos := d.pos
	d.open, d.errCode, d.errDetail = true, "", ""
	d.mu.Unlock()

	// Same boot as the saved position: skip what was already sent. Otherwise
	// read from DMESG_BACKFILL ago (0: only new records).
	resume := pos.BootID != "" && pos.BootID == bootID
	var notBefore time.Time
	if !resume {
		if d.backfill <= 0 {
			if _, err := f.Seek(0, io.SeekEnd); err != nil {
				f.Close()
				return nil, err
			}
		} else if !boot.IsZero() {
			notBefore = time.Now().Add(-d.backfill)
		}
		d.mu.Lock()
		d.pos, d.dirty = dmesgPos{BootID: bootID}, true
		d.mu.Unlock()
	}
	s := &kmsgSession{f: f, done: make(chan struct{})}
	go func() {
		defer close(s.done)
		s.err = d.consume(f, bootID, boot, resume, pos.Seq, notBefore)
	}()
	return s, nil
}

func readTrim(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// bootTime is now minus the uptime; zero when unknown (records are then dated
// when they are read).
func (d *Dmesg) bootTime(now time.Time) time.Time {
	f := strings.Fields(readTrim(d.uptimePath))
	if len(f) == 0 {
		return time.Time{}
	}
	up, err := strconv.ParseFloat(f[0], 64)
	if err != nil || up <= 0 {
		return time.Time{}
	}
	return now.Add(-time.Duration(up * float64(time.Second)))
}

// consume reads records until an error. Each read of /dev/kmsg returns one
// record; several records in one read (a regular file, in tests) are split.
func (d *Dmesg) consume(r io.Reader, bootID string, boot time.Time, resume bool, after uint64, notBefore time.Time) error {
	buf := make([]byte, 64<<10) // a record is at most 8 KiB
	last, haveLast := after, resume
	for {
		n, err := r.Read(buf)
		if n > 0 {
			for _, raw := range splitKmsg(buf[:n]) {
				rec, ok := parseKmsg(raw)
				if !ok {
					continue
				}
				if haveLast && rec.Seq <= last {
					continue // already sent before a restart
				}
				var lost uint64
				if haveLast && rec.Seq > last+1 {
					lost = rec.Seq - last - 1 // overwritten in the ring buffer before being read
				}
				last, haveLast = rec.Seq, true
				now := time.Now()
				t := now
				if !boot.IsZero() {
					t = boot.Add(time.Duration(rec.Usec) * time.Microsecond)
				}
				if !notBefore.IsZero() && t.Before(notBefore) {
					d.advance(bootID, rec.Seq, 0, 0)
					continue
				}
				d.sink(kmsgEntry(rec, t, now))
				d.advance(bootID, rec.Seq, 1, lost)
			}
		}
		if err != nil {
			if errors.Is(err, os.ErrClosed) {
				return nil
			}
			if errors.Is(err, syscall.EPIPE) {
				continue // records overwritten while reading: the next read goes on
			}
			return err
		}
	}
}

func (d *Dmesg) advance(bootID string, seq, read, lost uint64) {
	d.mu.Lock()
	d.pos, d.dirty = dmesgPos{BootID: bootID, Seq: seq}, true
	d.read += read
	d.lost += lost
	d.mu.Unlock()
}

// kmsgRecord is one record of /dev/kmsg:
//
//	prio,seq,usec,flags[,...];message
//	 KEY=value          (optional dictionary lines)
type kmsgRecord struct {
	Prio int
	Seq  uint64
	Usec uint64
	Msg  string
	Dict map[string]string
}

// splitKmsg cuts a buffer into records: a new record starts after a newline
// that is not followed by a space (dictionary lines start with one).
func splitKmsg(b []byte) [][]byte {
	var out [][]byte
	start := 0
	for i := 0; i < len(b); i++ {
		if b[i] == '\n' && (i+1 == len(b) || b[i+1] != ' ') {
			out = append(out, b[start:i])
			start = i + 1
		}
	}
	if start < len(b) {
		out = append(out, b[start:])
	}
	return out
}

func parseKmsg(raw []byte) (kmsgRecord, bool) {
	var rec kmsgRecord
	lines := bytes.Split(raw, []byte{'\n'})
	head, msg, ok := bytes.Cut(lines[0], []byte{';'})
	if !ok {
		return rec, false
	}
	f := strings.Split(string(head), ",")
	if len(f) < 3 {
		return rec, false
	}
	prio, err1 := strconv.Atoi(f[0])
	seq, err2 := strconv.ParseUint(f[1], 10, 64)
	usec, err3 := strconv.ParseUint(f[2], 10, 64)
	if err1 != nil || err2 != nil || err3 != nil || prio < 0 {
		return rec, false
	}
	rec.Prio, rec.Seq, rec.Usec = prio, seq, usec
	rec.Msg = unescapeKmsg(string(msg))
	for _, l := range lines[1:] {
		if k, v, ok := strings.Cut(strings.TrimPrefix(string(l), " "), "="); ok && k != "" {
			if rec.Dict == nil {
				rec.Dict = map[string]string{}
			}
			rec.Dict[k] = unescapeKmsg(v)
		}
	}
	return rec, true
}

// unescapeKmsg decodes the \xNN escapes the kernel uses for non-printable bytes.
func unescapeKmsg(s string) string {
	if !strings.Contains(s, `\x`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) && s[i+1] == 'x' {
			if v, err := strconv.ParseUint(s[i+2:i+4], 16, 8); err == nil {
				b.WriteByte(byte(v))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// kmsgEntry turns a record into an Entry. Kernel records have the kern
// facility; programs writing to /dev/kmsg (systemd at boot…) use another
// one and prefix their name: "systemd[1]: …".
func kmsgEntry(rec kmsgRecord, t, now time.Time) *Entry {
	sev := rec.Prio & 7
	fac := rec.Prio >> 3
	if fac >= len(facilityNames) {
		fac = 1 // user
	}
	msg := strings.TrimRight(rec.Msg, "\n")
	if !utf8.ValidString(msg) {
		msg = strings.ToValidUTF8(msg, "�")
	}
	app, pid := "kernel", ""
	if fac != 0 {
		if a, p, rest, ok := splitTag(msg); ok {
			app, pid, msg = a, p, rest
		}
	}
	e := &Entry{
		Time:       t,
		Received:   now,
		Host:       "localhost",
		App:        app,
		ProcID:     pid,
		Facility:   facilityNames[fac],
		Severity:   severityNames[sev],
		SevNum:     sev,
		Message:    msg,
		Proto:      "kmsg",
		SourceType: "kernel",
		Wait:       true,
	}
	if s, dev := rec.Dict["SUBSYSTEM"], rec.Dict["DEVICE"]; s != "" || dev != "" {
		e.Extra = map[string]string{}
		if s != "" {
			e.Extra["subsystem"] = s
		}
		if dev != "" {
			e.Extra["device"] = dev
		}
	}
	return e
}

// splitTag cuts "name[pid]: message" or "name: message".
func splitTag(msg string) (app, pid, rest string, ok bool) {
	tag, rest, found := strings.Cut(msg, ": ")
	if !found || tag == "" || len(tag) > 48 || strings.ContainsAny(tag, " \t") {
		return "", "", "", false
	}
	if i := strings.IndexByte(tag, '['); i > 0 && strings.HasSuffix(tag, "]") {
		return tag[:i], tag[i+1 : len(tag)-1], rest, true
	}
	return tag, "", rest, true
}

// Configure saves and applies the configuration.
func (d *Dmesg) Configure(cfg dmesgConfig) error {
	d.mu.Lock()
	if d.cfg.Enabled && !cfg.Enabled {
		d.reset = true
		d.errCode, d.errDetail = "", ""
	}
	d.cfg = cfg
	d.mu.Unlock()
	select {
	case d.wake <- struct{}{}:
	default:
	}
	return writeJSONFile(d.cfgPath, cfg)
}

// Status is the state shown in Settings > Sources.
func (d *Dmesg) Status() map[string]any {
	d.mu.Lock()
	defer d.mu.Unlock()
	return map[string]any{
		"enabled": d.cfg.Enabled,
		"open":    d.open && d.cfg.Enabled,
		"device":  d.device,
		"read":    d.read,
		"lost":    d.lost,
		"code":    d.errCode,
		"error":   d.errDetail,
	}
}
