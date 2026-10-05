package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// fileSourceDef is one log file declared in Settings > Sources.
type fileSourceDef struct {
	ID      string `json:"id"`
	Name    string `json:"name"`            // app of the entries
	Path    string `json:"path"`            // the file, as mounted in the container
	Format  string `json:"format"`          // auto (detected), or text, json, logfmt, regex, clf, traefik
	Regex   string `json:"regex,omitempty"` // format regex: named groups time, level, msg, host, app…
	Host    string `json:"host,omitempty"`  // host of the entries (empty: the host name of the machine)
	Enabled bool   `json:"enabled"`
}

// filesConfig is saved in /data/files.json.
type filesConfig struct {
	Sources []fileSourceDef `json:"sources"`
}

// filePos is where reading resumes in a file: its inode and an offset before
// which every entry was sent. Until: end of the history found when the
// source was turned on; entries before it are kept only when dated within
// FILES_BACKFILL.
type filePos struct {
	Ino   uint64 `json:"ino"`
	Off   int64  `json:"off"`
	Until int64  `json:"until,omitempty"`
}

type filesState struct {
	Pos     map[string]filePos   `json:"pos"`     // by source id
	Started map[string]time.Time `json:"started"` // when each source was turned on
}

type fileSourceStatus struct {
	Open     bool
	Read     uint64
	Detected string // format found when the source is in auto
	Behind   int64  // bytes left to read (history being read)
	Code     string
	Error    string
}

// fileSource is a definition with its parser (nil in auto: detected from
// the file).
type fileSource struct {
	fileSourceDef
	parser *fileParser
}

// pendingEntry is an entry waiting for its continuation lines.
type pendingEntry struct {
	rec     *fileRec
	off     int64     // offset of its first line
	seen    time.Time // when its last line was read
	history bool      // read from the history found when the source was turned on
	lines   int
}

// fileTail is the reading state of one file, owned by the Run goroutine.
type fileTail struct {
	readOff int64 // where the next read starts (after the lines of the pending entry)
	known   bool  // readOff is set
	pending *pendingEntry
	auto    *fileParser // format detected from the file, in auto
	behind  int64       // bytes left to read after the last poll
}

// Files follows log files of other programs (Traefik, nginx, applications…)
// mounted in the container, one entry per line in the format detected or
// chosen for each file. Source type "file".
type Files struct {
	sink      func(*Entry)
	hostName  string
	backfill  time.Duration
	cfgPath   string
	statePath string
	wake      chan struct{}

	// Owned by the Run goroutine.
	st    filesState
	tails map[string]*fileTail
	dirty bool

	mu       sync.Mutex
	cfg      []*fileSource
	resets   map[string]bool // sources to read again from FILES_BACKFILL ago
	redetect map[string]bool // sources whose format changed
	status   map[string]*fileSourceStatus
}

func NewFiles(hostRoot, dataDir string, backfill time.Duration, sink func(*Entry)) *Files {
	f := &Files{
		sink:      sink,
		hostName:  readTrim(filepath.Join(hostRoot, "etc/hostname")),
		backfill:  backfill,
		cfgPath:   filepath.Join(dataDir, "files.json"),
		statePath: filepath.Join(dataDir, "files-state.json"),
		wake:      make(chan struct{}, 1),
		tails:     map[string]*fileTail{},
		resets:    map[string]bool{},
		redetect:  map[string]bool{},
		status:    map[string]*fileSourceStatus{},
	}
	if f.hostName == "" {
		f.hostName = "localhost"
	}
	var cfg filesConfig
	if b, err := os.ReadFile(f.cfgPath); err == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	for _, d := range cfg.Sources {
		src, err := compileFileSource(d)
		if err != nil {
			logEvent("files", 4, "file %q ignored: %v", d.Name, err)
			continue
		}
		f.cfg = append(f.cfg, src)
	}
	if b, err := os.ReadFile(f.statePath); err == nil {
		_ = json.Unmarshal(b, &f.st)
	}
	if f.st.Pos == nil {
		f.st.Pos = map[string]filePos{}
	}
	if f.st.Started == nil {
		f.st.Started = map[string]time.Time{}
	}
	return f
}

const maxFileSources = 50

var fileIDRe = regexp.MustCompile(`^[0-9a-f]{1,32}$`)

// checkFilePath: one file, absolute, without wildcards.
func checkFilePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	switch {
	case !filepath.IsAbs(p):
		return "", errors.New("the path must be absolute (as mounted in the container)")
	case strings.ContainsAny(p, "*?["):
		return "", errors.New("give the path of one file (no wildcards)")
	}
	return filepath.Clean(p), nil
}

// compileFileSource checks a definition and builds its parser.
func compileFileSource(d fileSourceDef) (*fileSource, error) {
	d.Name = strings.TrimSpace(d.Name)
	d.Host = strings.TrimSpace(d.Host)
	if d.Format == "" {
		d.Format = fmtAuto
	}
	if d.Format != fmtRegex {
		d.Regex = ""
	}
	switch {
	case !fileIDRe.MatchString(d.ID):
		return nil, errors.New("invalid id")
	case d.Name == "" || len(d.Name) > 64:
		return nil, errors.New("the name is required (64 characters at most)")
	case len(d.Host) > 253:
		return nil, fmt.Errorf("%s: host name too long", d.Name)
	}
	p, err := checkFilePath(d.Path)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", d.Name, err)
	}
	d.Path = p
	src := &fileSource{fileSourceDef: d}
	if d.Format != fmtAuto {
		if src.parser, err = newFileParser(d.Format, d.Regex); err != nil {
			return nil, fmt.Errorf("%s: %v", d.Name, err)
		}
	}
	return src, nil
}

func newFileID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Configure checks, saves and applies the list of files. Files turned off,
// removed or given another path are read again from FILES_BACKFILL ago the
// next time they are on.
func (f *Files) Configure(cfg filesConfig) error {
	if len(cfg.Sources) > maxFileSources {
		return fmt.Errorf("%d files at most", maxFileSources)
	}
	var srcs []*fileSource
	ids := map[string]bool{}
	for _, d := range cfg.Sources {
		if d.ID == "" {
			d.ID = newFileID()
		}
		if ids[d.ID] {
			return fmt.Errorf("duplicate id %s", d.ID)
		}
		ids[d.ID] = true
		src, err := compileFileSource(d)
		if err != nil {
			return err
		}
		srcs = append(srcs, src)
	}
	saved := filesConfig{Sources: []fileSourceDef{}}
	for _, s := range srcs {
		saved.Sources = append(saved.Sources, s.fileSourceDef)
	}
	if err := writeJSONFile(f.cfgPath, saved); err != nil {
		return err
	}
	f.mu.Lock()
	old := map[string]*fileSource{}
	for _, s := range f.cfg {
		old[s.ID] = s
	}
	for _, s := range srcs {
		o := old[s.ID]
		if o != nil && ((o.Enabled && !s.Enabled) || o.Path != s.Path) {
			f.resets[s.ID] = true
			delete(f.status, s.ID)
		}
		if o != nil && (o.Format != s.Format || o.Regex != s.Regex) {
			f.redetect[s.ID] = true
		}
		delete(old, s.ID)
	}
	for id := range old { // removed
		f.resets[id] = true
		delete(f.status, id)
	}
	f.cfg = srcs
	f.mu.Unlock()
	select {
	case f.wake <- struct{}{}:
	default:
	}
	return nil
}

// Status is the state shown in Settings > Sources.
func (f *Files) Status() map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	list := make([]map[string]any, 0, len(f.cfg))
	for _, s := range f.cfg {
		m := map[string]any{
			"id": s.ID, "name": s.Name, "path": s.Path, "format": s.Format, "regex": s.Regex,
			"host": s.Host, "enabled": s.Enabled, "open": false, "read": 0, "detected": "", "behind": 0, "code": "", "error": "",
		}
		if st := f.status[s.ID]; st != nil && s.Enabled {
			m["open"], m["read"], m["detected"], m["code"], m["error"] = st.Open, st.Read, st.Detected, st.Code, st.Error
			m["behind"] = st.Behind
		}
		list = append(list, m)
	}
	return map[string]any{"sources": list, "defaultHost": f.hostName}
}

// Run polls the files every second.
func (f *Files) Run(ctx context.Context) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	n := 0
	for {
		select {
		case <-ctx.Done():
			now := time.Now()
			for id := range f.tails {
				f.flushPending(id, now)
			}
			f.checkpointAll()
			f.saveState()
			return
		case <-tick.C:
		case <-f.wake:
		}
		f.poll(time.Now())
		if n++; n%5 == 0 {
			f.saveState()
		}
	}
}

func (f *Files) saveState() {
	if !f.dirty {
		return
	}
	f.dirty = false
	if err := writeJSONFile(f.statePath, f.st); err != nil {
		logEvent("files", 4, "saving positions: %v", err)
	}
}

// poll reads what was added to every file that is on.
func (f *Files) poll(now time.Time) {
	f.mu.Lock()
	srcs := f.cfg
	resets, redetect := f.resets, f.redetect
	f.resets, f.redetect = map[string]bool{}, map[string]bool{}
	f.mu.Unlock()
	for id := range resets {
		delete(f.st.Pos, id)
		delete(f.st.Started, id)
		delete(f.tails, id)
		f.dirty = true
	}
	for id := range redetect {
		if tl := f.tails[id]; tl != nil {
			tl.auto = nil
		}
	}
	active := map[string]bool{}
	for _, s := range srcs {
		if !s.Enabled {
			continue
		}
		active[s.ID] = true
		f.setStatus(s.ID, f.pollFile(s, now))
	}
	// Files turned off: their position is kept until the next poll forgets it.
	for id := range f.tails {
		if !active[id] {
			delete(f.tails, id)
		}
	}
	f.checkpointAll()
}

const (
	maxFileRead       = 4 << 20         // per read
	maxFileReads      = 16              // reads per file and per poll (64 MB)
	fileContinueDelay = 2 * time.Second // an entry waits this long for continuation lines
)

func (f *Files) setStatus(id string, err error) {
	code, detail := "", ""
	if err != nil {
		code, detail = errCode(err), err.Error()
		if errors.Is(err, fs.ErrNotExist) {
			code = "not_found"
		}
	}
	detected := ""
	if tl := f.tails[id]; tl != nil && tl.auto != nil {
		detected = tl.auto.format
	}
	f.mu.Lock()
	st := f.status[id]
	if st == nil {
		st = &fileSourceStatus{}
		f.status[id] = st
	}
	changed := code != st.Code
	st.Open, st.Detected, st.Code, st.Error = err == nil, detected, code, detail
	st.Behind = 0
	if tl := f.tails[id]; tl != nil && err == nil {
		st.Behind = tl.behind
	}
	name := id
	for _, s := range f.cfg {
		if s.ID == id {
			name = s.Name
		}
	}
	f.mu.Unlock()
	if changed && code != "" && code != "not_found" {
		logEvent("files", 4, "cannot read the file %s: %v", name, err)
	}
}

func (f *Files) count(id string) {
	f.mu.Lock()
	st := f.status[id]
	if st == nil { // first poll of the file: its status comes after
		st = &fileSourceStatus{}
		f.status[id] = st
	}
	st.Read++
	f.mu.Unlock()
}

// parserOf is the parser of a file: the chosen format, or the one detected
// from its last lines (nil while the file has none).
func (f *Files) parserOf(s *fileSource, tl *fileTail, path string) *fileParser {
	if s.parser != nil {
		return s.parser
	}
	if tl.auto == nil {
		lines, err := sampleLines(path)
		if err != nil || len(lines) == 0 {
			return nil
		}
		tl.auto, _ = newFileParser(detectFormat(lines), "")
	}
	return tl.auto
}

func (f *Files) pollFile(s *fileSource, now time.Time) error {
	path := s.Path
	first := false
	if _, ok := f.st.Started[s.ID]; !ok {
		f.st.Started[s.ID], f.dirty, first = now, true, true
	}
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	tl := f.tails[s.ID]
	if tl == nil {
		tl = &fileTail{}
		f.tails[s.ID] = tl
	}
	ino := inodeOf(fi)
	pos, known := f.st.Pos[s.ID]
	if !known {
		pos = filePos{Ino: ino}
		if first {
			// Turned on: the last FILES_BACKFILL of what is already there
			// (0: only new lines), found by date rather than read from the
			// start of a file that can hold months of logs.
			pos.Off = fi.Size()
			if p := f.parserOf(s, tl, path); f.backfill > 0 && p != nil {
				pos.Off = backfillStart(path, p, fi.Size(), f.st.Started[s.ID].Add(-f.backfill), now)
				pos.Until = fi.Size()
			}
		}
		f.st.Pos[s.ID], f.dirty = pos, true
	}
	if !tl.known {
		tl.readOff, tl.known = pos.Off, true
	}
	if pos.Ino != ino {
		// Rotated (access.log -> access.log.1): finish the old file, then
		// read the new one from its start.
		if old, err := os.Stat(path + ".1"); err == nil && inodeOf(old) == pos.Ino {
			if p := f.parserOf(s, tl, path+".1"); p != nil {
				for {
					next, more, err := f.readFrom(s, tl, p, path+".1", tl.readOff, pos.Until, now)
					if err != nil || !more {
						break
					}
					tl.readOff = next
				}
			}
		}
		f.flushPending(s.ID, now)
		pos, tl.readOff = filePos{Ino: ino}, 0
		f.st.Pos[s.ID], f.dirty = pos, true
	} else if fi.Size() < tl.readOff {
		f.flushPending(s.ID, now) // truncated (copytruncate)
		pos.Until, tl.readOff = 0, 0
		f.st.Pos[s.ID], f.dirty = pos, true
	}
	if fi.Size() > tl.readOff {
		p := f.parserOf(s, tl, path)
		if p == nil {
			return nil // nothing complete to detect the format from yet
		}
		for i := 0; i < maxFileReads; i++ {
			next, more, err := f.readFrom(s, tl, p, path, tl.readOff, pos.Until, now)
			if err != nil {
				return err
			}
			tl.readOff = next
			if !more {
				break
			}
		}
	}
	tl.behind = max(fi.Size()-tl.readOff, 0)
	if pe := tl.pending; pe != nil && now.Sub(pe.seen) >= fileContinueDelay {
		f.flushPending(s.ID, now)
	}
	return nil
}

// backfillStart finds where the lines dated notBefore or later start, by
// bisection on the dates of the lines (logs are written in order). Undated
// lines are left out of the history anyway: then only new lines are read.
func backfillStart(path string, p *fileParser, size int64, notBefore, now time.Time) int64 {
	fh, err := os.Open(path)
	if err != nil {
		return size
	}
	defer fh.Close()
	buf := make([]byte, 64<<10)
	// lineAt reads the first line starting at or after off (off itself when
	// it is 0): its start and its date.
	lineAt := func(off int64) (int64, time.Time, bool) {
		n, _ := fh.ReadAt(buf, off)
		b := buf[:n]
		start := off
		if off > 0 {
			nl := bytes.IndexByte(b, '\n')
			if nl < 0 {
				return size, time.Time{}, false
			}
			b, start = b[nl+1:], off+int64(nl)+1
		}
		end := bytes.IndexByte(b, '\n')
		if end < 0 {
			return size, time.Time{}, false
		}
		r := p.parse(string(b[:end]), now)
		return start, r.t, r.dated
	}
	if _, t, ok := lineAt(0); !ok || !t.Before(notBefore) {
		if !ok {
			// No date on the first line: try the last ones.
			if _, _, ok := lineAt(max(size-int64(len(buf)), 0)); !ok {
				return size
			}
		} else {
			return 0 // all of it is recent
		}
	}
	lo, hi := int64(0), size // line at lo: before notBefore; at hi: not
	for hi-lo > int64(len(buf)) {
		mid := lo + (hi-lo)/2
		_, t, ok := lineAt(mid)
		if ok && t.Before(notBefore) {
			lo = mid
		} else {
			hi = mid
		}
	}
	start, _, _ := lineAt(lo)
	return min(start, size)
}

// readFrom parses the complete lines written after off and returns the new
// offset; more tells whether the read window was full.
func (f *Files) readFrom(s *fileSource, tl *fileTail, p *fileParser, path string, off, until int64, now time.Time) (int64, bool, error) {
	fh, err := os.Open(path)
	if err != nil {
		return off, false, err
	}
	defer fh.Close()
	buf, err := io.ReadAll(io.NewSectionReader(fh, off, maxFileRead))
	if err != nil {
		return off, false, err
	}
	full := len(buf) == maxFileRead
	end := bytes.LastIndexByte(buf, '\n')
	if end < 0 {
		if !full {
			return off, false, nil // partial line: wait for its end
		}
		end = len(buf) - 1 // line longer than the read window: cut it
	}
	start := off
	for _, b := range bytes.Split(buf[:end], []byte{'\n'}) {
		f.line(s, tl, p, string(b), start, start < until, now)
		start += int64(len(b)) + 1
	}
	return off + int64(end) + 1, full, nil
}

// line handles one line: a continuation of the pending entry, or a new one.
func (f *Files) line(s *fileSource, tl *fileTail, p *fileParser, line string, off int64, history bool, now time.Time) {
	if strings.TrimSpace(line) == "" {
		return
	}
	if pe := tl.pending; p.multiline() && pe != nil && p.continuation(line) {
		if pe.lines < maxContinuation && len(pe.rec.msg) < maxFileMsg {
			pe.rec.msg += "\n" + strings.TrimRight(cleanLine(line), " \t")
			pe.lines++
		}
		pe.seen = now
		return
	}
	f.flushPending(s.ID, now)
	pe := &pendingEntry{rec: p.parse(line, now), off: off, seen: now, history: history}
	if p.multiline() {
		tl.pending = pe
		return
	}
	f.emit(s, pe, now)
}

func (f *Files) flushPending(id string, now time.Time) {
	tl := f.tails[id]
	if tl == nil || tl.pending == nil {
		return
	}
	pe := tl.pending
	tl.pending = nil
	f.mu.Lock()
	var src *fileSource
	for _, s := range f.cfg {
		if s.ID == id {
			src = s
		}
	}
	f.mu.Unlock()
	if src != nil {
		f.emit(src, pe, now)
	}
}

// emit sends an entry; history entries are kept only when dated within
// FILES_BACKFILL of when the file was turned on.
func (f *Files) emit(s *fileSource, pe *pendingEntry, now time.Time) {
	r := pe.rec
	if pe.history {
		notBefore := f.st.Started[s.ID].Add(-f.backfill)
		if !r.dated || r.t.Before(notBefore) {
			return
		}
	}
	if len(r.msg) > maxFileMsg {
		r.msg = strings.ToValidUTF8(r.msg[:maxFileMsg], "") + "…"
	}
	extra := map[string]string{"log_file": s.Path}
	for k, v := range r.fields {
		extra[k] = v
	}
	f.sink(&Entry{
		Time:       r.t,
		Received:   now,
		Host:       firstNonEmpty(r.host, s.Host, f.hostName),
		App:        firstNonEmpty(r.app, s.Name),
		SevNum:     r.sev,
		Severity:   severityNames[r.sev],
		Message:    r.msg,
		Proto:      "file",
		SourceType: "file",
		Extra:      extra,
		Wait:       true, // the file keeps the lines: wait for room in the queue
	})
	f.count(s.ID)
}

// checkpointAll saves, for each file, an offset before which every entry is
// sent: the start of its pending entry, or the read offset.
func (f *Files) checkpointAll() {
	for id, tl := range f.tails {
		pos, ok := f.st.Pos[id]
		if !ok || !tl.known {
			continue
		}
		off := tl.readOff
		if pe := tl.pending; pe != nil && pe.off < off {
			off = pe.off
		}
		if off != pos.Off {
			pos.Off, f.dirty = off, true
			f.st.Pos[id] = pos
		}
	}
}

// DetectFile tells, for the settings, whether a file can be read and its
// format, with a line as example.
func DetectFile(path string) map[string]any {
	res := map[string]any{"format": "", "sample": "", "code": "", "error": ""}
	p, err := checkFilePath(path)
	if err != nil {
		res["code"], res["error"] = "invalid", err.Error()
		return res
	}
	var lines []string
	if fi, err2 := os.Stat(p); err2 != nil {
		err = err2
	} else if !fi.Mode().IsRegular() {
		err = fmt.Errorf("%s is not a regular file", p)
	} else {
		lines, err = sampleLines(p)
	}
	switch {
	case errors.Is(err, fs.ErrNotExist):
		res["code"], res["error"] = "not_found", err.Error()
	case err != nil:
		res["code"], res["error"] = errCode(err), err.Error()
	case len(lines) == 0:
		res["code"] = "empty"
	default:
		res["format"] = detectFormat(lines)
		sample := lines[len(lines)-1]
		if len(sample) > 300 {
			sample = strings.ToValidUTF8(sample[:300], "") + "…"
		}
		res["sample"] = cleanLine(sample)
	}
	return res
}
