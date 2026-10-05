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
	"sort"
	"strings"
	"sync"
	"time"
)

// fileSourceDef is one log file (or glob) declared in Settings > Sources.
type fileSourceDef struct {
	ID      string `json:"id"`
	Name    string `json:"name"`            // app of the entries
	Path    string `json:"path"`            // file, or glob (/logs/traefik/*.log)
	Format  string `json:"format"`          // text, json, logfmt, regex, clf, traefik
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
	Pos     map[string]filePos   `json:"pos"`     // key: source id + "|" + path
	Started map[string]time.Time `json:"started"` // when each source was turned on
}

type fileSourceStatus struct {
	Files int
	Read  uint64
	Code  string
	Error string
}

// fileSource is a definition with its parser.
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

// Files follows log files of other programs (Traefik, nginx, applications…)
// mounted in the container, one entry per line in the format chosen for each
// file. Source type "file".
type Files struct {
	sink      func(*Entry)
	hostName  string
	backfill  time.Duration
	cfgPath   string
	statePath string
	wake      chan struct{}

	// Owned by the Run goroutine.
	st      filesState
	readOff map[string]int64
	pending map[string]*pendingEntry
	dirty   bool

	mu     sync.Mutex
	cfg    []*fileSource
	resets map[string]bool // sources to read again from FILES_BACKFILL ago
	status map[string]*fileSourceStatus
}

func NewFiles(hostRoot, dataDir string, backfill time.Duration, sink func(*Entry)) *Files {
	f := &Files{
		sink:      sink,
		hostName:  readTrim(filepath.Join(hostRoot, "etc/hostname")),
		backfill:  backfill,
		cfgPath:   filepath.Join(dataDir, "files.json"),
		statePath: filepath.Join(dataDir, "files-state.json"),
		wake:      make(chan struct{}, 1),
		readOff:   map[string]int64{},
		pending:   map[string]*pendingEntry{},
		resets:    map[string]bool{},
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

// compileFileSource checks a definition and builds its parser.
func compileFileSource(d fileSourceDef) (*fileSource, error) {
	d.Name = strings.TrimSpace(d.Name)
	d.Path = strings.TrimSpace(d.Path)
	d.Host = strings.TrimSpace(d.Host)
	if d.Format == "" {
		d.Format = fmtText
	}
	if d.Format != fmtRegex {
		d.Regex = ""
	}
	switch {
	case !fileIDRe.MatchString(d.ID):
		return nil, errors.New("invalid id")
	case d.Name == "" || len(d.Name) > 64:
		return nil, errors.New("the name is required (64 characters at most)")
	case !filepath.IsAbs(d.Path):
		return nil, fmt.Errorf("%s: the path must be absolute (as mounted in the container)", d.Name)
	case len(d.Host) > 253:
		return nil, fmt.Errorf("%s: host name too long", d.Name)
	}
	d.Path = filepath.Clean(d.Path)
	if _, err := filepath.Match(d.Path, ""); err != nil {
		return nil, fmt.Errorf("%s: invalid path pattern: %v", d.Name, err)
	}
	p, err := newFileParser(d.Format, d.Regex)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", d.Name, err)
	}
	return &fileSource{fileSourceDef: d, parser: p}, nil
}

func newFileID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Configure checks, saves and applies the list of files. Sources turned off,
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
		if o := old[s.ID]; o != nil && ((o.Enabled && !s.Enabled) || o.Path != s.Path) {
			f.resets[s.ID] = true
			delete(f.status, s.ID)
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
			"host": s.Host, "enabled": s.Enabled, "files": 0, "read": 0, "code": "", "error": "",
		}
		if st := f.status[s.ID]; st != nil && s.Enabled {
			m["files"], m["read"], m["code"], m["error"] = st.Files, st.Read, st.Code, st.Error
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
			for key := range f.pending {
				f.flushPending(key, now)
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

func posKey(id, path string) string { return id + "|" + path }

// forget drops the positions of a source.
func (f *Files) forget(id string) {
	prefix := id + "|"
	for key := range f.st.Pos {
		if strings.HasPrefix(key, prefix) {
			delete(f.st.Pos, key)
			delete(f.readOff, key)
			delete(f.pending, key)
		}
	}
	delete(f.st.Started, id)
	f.dirty = true
}

// poll reads what was added to the files of every source that is on.
func (f *Files) poll(now time.Time) {
	f.mu.Lock()
	srcs := f.cfg
	resets := f.resets
	f.resets = map[string]bool{}
	f.mu.Unlock()
	for id := range resets {
		f.forget(id)
	}
	active := map[string]bool{}
	for _, s := range srcs {
		if !s.Enabled {
			continue
		}
		active[s.ID] = true
		f.pollSource(s, now)
	}
	// Sources removed or off: nothing kept but their positions (removed ones
	// were forgotten above).
	for key := range f.pending {
		id, _, _ := strings.Cut(key, "|")
		if !active[id] {
			delete(f.pending, key)
		}
	}
	f.checkpointAll()
}

const (
	maxFileRead       = 4 << 20 // per file and per poll
	maxFilesPerGlob   = 200
	fileContinueDelay = 2 * time.Second // an entry waits this long for continuation lines
)

// rotatedName: copies left by logrotate (access.log.1, app.log.2.gz,
// app-20261005.log.gz…), not read when a glob matches them.
var rotatedName = regexp.MustCompile(`(\.\d+|\.gz|\.bz2|\.xz|\.zst|\.zip|\.lz4)$`)

// matches lists the files of a source.
func (s *fileSource) matches() ([]string, error) {
	if !strings.ContainsAny(s.Path, "*?[") {
		fi, err := os.Stat(s.Path)
		if err != nil {
			return nil, err
		}
		if !fi.Mode().IsRegular() {
			return nil, fmt.Errorf("%s is not a regular file", s.Path)
		}
		return []string{s.Path}, nil
	}
	paths, err := filepath.Glob(s.Path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range paths {
		if rotatedName.MatchString(p) {
			continue
		}
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	if len(out) > maxFilesPerGlob {
		out = out[:maxFilesPerGlob]
	}
	if len(out) == 0 {
		// Tell a missing directory (not mounted) from an empty one.
		if _, err := os.Stat(filepath.Dir(s.Path)); err != nil && !strings.ContainsAny(filepath.Dir(s.Path), "*?[") {
			return nil, err
		}
		return nil, fs.ErrNotExist
	}
	return out, nil
}

func (f *Files) setStatus(id string, files int, err error) {
	code, detail := "", ""
	if err != nil {
		code, detail = errCode(err), err.Error()
		if errors.Is(err, fs.ErrNotExist) {
			code = "not_found"
		}
	}
	f.mu.Lock()
	st := f.status[id]
	if st == nil {
		st = &fileSourceStatus{}
		f.status[id] = st
	}
	changed := code != st.Code
	st.Files, st.Code, st.Error = files, code, detail
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
	if st := f.status[id]; st != nil {
		st.Read++
	}
	f.mu.Unlock()
}

func (f *Files) pollSource(s *fileSource, now time.Time) {
	first := false
	if _, ok := f.st.Started[s.ID]; !ok {
		f.st.Started[s.ID], f.dirty, first = now, true, true
	}
	paths, err := s.matches()
	if err != nil {
		f.setStatus(s.ID, 0, err)
		return
	}
	prefix := s.ID + "|"
	matched := map[string]bool{}
	for _, p := range paths {
		matched[posKey(s.ID, p)] = true
	}
	// Files that left the glob, by inode: a file renamed within it goes on
	// from where it was.
	gone := map[uint64]string{}
	for key, pos := range f.st.Pos {
		if strings.HasPrefix(key, prefix) && !matched[key] {
			gone[pos.Ino] = key
		}
	}
	var firstErr error
	read := 0
	for _, p := range paths {
		if err := f.pollFile(s, p, first, gone, now); err != nil {
			firstErr = keepFirst(firstErr, err)
			continue
		}
		read++
	}
	for key := range f.st.Pos {
		if strings.HasPrefix(key, prefix) && !matched[key] {
			f.flushPending(key, now)
			delete(f.st.Pos, key)
			delete(f.readOff, key)
			f.dirty = true
		}
	}
	if read > 0 {
		firstErr = nil
	}
	f.setStatus(s.ID, read, firstErr)
}

func (f *Files) pollFile(s *fileSource, path string, first bool, gone map[uint64]string, now time.Time) error {
	key := posKey(s.ID, path)
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	ino := inodeOf(fi)
	pos, known := f.st.Pos[key]
	if !known {
		if old, ok := gone[ino]; ok && ino != 0 {
			// Renamed within the glob.
			pos, known = f.st.Pos[old], true
			if off, ok := f.readOff[old]; ok {
				f.readOff[key] = off
			}
			if pe := f.pending[old]; pe != nil {
				f.pending[key] = pe
			}
			delete(f.st.Pos, old)
			delete(f.readOff, old)
			delete(f.pending, old)
			delete(gone, ino)
		} else if first {
			// Turned on: the last FILES_BACKFILL of the files already there
			// (0: only new lines).
			pos = filePos{Ino: ino}
			if f.backfill > 0 {
				pos.Until = fi.Size()
			} else {
				pos.Off = fi.Size()
			}
		} else {
			pos = filePos{Ino: ino} // new file: read from its start
		}
		f.st.Pos[key], f.dirty = pos, true
	}
	off, ok := f.readOff[key]
	if !ok {
		off = pos.Off
	}
	if pos.Ino != ino {
		// Rotated (access.log -> access.log.1): finish the old file, then
		// read the new one from its start.
		if old, err := os.Stat(path + ".1"); err == nil && inodeOf(old) == pos.Ino {
			for {
				next, more, err := f.readFrom(s, key, path+".1", off, pos.Until, now)
				if err != nil || !more {
					break
				}
				off = next
			}
		}
		f.flushPending(key, now)
		pos, off = filePos{Ino: ino}, 0
		f.st.Pos[key], f.dirty = pos, true
	} else if fi.Size() < off {
		f.flushPending(key, now) // truncated (copytruncate)
		pos.Until, off = 0, 0
		f.st.Pos[key], f.dirty = pos, true
	}
	if fi.Size() > off {
		next, _, err := f.readFrom(s, key, path, off, pos.Until, now)
		if err != nil {
			f.readOff[key] = off
			return err
		}
		off = next
	}
	f.readOff[key] = off
	if pe := f.pending[key]; pe != nil && now.Sub(pe.seen) >= fileContinueDelay {
		f.flushPending(key, now)
	}
	return nil
}

// readFrom parses the complete lines written after off and returns the new
// offset; more tells whether the read window was full.
func (f *Files) readFrom(s *fileSource, key, path string, off, until int64, now time.Time) (int64, bool, error) {
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
		f.line(s, key, path, string(b), start, start < until, now)
		start += int64(len(b)) + 1
	}
	return off + int64(end) + 1, full, nil
}

// line handles one line: a continuation of the pending entry, or a new one.
func (f *Files) line(s *fileSource, key, path, line string, off int64, history bool, now time.Time) {
	if strings.TrimSpace(line) == "" {
		return
	}
	pe := f.pending[key]
	if s.parser.multiline() && pe != nil && s.parser.continuation(line) {
		if pe.lines < maxContinuation && len(pe.rec.msg) < maxFileMsg {
			pe.rec.msg += "\n" + strings.TrimRight(cleanLine(line), " \t")
			pe.lines++
		}
		pe.seen = now
		return
	}
	f.flushPending(key, now)
	rec := s.parser.parse(line, now)
	pe = &pendingEntry{rec: rec, off: off, seen: now, history: history}
	if s.parser.multiline() {
		f.pending[key] = pe
		return
	}
	f.emit(s, path, pe, now)
}

func (f *Files) flushPending(key string, now time.Time) {
	pe := f.pending[key]
	if pe == nil {
		return
	}
	delete(f.pending, key)
	id, path, _ := strings.Cut(key, "|")
	f.mu.Lock()
	var src *fileSource
	for _, s := range f.cfg {
		if s.ID == id {
			src = s
		}
	}
	f.mu.Unlock()
	if src != nil {
		f.emit(src, path, pe, now)
	}
}

// emit sends an entry; history entries are kept only when dated within
// FILES_BACKFILL of when the source was turned on.
func (f *Files) emit(s *fileSource, path string, pe *pendingEntry, now time.Time) {
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
	extra := map[string]string{"log_file": path}
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
	for key, off := range f.readOff {
		pos, ok := f.st.Pos[key]
		if !ok {
			continue
		}
		if pe := f.pending[key]; pe != nil && pe.off < off {
			off = pe.off
		}
		if off != pos.Off {
			pos.Off, f.dirty = off, true
			f.st.Pos[key] = pos
		}
	}
}
