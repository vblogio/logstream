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
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
)

// hostLogsConfig is saved in /data/hostlogs.json and edited in Settings > Sources.
type hostLogsConfig struct {
	Enabled bool `json:"enabled"`
}

// hostPos is where reading resumes in a journal file (by file ID) or a text
// file (by name, with its inode to detect rotations). Off -1: archived file
// read to the end.
type hostPos struct {
	Off int64  `json:"off"`
	Ino uint64 `json:"ino,omitempty"`
}

// HostLogs collects the system logs of the machine hosting the stack: the
// systemd journal when there is one, else the text files of /var/log. The
// host directories are mounted read-only under root (/host by default).
type HostLogs struct {
	root      string
	sink      func(*Entry)
	backfill  time.Duration
	cfgPath   string
	statePath string
	wake      chan struct{}

	// Owned by the Run goroutine.
	pos     map[string]hostPos
	dirty   bool
	started bool // a first scan was done since enabling: new files are read from their start

	mu         sync.Mutex
	cfg        hostLogsConfig
	reset      bool
	mode       string // "journal", "files" or "" (nothing found)
	files      int
	read       uint64
	compressed uint64
	errCode    string
	errDetail  string
}

func NewHostLogs(root, dataDir string, backfill time.Duration, sink func(*Entry)) *HostLogs {
	h := &HostLogs{
		root:      root,
		sink:      sink,
		backfill:  backfill,
		cfgPath:   filepath.Join(dataDir, "hostlogs.json"),
		statePath: filepath.Join(dataDir, "hostlogs-state.json"),
		wake:      make(chan struct{}, 1),
		pos:       map[string]hostPos{},
	}
	if b, err := os.ReadFile(h.cfgPath); err == nil {
		_ = json.Unmarshal(b, &h.cfg)
	}
	if b, err := os.ReadFile(h.statePath); err == nil {
		_ = json.Unmarshal(b, &h.pos)
		h.started = len(h.pos) > 0
	}
	return h
}

// Run polls the host logs every second while the source is enabled.
func (h *HostLogs) Run(ctx context.Context) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	n := 0
	for {
		select {
		case <-ctx.Done():
			h.saveState()
			return
		case <-tick.C:
		case <-h.wake:
		}
		h.mu.Lock()
		enabled, reset := h.cfg.Enabled, h.reset
		h.reset = false
		h.mu.Unlock()
		if reset {
			// Disabled then enabled again: start over from HOST_LOGS_BACKFILL ago
			// rather than reading everything written in between.
			h.pos, h.started, h.dirty = map[string]hostPos{}, false, true
		}
		if enabled {
			h.scan(time.Now())
		}
		if n++; n%5 == 0 || reset {
			h.saveState()
		}
	}
}

func (h *HostLogs) saveState() {
	if !h.dirty {
		return
	}
	h.dirty = false
	if err := writeJSONFile(h.statePath, h.pos); err != nil {
		log.Printf("host logs: saving positions: %v", err)
	}
}

func (h *HostLogs) setStatus(mode string, files int, code, detail string) {
	h.mu.Lock()
	h.mode, h.files, h.errCode, h.errDetail = mode, files, code, detail
	h.mu.Unlock()
}

// scan reads what was added since the previous poll.
func (h *HostLogs) scan(now time.Time) {
	notBefore := time.Time{}
	if !h.started {
		notBefore = now.Add(-h.backfill)
	}
	defer func() { h.started = true }()

	var journals []string
	for _, dir := range []string{"var/log/journal", "run/log/journal"} {
		base := filepath.Join(h.root, dir)
		for _, pat := range []string{"*.journal", "*/*.journal"} {
			m, _ := filepath.Glob(filepath.Join(base, pat))
			journals = append(journals, m...)
		}
	}
	if len(journals) > 0 {
		h.scanJournals(journals, notBefore)
		return
	}
	h.scanFiles(notBefore.IsZero())
}

func (h *HostLogs) scanJournals(paths []string, notBefore time.Time) {
	seen := map[string]bool{}
	var firstErr error
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			firstErr = keepFirst(firstErr, err)
			continue
		}
		hdr, err := readJournalHeader(f)
		f.Close()
		if err != nil {
			continue // file being created, or not a journal
		}
		key := "j:" + hdr.fileID
		seen[key] = true
		pos, known := h.pos[key]
		if known && pos.Off < 0 {
			continue
		}
		from, nb := pos.Off, time.Time{}
		if !known {
			if hdr.archived && !notBefore.IsZero() && time.UnixMicro(int64(hdr.tailRealtimeUS)).Before(notBefore) {
				h.pos[key], h.dirty = hostPos{Off: -1}, true // archived before the backfill window
				continue
			}
			nb = notBefore
		}
		next, hdr, err := readJournal(p, from, nb, h.emitJournal)
		if err != nil {
			firstErr = keepFirst(firstErr, err)
			continue
		}
		if hdr.archived {
			next = -1
		}
		if !known || next != pos.Off {
			h.pos[key], h.dirty = hostPos{Off: next}, true
		}
	}
	h.prune("j:", seen)
	code, detail := "", ""
	if firstErr != nil && len(seen) == 0 {
		code, detail = errCode(firstErr), firstErr.Error()
	}
	h.setStatus("journal", len(seen), code, detail)
}

func keepFirst(first, err error) error {
	if first != nil {
		return first
	}
	return err
}

func errCode(err error) string {
	if errors.Is(err, fs.ErrPermission) {
		return "permission"
	}
	return "read"
}

func (h *HostLogs) prune(prefix string, seen map[string]bool) {
	for k := range h.pos {
		if strings.HasPrefix(k, prefix) && !seen[k] {
			delete(h.pos, k)
			h.dirty = true
		}
	}
}

// hostLogFile reports the classic text logs of /var/log (rotated and
// compressed copies are left out).
func hostLogFile(name string) bool {
	return name == "syslog" || name == "messages" || strings.HasSuffix(name, ".log")
}

const maxHostRead = 4 << 20 // per file and per poll

// scanFiles follows the text files of /var/log, for hosts without journald.
// Files present at the first scan are read from their end (only new lines).
func (h *HostLogs) scanFiles(fromStart bool) {
	dir := filepath.Join(h.root, "var/log")
	ents, err := os.ReadDir(dir)
	if err != nil {
		code := errCode(err)
		if errors.Is(err, fs.ErrNotExist) {
			code = "not_mounted"
		}
		h.setStatus("", 0, code, err.Error())
		return
	}
	seen := map[string]bool{}
	var firstErr error
	for _, de := range ents {
		if !de.Type().IsRegular() || !hostLogFile(de.Name()) {
			continue
		}
		name := de.Name()
		key := "f:" + name
		fi, err := de.Info()
		if err != nil {
			continue
		}
		var ino uint64
		if st, ok := fi.Sys().(*syscall.Stat_t); ok {
			ino = uint64(st.Ino)
		}
		pos, known := h.pos[key]
		switch {
		case !known && !fromStart:
			pos = hostPos{Off: fi.Size(), Ino: ino}
		case !known || pos.Ino != ino || fi.Size() < pos.Off:
			pos = hostPos{Off: 0, Ino: ino} // new, rotated or truncated file
		}
		if fi.Size() > pos.Off {
			off, err := h.readFile(filepath.Join(dir, name), name, pos.Off)
			if err != nil {
				firstErr = keepFirst(firstErr, err)
				continue // not readable: not counted as followed
			}
			pos.Off = off
		}
		seen[key] = true
		if old, ok := h.pos[key]; !ok || old != pos {
			h.pos[key], h.dirty = pos, true
		}
	}
	h.prune("f:", seen)
	code, detail := "", ""
	if firstErr != nil && len(seen) == 0 {
		code, detail = errCode(firstErr), firstErr.Error()
	}
	mode := "files"
	if len(seen) == 0 && code == "" {
		mode, code = "", "empty"
	}
	h.setStatus(mode, len(seen), code, detail)
}

// readFile sends the complete lines written after off and returns the new offset.
func (h *HostLogs) readFile(path, name string, off int64) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return off, err
	}
	defer f.Close()
	buf, err := io.ReadAll(io.NewSectionReader(f, off, maxHostRead))
	if err != nil {
		return off, err
	}
	end := bytes.LastIndexByte(buf, '\n')
	if end < 0 {
		if len(buf) < maxHostRead {
			return off, nil // partial line: wait for its end
		}
		end = len(buf) - 1 // line longer than the read window: cut it
	}
	now := time.Now()
	fac := fileFacility(name)
	for _, line := range bytes.Split(buf[:end+1], []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		e := ParseSyslog(line, "", "file", now)
		if e.Host == "" {
			e.Host = "localhost"
		}
		if n, ok := detectLevel(e.Message); ok {
			e.SevNum, e.Severity = n, severityNames[n]
		} else {
			e.SevNum, e.Severity = 6, "info"
		}
		if fac >= 0 {
			e.Facility = facilityNames[fac]
		}
		e.SourceType = "host"
		e.Extra = map[string]string{"log_file": "/var/log/" + name}
		h.sink(e)
		h.count(1, 0)
	}
	return off + int64(end) + 1, nil
}

// fileFacility guesses the facility from the usual Debian/RHEL file names.
func fileFacility(name string) int {
	switch strings.TrimSuffix(name, ".log") {
	case "kern":
		return 0
	case "mail", "maillog":
		return 2
	case "daemon":
		return 3
	case "auth", "secure":
		return 4
	case "cron":
		return 9
	}
	return -1
}

func (h *HostLogs) count(read, compressed uint64) {
	h.mu.Lock()
	h.read += read
	h.compressed += compressed
	h.mu.Unlock()
}

// emitJournal turns a journal entry into an Entry.
func (h *HostLogs) emitJournal(je *journalEntry) {
	f := je.Fields
	msg := f["MESSAGE"]
	if msg == "" && je.Compressed > 0 {
		msg = "(compressed journal entry: read it with journalctl)"
	}
	h.count(1, uint64(je.Compressed))
	if strings.TrimSpace(msg) == "" {
		return
	}
	if !utf8.ValidString(msg) {
		msg = strings.ToValidUTF8(msg, "�")
	}
	sev := 6
	if n, err := strconv.Atoi(f["PRIORITY"]); err == nil && n >= 0 && n <= 7 {
		sev = n
	}
	fac := 1 // user
	switch {
	case f["_TRANSPORT"] == "kernel":
		fac = 0
	case f["_SYSTEMD_UNIT"] != "":
		fac = 3 // daemon
	}
	if n, err := strconv.Atoi(f["SYSLOG_FACILITY"]); err == nil && n >= 0 && n < len(facilityNames) {
		fac = n
	}
	app := firstNonEmpty(f["SYSLOG_IDENTIFIER"], f["_COMM"], strings.TrimSuffix(f["_SYSTEMD_UNIT"], ".service"))
	host := firstNonEmpty(f["_HOSTNAME"], "localhost")
	h.sink(&Entry{
		Time:       je.Realtime,
		Received:   time.Now(),
		Host:       host,
		App:        app,
		ProcID:     firstNonEmpty(f["SYSLOG_PID"], f["_PID"]),
		Facility:   facilityNames[fac],
		Severity:   severityNames[sev],
		SevNum:     sev,
		Message:    strings.TrimRight(msg, "\n"),
		Proto:      "journal",
		SourceType: "host",
		Extra:      map[string]string{"unit": f["_SYSTEMD_UNIT"]},
	})
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// Configure saves and applies the configuration.
func (h *HostLogs) Configure(cfg hostLogsConfig) error {
	h.mu.Lock()
	if h.cfg.Enabled && !cfg.Enabled {
		h.reset = true
		h.mode, h.files, h.errCode, h.errDetail = "", 0, "", ""
	}
	h.cfg = cfg
	h.mu.Unlock()
	select {
	case h.wake <- struct{}{}:
	default:
	}
	return writeJSONFile(h.cfgPath, cfg)
}

// Status is the state shown in Settings > Sources.
func (h *HostLogs) Status() map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	dirs := []string{}
	for _, d := range []string{"var/log/journal", "run/log/journal", "var/log"} {
		if _, err := os.Stat(filepath.Join(h.root, d)); err == nil {
			dirs = append(dirs, "/"+d)
		}
	}
	sort.Strings(dirs)
	return map[string]any{
		"enabled":    h.cfg.Enabled,
		"mode":       h.mode,
		"files":      h.files,
		"read":       h.read,
		"compressed": h.compressed,
		"code":       h.errCode,
		"error":      h.errDetail,
		"mounted":    dirs,
	}
}
