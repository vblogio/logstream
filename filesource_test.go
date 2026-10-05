package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func mustParser(t *testing.T, format, expr string) *fileParser {
	t.Helper()
	p, err := newFileParser(format, expr)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseLogTime(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	utc := func(s string) time.Time { t, _ := time.Parse(time.RFC3339Nano, s); return t }
	tests := []struct {
		in   string
		want time.Time
	}{
		{"2026-10-05T10:00:00.123Z", utc("2026-10-05T10:00:00.123Z")},
		{"2026-10-05T10:00:00+02:00", utc("2026-10-05T08:00:00Z")},
		{"2026-10-05 10:00:00,250 +0200", utc("2026-10-05T08:00:00.25Z")},
		{"2026/10/05 10:00:00Z", utc("2026-10-05T10:00:00Z")},
		{"05/Oct/2026:10:00:00 +0200", utc("2026-10-05T08:00:00Z")},
		{"1759572000", time.Unix(1759572000, 0)},
		{"1759572000123", time.UnixMilli(1759572000123)},
	}
	for _, tt := range tests {
		got, ok := parseLogTime(tt.in, now)
		if !ok || !got.Equal(tt.want) {
			t.Errorf("%q: got %v %v, want %v", tt.in, got, ok, tt.want)
		}
	}
	// Syslog timestamps: no year, the last one before tomorrow.
	if got, ok := parseLogTime("Dec 31 23:00:00", now); !ok || got.Year() != 2025 {
		t.Errorf("Dec 31: %v %v", got, ok)
	}
	if got, ok := parseLogTime("Oct  5 11:00:00", now); !ok || got.Year() != 2026 || got.Day() != 5 {
		t.Errorf("Oct 5: %v %v", got, ok)
	}
	for _, bad := range []string{"", "hello", "200", "12345"} {
		if _, ok := parseLogTime(bad, now); ok {
			t.Errorf("%q parsed", bad)
		}
	}
}

func TestParseText(t *testing.T) {
	now := time.Now()
	p := mustParser(t, fmtText, "")
	r := p.parse("2026-10-05 10:00:00.5 ERROR db: connection refused", now)
	if !r.dated || r.msg != "ERROR db: connection refused" || r.sev != 3 {
		t.Errorf("got %+v", r)
	}
	r = p.parse("[2026-10-05T10:00:00Z] - started", now)
	if !r.dated || r.msg != "started" || r.sev != 6 {
		t.Errorf("got %+v", r)
	}
	r = p.parse("\x1b[33mWARN\x1b[0m no timestamp here", now)
	if r.dated || r.msg != "WARN no timestamp here" || r.sev != 4 || !r.t.Equal(now) {
		t.Errorf("got %+v", r)
	}
}

func TestParseJSON(t *testing.T) {
	now := time.Now()
	p := mustParser(t, fmtJSON, "")
	r := p.parse(`{"time":"2026-10-05T10:00:00Z","level":"warn","msg":"slow query","host":"db.example","req":{"id":7,"tags":["a"]},"ok":true}`, now)
	if !r.dated || r.sev != 4 || r.msg != "slow query" {
		t.Errorf("got %+v", r)
	}
	for k, want := range map[string]string{"field_host": "db.example", "req.id": "7", "req.tags": `["a"]`, "ok": "true"} {
		if r.fields[k] != want {
			t.Errorf("%s = %q, want %q", k, r.fields[k], want)
		}
	}
	// pino: numeric level, epoch in ms.
	r = p.parse(`{"level":50,"time":1759572000123,"msg":"boom"}`, now)
	if r.sev != 3 || !r.t.Equal(time.UnixMilli(1759572000123)) {
		t.Errorf("got %+v", r)
	}
	// No message key: the whole line; not JSON: text.
	line := `{"event":"login","user":"bob"}`
	if r = p.parse(line, now); r.msg != line || r.fields["user"] != "bob" {
		t.Errorf("got %+v", r)
	}
	if r = p.parse("plain ERROR line", now); r.msg != "plain ERROR line" || r.sev != 3 {
		t.Errorf("got %+v", r)
	}
}

func TestParseLogfmt(t *testing.T) {
	p := mustParser(t, fmtLogfmt, "")
	r := p.parse(`time="2026-10-05T10:00:00Z" level=error msg="Cannot connect: \"docker\" down" providerName=docker flag`, time.Now())
	if !r.dated || r.sev != 3 || r.msg != `Cannot connect: "docker" down` || r.fields["providerName"] != "docker" {
		t.Errorf("got %+v", r)
	}
}

func TestParseRegex(t *testing.T) {
	if _, err := newFileParser(fmtRegex, "("); err == nil {
		t.Error("invalid regex accepted")
	}
	if _, err := newFileParser(fmtRegex, ""); err == nil {
		t.Error("empty regex accepted")
	}
	p := mustParser(t, fmtRegex, `^(?P<time>\S+ \S+) \[(?P<level>\w+)\] (?P<module>[\w.]+): (?P<msg>.*)$`)
	r := p.parse("2026-10-05 10:00:00 [warning] app.db: pool exhausted", time.Now())
	if !r.dated || r.sev != 4 || r.msg != "pool exhausted" || r.fields["module"] != "app.db" {
		t.Errorf("got %+v", r)
	}
	if !p.continuation("Traceback (most recent call last):") || p.continuation("2026-10-05 10:00:01 [info] a: b") {
		t.Error("continuation")
	}
}

func TestParseCLF(t *testing.T) {
	p := mustParser(t, fmtCLF, "")
	r := p.parse(`203.0.113.9 - - [05/Oct/2026:10:00:00 +0200] "GET /api/x?y=1 HTTP/1.1" 502 157 "-" "curl/8.0" 4211 "api@docker" "http://172.18.0.5:8080" 31ms`, time.Now())
	if !r.dated || r.sev != 3 || !strings.HasPrefix(r.msg, "203.0.113.9 - - [") {
		t.Errorf("got %+v", r)
	}
	for k, want := range map[string]string{"client_ip": "203.0.113.9", "http_method": "GET", "http_path": "/api/x?y=1", "http_status": "502",
		"user_agent": "curl/8.0", "router": "api@docker", "backend": "http://172.18.0.5:8080", "duration_ms": "31"} {
		if r.fields[k] != want {
			t.Errorf("%s = %q, want %q", k, r.fields[k], want)
		}
	}
	if _, ok := r.fields["http_referer"]; ok {
		t.Error("empty referer kept")
	}
	// nginx combined, 404.
	r = p.parse(`10.0.0.1 - alice [05/Oct/2026:10:00:00 +0000] "POST /login HTTP/2.0" 404 0 "https://example.com/" "Mozilla/5.0"`, time.Now())
	if r.sev != 4 || r.fields["http_user"] != "alice" || r.fields["http_referer"] != "https://example.com/" {
		t.Errorf("got %+v", r)
	}
}

func TestParseTraefikJSON(t *testing.T) {
	p := mustParser(t, fmtTraefik, "")
	r := p.parse(`{"ClientHost":"203.0.113.9","DownstreamStatus":200,"Duration":12500000,"RequestHost":"app.example.com","RequestMethod":"GET","RequestPath":"/","RequestProtocol":"HTTP/2.0","RouterName":"app@docker","StartUTC":"2026-10-05T10:00:00.1Z","entryPointName":"websecure","level":"info","msg":"","time":"2026-10-05T10:00:00Z","TLSVersion":"1.3"}`, time.Now())
	if !r.dated || r.sev != 6 || r.msg != "GET app.example.com/ 200 · 12.5 ms · app@docker · 203.0.113.9" {
		t.Errorf("got %+v", r)
	}
	for k, want := range map[string]string{"entrypoint": "websecure", "http_status": "200", "TLSVersion": "1.3", "duration_ms": "12.5"} {
		if r.fields[k] != want {
			t.Errorf("%s = %q, want %q", k, r.fields[k], want)
		}
	}
	// Traefik's own logs in the same file: generic JSON.
	r = p.parse(`{"level":"error","msg":"Provider error","time":"2026-10-05T10:00:00Z"}`, time.Now())
	if r.sev != 3 || r.msg != "Provider error" {
		t.Errorf("got %+v", r)
	}
}

func TestFileSourceValidation(t *testing.T) {
	f := NewFiles(t.TempDir(), t.TempDir(), time.Hour, func(*Entry) {})
	bad := []fileSourceDef{
		{Name: "", Path: "/logs/a.log"},
		{Name: "a", Path: "relative.log"},
		{Name: "a", Path: "/logs/[.log"},
		{Name: "a", Path: "/logs/a.log", Format: "xml"},
		{Name: "a", Path: "/logs/a.log", Format: fmtRegex, Regex: "(?P<x"},
	}
	for _, d := range bad {
		if err := f.Configure(filesConfig{Sources: []fileSourceDef{d}}); err == nil {
			t.Errorf("%+v accepted", d)
		}
	}
	if err := f.Configure(filesConfig{Sources: []fileSourceDef{{Name: " traefik ", Path: "/logs/traefik/*.log", Format: fmtCLF}}}); err != nil {
		t.Fatal(err)
	}
	src := f.Status()["sources"].([]map[string]any)
	if len(src) != 1 || src[0]["name"] != "traefik" || src[0]["id"] == "" {
		t.Errorf("got %+v", src)
	}
}

type fileSink struct {
	mu sync.Mutex
	e  []*Entry
}

func (s *fileSink) add(e *Entry) { s.mu.Lock(); s.e = append(s.e, e); s.mu.Unlock() }
func (s *fileSink) take() []*Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.e
	s.e = nil
	return e
}

func appendFile(t *testing.T, path, s string) {
	t.Helper()
	fh, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fh.WriteString(s); err != nil {
		t.Fatal(err)
	}
	fh.Close()
}

func msgs(es []*Entry) []string {
	var out []string
	for _, e := range es {
		out = append(out, e.Message)
	}
	return out
}

func TestFilesFollow(t *testing.T) {
	dir, data := t.TempDir(), t.TempDir()
	logs := filepath.Join(dir, "logs")
	os.Mkdir(logs, 0o755)
	now := time.Now()
	old := now.Add(-3 * time.Hour).UTC().Format(time.RFC3339)
	recent := now.Add(-10 * time.Minute).UTC().Format(time.RFC3339)
	app := filepath.Join(logs, "app.log")
	appendFile(t, app, old+" INFO too old\n"+recent+" ERROR recent\n  at line 1\nundated history line\n")
	appendFile(t, filepath.Join(logs, "app.log.1"), recent+" INFO rotated copy\n")

	sink := &fileSink{}
	f := NewFiles(dir, data, time.Hour, sink.add)
	if err := f.Configure(filesConfig{Sources: []fileSourceDef{{Name: "myapp", Path: filepath.Join(logs, "*.log"), Format: fmtText, Host: "web1", Enabled: true}}}); err != nil {
		t.Fatal(err)
	}
	f.poll(now)
	f.poll(now.Add(3 * time.Second)) // the pending entry waited for continuation lines
	got := sink.take()
	if len(got) != 1 || got[0].Message != "ERROR recent\n  at line 1" || got[0].SevNum != 3 || got[0].App != "myapp" ||
		got[0].Host != "web1" || got[0].SourceType != "file" || got[0].Extra["log_file"] != app {
		t.Fatalf("history: got %q %+v", msgs(got), got)
	}

	// New lines, a new file in the glob, then a rotation.
	appendFile(t, app, "plain new line\n\tcontinued\n")
	appendFile(t, filepath.Join(logs, "other.log"), "first of other\n")
	f.poll(now.Add(4 * time.Second))
	appendFile(t, app, "after\n")
	f.poll(now.Add(5 * time.Second))
	f.poll(now.Add(8 * time.Second))
	got = sink.take()
	m := msgs(got)
	sort.Strings(m)
	if strings.Join(m, "|") != "after|first of other|plain new line\n\tcontinued" {
		t.Fatalf("new lines: got %q", msgs(got))
	}

	appendFile(t, app, "before rotation\n")
	if err := os.Rename(app, app+".1"); err != nil {
		t.Fatal(err)
	}
	appendFile(t, app, "after rotation\n")
	f.poll(now.Add(9 * time.Second))
	f.poll(now.Add(12 * time.Second))
	if got = sink.take(); strings.Join(msgs(got), "|") != "before rotation|after rotation" {
		t.Fatalf("rotation: got %q", msgs(got))
	}

	// Restart: resumes where it stopped, nothing sent twice.
	f.checkpointAll()
	f.dirty = true
	f.saveState()
	appendFile(t, app, "while stopped\n")
	f2 := NewFiles(dir, data, time.Hour, sink.add)
	f2.poll(now.Add(13 * time.Second))
	f2.poll(now.Add(16 * time.Second))
	if got = sink.take(); strings.Join(msgs(got), "|") != "while stopped" {
		t.Fatalf("restart: got %q", msgs(got))
	}

	// Truncated in place (copytruncate).
	os.Truncate(app, 0)
	appendFile(t, app, "x\n")
	f2.poll(now.Add(17 * time.Second))
	f2.poll(now.Add(20 * time.Second))
	if got = sink.take(); strings.Join(msgs(got), "|") != "x" {
		t.Fatalf("truncate: got %q", msgs(got))
	}
	st := f2.Status()["sources"].([]map[string]any)[0]
	if st["files"] != 2 || st["code"] != "" {
		t.Errorf("status %+v", st)
	}
}

func TestFilesJSONAndErrors(t *testing.T) {
	dir, data := t.TempDir(), t.TempDir()
	path := filepath.Join(dir, "access.json")
	sink := &fileSink{}
	f := NewFiles(dir, data, 0, sink.add) // FILES_BACKFILL=0: only new lines
	appendFile(t, path, `{"msg":"existing"}`+"\n")
	cfg := filesConfig{Sources: []fileSourceDef{
		{ID: "a1", Name: "api", Path: path, Format: fmtJSON, Enabled: true},
		{ID: "b2", Name: "missing", Path: filepath.Join(dir, "nope.log"), Enabled: true},
	}}
	if err := f.Configure(cfg); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	f.poll(now)
	appendFile(t, path, `{"msg":"one","level":"error"}`+"\n"+`{"msg":"two"}`+"\n"+`{"msg":"partial`)
	f.poll(now.Add(time.Second))
	got := sink.take()
	if strings.Join(msgs(got), "|") != "one|two" || got[0].SevNum != 3 {
		t.Fatalf("got %q", msgs(got))
	}
	src := f.Status()["sources"].([]map[string]any)
	if src[0]["files"] != 1 || src[1]["code"] != "not_found" {
		t.Errorf("status %+v", src)
	}

	// Turned off then on: starts over (only new lines here).
	cfg.Sources[0].Enabled = false
	f.Configure(cfg)
	f.poll(now.Add(2 * time.Second))
	appendFile(t, path, `"}`+"\n")
	cfg.Sources[0].Enabled = true
	f.Configure(cfg)
	f.poll(now.Add(3 * time.Second))
	appendFile(t, path, `{"msg":"three"}`+"\n")
	f.poll(now.Add(4 * time.Second))
	if got = sink.take(); strings.Join(msgs(got), "|") != "three" {
		t.Fatalf("re-enabled: got %q", msgs(got))
	}
}
