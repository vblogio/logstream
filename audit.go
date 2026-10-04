package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
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

// auditConfig is saved in /data/audit.json and edited in Settings > Sources.
type auditConfig struct {
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"` // empty: AUDIT_LOG (/var/log/audit/audit.log)
}

// auditPos is where reading resumes: the inode of the file and an offset
// before which every event was sent.
type auditPos struct {
	Ino uint64 `json:"ino"`
	Off int64  `json:"off"`
}

// Audit follows the log of the Linux audit daemon (auditd) and sends one
// entry per audit event: the records of an event (SYSCALL, EXECVE, PATH…)
// share the same msg=audit(time:serial) and are grouped together.
type Audit struct {
	defPath   string
	hostRoot  string // to read the host's /etc/passwd and /etc/hostname when mounted
	hostName  string // AUDIT_HOST: host name of the entries when records have no node=
	sink      func(*Entry)
	backfill  time.Duration
	cfgPath   string
	statePath string
	wake      chan struct{}

	// Owned by the Run goroutine.
	pos       auditPos // checkpoint saved in /data
	readOff   int64    // where the next read starts (after the lines of open events)
	known     bool
	dirty     bool
	notBefore time.Time
	asm       *auditAssembler
	users     *userNames

	mu        sync.Mutex
	cfg       auditConfig
	reset     bool
	open      bool
	read      uint64
	errCode   string
	errDetail string
}

func NewAudit(defPath, hostRoot, hostName, dataDir string, backfill time.Duration, sink func(*Entry)) *Audit {
	a := &Audit{
		defPath:   defPath,
		hostRoot:  hostRoot,
		hostName:  hostName,
		sink:      sink,
		backfill:  backfill,
		cfgPath:   filepath.Join(dataDir, "audit.json"),
		statePath: filepath.Join(dataDir, "audit-state.json"),
		wake:      make(chan struct{}, 1),
		asm:       newAuditAssembler(),
		users:     &userNames{path: filepath.Join(hostRoot, "etc/passwd")},
	}
	if b, err := os.ReadFile(a.cfgPath); err == nil {
		_ = json.Unmarshal(b, &a.cfg)
	}
	if b, err := os.ReadFile(a.statePath); err == nil && json.Unmarshal(b, &a.pos) == nil {
		a.known, a.readOff = true, a.pos.Off
	}
	if a.hostName == "" {
		a.hostName = readTrim(filepath.Join(hostRoot, "etc/hostname"))
	}
	return a
}

func (a *Audit) path() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cfg.Path != "" {
		return a.cfg.Path
	}
	return a.defPath
}

// Run polls the file every second while the source is enabled.
func (a *Audit) Run(ctx context.Context) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	n := 0
	for {
		select {
		case <-ctx.Done():
			a.emit(a.asm.flushAll())
			a.checkpoint()
			a.saveState()
			return
		case <-tick.C:
		case <-a.wake:
		}
		a.mu.Lock()
		enabled, reset := a.cfg.Enabled, a.reset
		a.reset = false
		a.mu.Unlock()
		if reset {
			// Disabled then enabled again (or another file): start over from
			// AUDIT_BACKFILL ago rather than reading everything in between.
			a.asm = newAuditAssembler()
			a.pos, a.readOff, a.known, a.dirty = auditPos{}, 0, false, true
		}
		if enabled {
			a.poll(time.Now())
		}
		if n++; n%5 == 0 || reset {
			a.saveState()
		}
	}
}

func (a *Audit) saveState() {
	if !a.dirty {
		return
	}
	a.dirty = false
	if err := writeJSONFile(a.statePath, a.pos); err != nil {
		logEvent("audit", 4, "saving position: %v", err)
	}
}

func (a *Audit) fail(err error) {
	code := "read"
	switch {
	case errors.Is(err, fs.ErrNotExist):
		code = "not_found"
	case errors.Is(err, fs.ErrPermission):
		code = "permission"
	}
	a.mu.Lock()
	changed := code != a.errCode
	a.open, a.errCode, a.errDetail = false, code, err.Error()
	a.mu.Unlock()
	if changed {
		logEvent("audit", 4, "cannot read the audit log: %v", err)
	}
}

const maxAuditRead = 4 << 20 // per poll

// poll reads the lines added since the previous one, follows rotations and
// sends the events that are complete.
func (a *Audit) poll(now time.Time) {
	path := a.path()
	fi, err := os.Stat(path)
	if err != nil {
		a.fail(err)
		return
	}
	ino := inodeOf(fi)
	if !a.known {
		// First read: the last AUDIT_BACKFILL (0: only new events).
		a.pos, a.readOff, a.known, a.dirty = auditPos{Ino: ino}, 0, true, true
		a.notBefore = time.Time{}
		if a.backfill <= 0 {
			a.readOff = fi.Size()
		} else {
			a.notBefore = now.Add(-a.backfill)
		}
	}
	if a.pos.Ino != ino {
		// Rotated by auditd (audit.log -> audit.log.1): finish the old file,
		// then read the new one from its start.
		if old, err := os.Stat(path + ".1"); err == nil && inodeOf(old) == a.pos.Ino {
			for {
				off, more, err := a.readFrom(path+".1", a.readOff, now)
				if err != nil || !more {
					break
				}
				a.readOff = off
			}
		}
		a.emit(a.asm.flushAll())
		a.pos, a.readOff, a.dirty = auditPos{Ino: ino}, 0, true
	} else if fi.Size() < a.readOff {
		a.emit(a.asm.flushAll()) // truncated
		a.readOff = 0
	}
	if fi.Size() > a.readOff {
		off, _, err := a.readFrom(path, a.readOff, now)
		if err != nil {
			a.fail(err)
			return
		}
		a.readOff = off
	}
	a.mu.Lock()
	if !a.open {
		a.open, a.errCode, a.errDetail = true, "", ""
	}
	a.mu.Unlock()
	a.emit(a.asm.flush(now, auditCloseDelay))
	a.checkpoint()
}

// checkpoint saves an offset before which every event is sent: the start of
// the oldest event still open, or the read offset.
func (a *Audit) checkpoint() {
	off := a.readOff
	if o, ok := a.asm.minOff(); ok && o < off {
		off = o
	}
	if off != a.pos.Off {
		a.pos.Off, a.dirty = off, true
	}
}

// readFrom feeds the complete lines written after off to the assembler and
// returns the new offset; more tells whether the read window was full.
func (a *Audit) readFrom(path string, off int64, now time.Time) (int64, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return off, false, err
	}
	defer f.Close()
	buf, err := io.ReadAll(io.NewSectionReader(f, off, maxAuditRead))
	if err != nil {
		return off, false, err
	}
	full := len(buf) == maxAuditRead
	end := bytes.LastIndexByte(buf, '\n')
	if end < 0 {
		if !full {
			return off, false, nil // partial line: wait for its end
		}
		end = len(buf) - 1
	}
	start := off
	for _, line := range bytes.Split(buf[:end], []byte{'\n'}) {
		a.emit(a.asm.add(string(line), start, now))
		start += int64(len(line)) + 1
	}
	return off + int64(end) + 1, full, nil
}

func (a *Audit) emit(evs []*auditEvent) {
	for _, ev := range evs {
		if !a.notBefore.IsZero() && ev.Time.Before(a.notBefore) {
			continue
		}
		a.sink(auditEntry(ev, a.users, a.hostName, time.Now()))
		a.mu.Lock()
		a.read++
		a.mu.Unlock()
	}
}

func inodeOf(fi os.FileInfo) uint64 {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Ino)
	}
	return 0
}

// Configure saves and applies the configuration.
func (a *Audit) Configure(cfg auditConfig) error {
	cfg.Path = strings.TrimSpace(cfg.Path)
	a.mu.Lock()
	if (a.cfg.Enabled && !cfg.Enabled) || cfg.Path != a.cfg.Path {
		a.reset = true
		a.open, a.errCode, a.errDetail = false, "", ""
	}
	a.cfg = cfg
	a.mu.Unlock()
	select {
	case a.wake <- struct{}{}:
	default:
	}
	return writeJSONFile(a.cfgPath, cfg)
}

// Status is the state shown in Settings > Sources.
func (a *Audit) Status() map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()
	return map[string]any{
		"enabled":     a.cfg.Enabled,
		"path":        a.cfg.Path,
		"defaultPath": a.defPath,
		"open":        a.open && a.cfg.Enabled,
		"read":        a.read,
		"code":        a.errCode,
		"error":       a.errDetail,
	}
}

/* ---------- Records and events ---------- */

// auditCloseDelay: an event is complete when no line of it came for this
// long, or when the log has moved this far past its time (auditd does not
// write the EOE record that ends multi-record events).
const auditCloseDelay = 2 * time.Second

type auditField struct {
	k, v string
	bare bool // written without quotes: may be hexadecimal
}

type auditRecord struct {
	Type   string
	Fields []auditField
	Raw    string
}

type auditEvent struct {
	ID      string // "1700000000.123:456"
	Serial  string
	Time    time.Time
	Node    string
	Records []auditRecord
	off     int64     // offset of its first line in the file
	seen    time.Time // when its last line was read
	order   int
}

// parseAuditLine decodes "[node=N ]type=T msg=audit(SEC.MS:SERIAL): k=v …".
func parseAuditLine(line string) (rec auditRecord, id, node string, ok bool) {
	line = strings.TrimRight(line, "\r")
	rec.Raw = line
	head, rest, found := strings.Cut(line, "): ")
	if !found {
		head, found = strings.CutSuffix(line, ")")
		if !found {
			return rec, "", "", false
		}
	}
	pre, ts, found := strings.Cut(head, "msg=audit(")
	if !found || ts == "" {
		return rec, "", "", false
	}
	for _, f := range parseAuditKV(pre) {
		switch f.k {
		case "type":
			rec.Type = f.v
		case "node":
			node = f.v
		}
	}
	if rec.Type == "" {
		return rec, "", "", false
	}
	// Enriched format (log_format=ENRICHED): resolved values after a GS byte.
	main, enriched, _ := strings.Cut(rest, "\x1d")
	rec.Fields = parseAuditKV(main)
	for _, f := range parseAuditKV(enriched) {
		rec.Fields = append(rec.Fields, auditField{k: "@" + f.k, v: f.v})
	}
	return rec, ts, node, true
}

// hexFields are the fields auditd writes in hexadecimal when they contain
// spaces, quotes or control characters (and then without quotes).
var hexFields = map[string]bool{
	"proctitle": true, "cmd": true, "exe": true, "comm": true, "key": true, "acct": true,
	"name": true, "cwd": true, "data": true, "path": true, "old": true, "new": true,
}

// parseAuditKV splits "k=v k2="v 2" msg='k3=v3 …'"; the fields of a quoted
// msg='…' (user space records) follow the outer ones.
func parseAuditKV(s string) []auditField {
	var out []auditField
	var inner []auditField
	for i := 0; i < len(s); {
		for i < len(s) && s[i] == ' ' {
			i++
		}
		eq := strings.IndexByte(s[i:], '=')
		if eq <= 0 {
			break
		}
		k := s[i : i+eq]
		if sp := strings.IndexByte(k, ' '); sp >= 0 { // word without value
			i += sp + 1
			continue
		}
		i += eq + 1
		var v string
		quoted := i < len(s) && (s[i] == '"' || s[i] == '\'')
		if quoted {
			q := s[i]
			end := strings.IndexByte(s[i+1:], q)
			if end < 0 {
				end = len(s) - i - 1
			}
			v = s[i+1 : i+1+end]
			i += end + 2
		} else {
			end := strings.IndexByte(s[i:], ' ')
			if end < 0 {
				end = len(s) - i
			}
			v = s[i : i+end]
			i += end
		}
		if k == "msg" && quoted && strings.Contains(v, "=") {
			inner = append(inner, parseAuditKV(v)...)
			continue
		}
		if !quoted && hexFields[k] {
			v = decodeHex(k, v)
		}
		if !quoted && (v == "(null)" || v == "?") {
			v = ""
		}
		out = append(out, auditField{k: k, v: v, bare: !quoted})
	}
	return append(out, inner...)
}

// decodeHex decodes a hexadecimal value, or returns it unchanged.
func decodeHex(k, v string) string {
	if len(v) < 2 || len(v)%2 != 0 {
		return v
	}
	b, err := hex.DecodeString(v)
	if err != nil {
		return v
	}
	if k == "proctitle" {
		b = bytes.ReplaceAll(bytes.TrimRight(b, "\x00"), []byte{0}, []byte{' '})
	}
	if !utf8.Valid(b) {
		return v
	}
	return string(b)
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

// parseAuditTime decodes "1700000000.123:456" into a time and a serial.
func parseAuditTime(id string) (time.Time, string) {
	ts, serial, _ := strings.Cut(id, ":")
	sec, frac, _ := strings.Cut(ts, ".")
	s, err := strconv.ParseInt(sec, 10, 64)
	if err != nil {
		return time.Time{}, serial
	}
	ms, _ := strconv.ParseInt((frac + "000")[:3], 10, 64)
	return time.Unix(s, ms*int64(time.Millisecond)), serial
}

// auditAssembler groups the records of each event.
type auditAssembler struct {
	open   map[string]*auditEvent
	newest time.Time // most recent event time read
	n      int
}

func newAuditAssembler() *auditAssembler {
	return &auditAssembler{open: map[string]*auditEvent{}}
}

const maxOpenAudit = 2000

// add takes one line read at offset off and returns the events it completes
// (EOE record, or too many events open).
func (a *auditAssembler) add(line string, off int64, now time.Time) []*auditEvent {
	if strings.TrimSpace(line) == "" {
		return nil
	}
	rec, id, node, ok := parseAuditLine(line)
	if !ok {
		return nil
	}
	ev := a.open[id]
	if rec.Type == "EOE" {
		if ev == nil {
			return nil
		}
		delete(a.open, id)
		return []*auditEvent{ev}
	}
	if ev == nil {
		t, serial := parseAuditTime(id)
		a.n++
		ev = &auditEvent{ID: id, Serial: serial, Time: t, Node: node, off: off, order: a.n}
		a.open[id] = ev
		if t.After(a.newest) {
			a.newest = t
		}
	}
	ev.Records = append(ev.Records, rec)
	ev.seen = now
	if len(a.open) > maxOpenAudit {
		return a.closeOldest(len(a.open) - maxOpenAudit/2)
	}
	return nil
}

// flush returns the events with no new line for delay, or that the log has
// left delay behind, in the order they started.
func (a *auditAssembler) flush(now time.Time, delay time.Duration) []*auditEvent {
	var out []*auditEvent
	for id, ev := range a.open {
		if now.Sub(ev.seen) >= delay || a.newest.Sub(ev.Time) >= delay {
			out = append(out, ev)
			delete(a.open, id)
		}
	}
	sortEvents(out)
	return out
}

func (a *auditAssembler) flushAll() []*auditEvent {
	return a.closeOldest(len(a.open))
}

func (a *auditAssembler) closeOldest(n int) []*auditEvent {
	all := make([]*auditEvent, 0, len(a.open))
	for _, ev := range a.open {
		all = append(all, ev)
	}
	sortEvents(all)
	all = all[:n]
	for _, ev := range all {
		delete(a.open, ev.ID)
	}
	return all
}

func sortEvents(evs []*auditEvent) {
	sort.Slice(evs, func(i, j int) bool { return evs[i].order < evs[j].order })
}

func (a *auditAssembler) minOff() (int64, bool) {
	var m int64
	ok := false
	for _, ev := range a.open {
		if !ok || ev.off < m {
			m, ok = ev.off, true
		}
	}
	return m, ok
}

/* ---------- Entries ---------- */

// get returns the first non-empty value of a field across the records.
func (ev *auditEvent) get(k string) string {
	for _, r := range ev.Records {
		for _, f := range r.Fields {
			if f.k == k && f.v != "" {
				return f.v
			}
		}
	}
	return ""
}

func (ev *auditEvent) record(typ string) *auditRecord {
	for i := range ev.Records {
		if ev.Records[i].Type == typ {
			return &ev.Records[i]
		}
	}
	return nil
}

// result is "success", "failed" or "" from res= or success=.
func (ev *auditEvent) result() string {
	v := strings.ToLower(firstNonEmpty(ev.get("res"), ev.get("success"), ev.get("result")))
	switch v {
	case "success", "yes", "1":
		return "success"
	case "failed", "fail", "failure", "no", "0":
		return "failed"
	}
	return ""
}

const auditUnset = "4294967295"

// userOf resolves a uid: enriched name, then the host's /etc/passwd, then the number.
func (ev *auditEvent) userOf(field string, users *userNames) string {
	uid := ev.get(field)
	if uid == "" || uid == auditUnset || uid == "-1" {
		return ""
	}
	if n := strings.Trim(ev.get("@"+strings.ToUpper(field)), `"`); n != "" && n != "unset" && n != uid {
		return n
	}
	if n := users.name(uid); n != "" {
		return n
	}
	return uid
}

// auditEntry turns an event into an Entry: app is the type of its first record.
func auditEntry(ev *auditEvent, users *userNames, hostName string, now time.Time) *Entry {
	typ := "UNKNOWN"
	if len(ev.Records) > 0 {
		typ = ev.Records[0].Type
	}
	res := ev.result()
	auid := ev.get("auid")
	user := ev.userOf("auid", users)
	if user == "" {
		user = ev.userOf("uid", users)
	}
	sev := 6
	if res == "failed" {
		sev = 4
	}
	raw := make([]string, len(ev.Records))
	for i, r := range ev.Records {
		raw[i] = r.Raw
	}
	host := firstNonEmpty(ev.Node, hostName, "localhost")
	e := &Entry{
		Time:       ev.Time,
		Received:   now,
		Host:       host,
		App:        typ,
		ProcID:     ev.get("pid"),
		Facility:   "authpriv",
		Severity:   severityNames[sev],
		SevNum:     sev,
		Message:    auditSummary(ev, typ, user, users, res),
		Proto:      "file",
		SourceType: "audit",
		Extra: map[string]string{
			"audit_type":   typ,
			"audit_serial": ev.Serial,
			"auid":         auid,
			"user":         user,
			"exe":          ev.get("exe"),
			"res":          res,
			"key":          ev.get("key"),
			"audit_raw":    strings.Join(raw, "\n"),
		},
		Wait: true,
	}
	if e.Time.IsZero() {
		e.Time = now
	}
	return e
}

// auditSummary writes "who: what [key] → result".
func auditSummary(ev *auditEvent, typ, user string, users *userNames, res string) string {
	who := firstNonEmpty(user, "?")
	// Syscalls: the login user and, when different, the user the process ran
	// as (sudo, su). In user space records, uid is the daemon's (sshd: root).
	if uid := ev.userOf("uid", users); ev.get("syscall") != "" && uid != "" && user != "" && uid != user {
		who += " (as " + uid + ")"
	}
	var what string
	exe := ev.get("exe")
	switch {
	case typ == "USER_CMD":
		what = "sudo " + firstNonEmpty(ev.get("cmd"), "?")
		if cwd := ev.get("cwd"); cwd != "" {
			what += " (in " + cwd + ")"
		}
	case strings.HasPrefix(typ, "USER_") || strings.HasPrefix(typ, "CRED_") || typ == "LOGIN" ||
		typ == "ADD_USER" || typ == "DEL_USER" || typ == "ADD_GROUP" || typ == "DEL_GROUP":
		if acct := ev.get("acct"); acct != "" {
			who = acct
		}
		what = firstNonEmpty(ev.get("op"), strings.ToLower(typ))
		if exe != "" {
			what += " via " + exe
		}
		if addr := firstNonEmpty(ev.get("hostname"), ev.get("addr")); addr != "" {
			what += " from " + addr
		}
		if term := ev.get("terminal"); term != "" && term != "?" {
			what += " on " + term
		}
	case ev.record("EXECVE") != nil:
		what = "exec " + execLine(ev.record("EXECVE"))
	case ev.get("syscall") != "":
		what = "syscall " + firstNonEmpty(ev.get("@SYSCALL"), ev.get("syscall"))
		if name := pathName(ev); name != "" {
			what += " " + name
		}
		if prog := firstNonEmpty(exe, ev.get("comm")); prog != "" {
			what += " by " + prog
		}
	default:
		what = strings.ToLower(typ)
		if op := ev.get("op"); op != "" {
			what += " " + op
		}
		if exe != "" {
			what += " by " + exe
		}
	}
	if key := ev.get("key"); key != "" {
		what += " [" + key + "]"
	}
	if res != "" {
		what += " → " + res
	}
	return who + ": " + what
}

// execLine joins the arguments of an EXECVE record (a0, a1…).
func execLine(r *auditRecord) string {
	args := map[int]string{}
	argc := 0
	for _, f := range r.Fields {
		if f.k == "argc" {
			argc, _ = strconv.Atoi(f.v)
		} else if len(f.k) > 1 && f.k[0] == 'a' && isDigits(f.k[1:]) {
			n, _ := strconv.Atoi(f.k[1:])
			if f.bare {
				args[n] = decodeHex(f.k, f.v) // arguments with spaces or quotes
			} else {
				args[n] = f.v
			}
		}
	}
	if argc == 0 {
		argc = len(args)
	}
	parts := make([]string, 0, argc)
	for i := 0; i < argc; i++ {
		if v, ok := args[i]; ok {
			parts = append(parts, v)
		}
	}
	s := strings.Join(parts, " ")
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}

// pathName is the file a syscall worked on: the first PATH record that is not
// its parent directory.
func pathName(ev *auditEvent) string {
	var first string
	for _, r := range ev.Records {
		if r.Type != "PATH" {
			continue
		}
		var name, nametype string
		for _, f := range r.Fields {
			switch f.k {
			case "name":
				name = f.v
			case "nametype":
				nametype = f.v
			}
		}
		if name == "" {
			continue
		}
		if nametype != "PARENT" {
			return name
		}
		if first == "" {
			first = name
		}
	}
	return first
}

/* ---------- User names ---------- */

// userNames reads uid -> name from the host's /etc/passwd when it is mounted,
// at most once a minute.
type userNames struct {
	path    string
	names   map[string]string
	loaded  time.Time
	modTime time.Time
}

func (u *userNames) name(uid string) string {
	if u == nil {
		return ""
	}
	if time.Since(u.loaded) > time.Minute {
		u.loaded = time.Now()
		if fi, err := os.Stat(u.path); err == nil && !fi.ModTime().Equal(u.modTime) {
			u.modTime = fi.ModTime()
			u.names = readPasswd(u.path)
		}
	}
	return u.names[uid]
}

func readPasswd(path string) map[string]string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	names := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		p := strings.Split(sc.Text(), ":")
		if len(p) >= 3 && p[0] != "" && !strings.HasPrefix(p[0], "#") {
			names[p[2]] = p[0]
		}
	}
	return names
}
