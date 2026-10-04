package main

import (
	"context"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func hexOf(s string) string { return strings.ToUpper(hex.EncodeToString([]byte(s))) }

// Lines of real audit.log events (trimmed).
var (
	auditExec = []string{
		`type=SYSCALL msg=audit(1759572000.123:4242): arch=c000003e syscall=59 success=yes exit=0 a0=4142 a1=55d2 a2=55d3 a3=0 items=2 ppid=1000 pid=1234 auid=1000 uid=0 gid=0 euid=0 tty=pts0 ses=3 comm="apt" exe="/usr/bin/apt" subj=unconfined key="exec"`,
		`type=EXECVE msg=audit(1759572000.123:4242): argc=3 a0="apt" a1="install" a2=` + hexOf(`my "pkg"`),
		`type=CWD msg=audit(1759572000.123:4242): cwd="/root"`,
		`type=PATH msg=audit(1759572000.123:4242): item=0 name="/usr/bin/apt" inode=1 dev=fd:00 mode=0100755 ouid=0 ogid=0 nametype=NORMAL`,
		`type=PROCTITLE msg=audit(1759572000.123:4242): proctitle=` + hexOf("apt\x00install\x00my \"pkg\""),
	}
	auditSudo = `type=USER_CMD msg=audit(1759572001.500:4250): pid=2000 uid=1000 auid=1000 ses=3 subj=unconfined msg='cwd="/home/cedric" cmd=` + hexOf("systemctl restart nginx") + ` exe="/usr/bin/sudo" terminal=pts/0 res=success'`
	auditSSH  = `type=USER_AUTH msg=audit(1759572002.000:4260): pid=3000 uid=0 auid=4294967295 ses=4294967295 subj=unconfined msg='op=PAM:authentication grantors=? acct="bob" exe="/usr/sbin/sshd" hostname=203.0.113.9 addr=203.0.113.9 terminal=ssh res=failed'`
	auditOpen = []string{
		"node=web1 type=SYSCALL msg=audit(1759572003.250:4270): arch=c000003e syscall=257 success=no exit=-13 items=1 pid=4000 auid=1001 uid=1001 comm=\"cat\" exe=\"/usr/bin/cat\" key=\"shadow\"\x1dARCH=x86_64 SYSCALL=openat AUID=\"alice\" UID=\"alice\"",
		`node=web1 type=PATH msg=audit(1759572003.250:4270): item=0 name="/etc/shadow" inode=7 nametype=NORMAL`,
	}
)

func TestParseAuditLine(t *testing.T) {
	rec, id, node, ok := parseAuditLine(auditOpen[0])
	if !ok || rec.Type != "SYSCALL" || id != "1759572003.250:4270" || node != "web1" {
		t.Fatalf("got %+v %q %q %v", rec, id, node, ok)
	}
	ev := &auditEvent{Records: []auditRecord{rec}}
	for k, want := range map[string]string{"syscall": "257", "exe": "/usr/bin/cat", "key": "shadow", "@SYSCALL": "openat", "@AUID": "alice"} {
		if got := ev.get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	for _, bad := range []string{"", "garbage", "type=X no header", "msg=audit(1.2:3): no type"} {
		if _, _, _, ok := parseAuditLine(bad); ok {
			t.Errorf("%q parsed", bad)
		}
	}
}

func TestParseAuditKV(t *testing.T) {
	tests := []struct {
		in   string
		want map[string]string
	}{
		{`a=1 b="two words" c=`, map[string]string{"a": "1", "b": "two words", "c": ""}},
		{`pid=1 msg='op=login acct="bob" res=success'`, map[string]string{"pid": "1", "op": "login", "acct": "bob", "res": "success"}},
		{`comm=` + hexOf("my prog") + ` exe="/bin/x"`, map[string]string{"comm": "my prog", "exe": "/bin/x"}},
		{`key=(null) hostname=? terminal=?`, map[string]string{"key": "", "hostname": "", "terminal": ""}},
		{`a0=4142 name=ABC`, map[string]string{"a0": "4142", "name": "ABC"}}, // not hexadecimal fields, or odd length
		{`avc:  denied  { read } for  pid=12 comm="x"`, map[string]string{"pid": "12", "comm": "x"}},
	}
	for _, tt := range tests {
		got := map[string]string{}
		for _, f := range parseAuditKV(tt.in) {
			got[f.k] = f.v
		}
		for k, v := range tt.want {
			if got[k] != v {
				t.Errorf("%q: %s = %q, want %q (all: %v)", tt.in, k, got[k], v, got)
			}
		}
	}
}

func TestAuditAssembler(t *testing.T) {
	now := time.Unix(1759572010, 0)
	a := newAuditAssembler()
	// Two interleaved events, then an EOE that closes the first one.
	lines := []string{auditExec[0], auditSudo, auditExec[1], auditExec[2]}
	off := int64(0)
	for _, l := range lines {
		if out := a.add(l, off, now); out != nil {
			t.Fatalf("closed early: %v", out)
		}
		off += int64(len(l)) + 1
	}
	if len(a.open) != 2 {
		t.Fatalf("%d open events", len(a.open))
	}
	if o, ok := a.minOff(); !ok || o != 0 {
		t.Fatalf("minOff %d %v", o, ok)
	}
	out := a.add(`type=EOE msg=audit(1759572000.123:4242): `, off, now)
	if len(out) != 1 || out[0].Serial != "4242" || len(out[0].Records) != 3 {
		t.Fatalf("EOE: %+v", out)
	}
	if o, _ := a.minOff(); o != int64(len(lines[0])+1) {
		t.Fatalf("minOff after EOE: %d", o)
	}
	// An EOE of an unknown event is ignored.
	if out := a.add(`type=EOE msg=audit(1.0:1): `, 0, now); out != nil {
		t.Fatal(out)
	}
	// The sudo event is closed by time.
	if out := a.flush(now.Add(time.Second), 2*time.Second); len(out) != 0 {
		t.Fatalf("closed too early: %v", out)
	}
	out = a.flush(now.Add(3*time.Second), 2*time.Second)
	if len(out) != 1 || out[0].Records[0].Type != "USER_CMD" {
		t.Fatalf("flush: %+v", out)
	}
	// Events the log has left behind are closed without waiting.
	a.add(auditSSH, 0, now)
	a.add(auditOpen[0], 0, now)
	out = a.flush(now, 1*time.Second)
	if len(out) != 1 || out[0].Serial != "4260" || len(a.open) != 1 {
		t.Fatalf("newest: %+v, open %d", out, len(a.open))
	}
	if out = a.flushAll(); len(out) != 1 || out[0].Serial != "4270" || len(a.open) != 0 {
		t.Fatalf("flushAll: %+v", out)
	}
}

func auditEv(t *testing.T, lines ...string) *auditEvent {
	t.Helper()
	a := newAuditAssembler()
	for _, l := range lines {
		a.add(l, 0, time.Time{})
	}
	out := a.flushAll()
	if len(out) != 1 {
		t.Fatalf("%d events", len(out))
	}
	return out[0]
}

func TestAuditEntry(t *testing.T) {
	dir := t.TempDir()
	passwd := filepath.Join(dir, "passwd")
	if err := os.WriteFile(passwd, []byte("root:x:0:0::/root:/bin/sh\ncedric:x:1000:1000::/home/cedric:/bin/bash\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1759572100, 0)
	tests := []struct {
		name   string
		lines  []string
		host   string
		app    string
		sev    string
		msg    string
		fields map[string]string
	}{
		{"exec", auditExec, "srv", "SYSCALL", "info",
			`cedric (as root): exec apt install my "pkg" [exec] → success`,
			map[string]string{"audit_type": "SYSCALL", "audit_serial": "4242", "auid": "1000", "user": "cedric", "exe": "/usr/bin/apt", "res": "success", "key": "exec"}},
		{"sudo", []string{auditSudo}, "srv", "USER_CMD", "info",
			"cedric: sudo systemctl restart nginx (in /home/cedric) → success",
			map[string]string{"user": "cedric", "exe": "/usr/bin/sudo", "res": "success", "key": ""}},
		{"ssh failed", []string{auditSSH}, "srv", "USER_AUTH", "warning",
			"bob: PAM:authentication via /usr/sbin/sshd from 203.0.113.9 on ssh → failed",
			map[string]string{"auid": "4294967295", "user": "root", "res": "failed"}},
		{"enriched, node", auditOpen, "web1", "SYSCALL", "warning",
			"alice: syscall openat /etc/shadow by /usr/bin/cat [shadow] → failed",
			map[string]string{"user": "alice", "auid": "1001", "res": "failed", "key": "shadow"}},
		{"unknown uid", []string{`type=USER_LOGIN msg=audit(1759572004.000:4280): pid=1 uid=0 auid=4242 ses=9 msg='op=login id=4242 exe="/usr/sbin/sshd" hostname=? addr=10.0.0.5 terminal=/dev/pts/1 res=success'`},
			"srv", "USER_LOGIN", "info", "4242: login via /usr/sbin/sshd from 10.0.0.5 on /dev/pts/1 → success",
			map[string]string{"user": "4242"}},
		{"no result", []string{`type=CONFIG_CHANGE msg=audit(1759572005.000:4290): auid=1000 ses=3 op=add_rule key="watch" list=4`},
			"srv", "CONFIG_CHANGE", "info", "cedric: config_change add_rule [watch]",
			map[string]string{"res": ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := auditEv(t, tt.lines...)
			e := auditEntry(ev, &userNames{path: passwd}, "srv", now)
			if e.Host != tt.host || e.App != tt.app || e.Severity != tt.sev || e.SourceType != "audit" || !e.Wait {
				t.Errorf("entry %+v", e)
			}
			if e.Message != tt.msg {
				t.Errorf("message\n got %q\nwant %q", e.Message, tt.msg)
			}
			for k, v := range tt.fields {
				if e.Extra[k] != v {
					t.Errorf("%s = %q, want %q", k, e.Extra[k], v)
				}
			}
			if e.Extra["audit_raw"] != strings.Join(tt.lines, "\n") {
				t.Errorf("raw %q", e.Extra["audit_raw"])
			}
			if !e.Time.Equal(ev.Time) || e.Time.IsZero() {
				t.Errorf("time %v", e.Time)
			}
		})
	}
}

type entrySink struct {
	mu sync.Mutex
	es []*Entry
}

func (s *entrySink) add(e *Entry) {
	s.mu.Lock()
	s.es = append(s.es, e)
	s.mu.Unlock()
}

func (s *entrySink) serials() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, e := range s.es {
		out = append(out, e.Extra["audit_serial"])
	}
	return out
}

func appendLines(t *testing.T, path string, lines ...string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, l := range lines {
		if _, err := f.WriteString(l + "\n"); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAuditFollow(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")
	data := filepath.Join(dir, "data")
	if err := os.Mkdir(data, 0o755); err != nil {
		t.Fatal(err)
	}
	sink := &entrySink{}
	newAuditT := func() *Audit {
		a := NewAudit(logPath, dir, "srv", data, 100*365*24*time.Hour, sink.add)
		a.cfg.Enabled = true
		return a
	}
	a := newAuditT()
	now := time.Unix(1759572100, 0)

	// Missing file: reported, then read once it exists.
	a.poll(now)
	if st := a.Status(); st["code"] != "not_found" || st["open"] != false {
		t.Fatalf("status %v", st)
	}
	appendLines(t, logPath, auditExec[:3]...)
	appendLines(t, logPath, auditSudo)
	a.poll(now)
	if got := sink.serials(); len(got) != 0 { // events may still get records
		t.Fatalf("after first poll: %v", got)
	}
	// A partial line is not read.
	f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(auditSSH[:20])
	f.Close()
	a.poll(now.Add(5 * time.Second))
	if got := sink.serials(); len(got) != 2 || got[0] != "4242" || got[1] != "4250" {
		t.Fatalf("after second poll: %v", got)
	}
	if a.Status()["open"] != true || a.Status()["read"] != uint64(2) {
		t.Fatalf("status %v", a.Status())
	}
	a.saveState()

	// Restart: nothing is sent twice, the partial line is read once complete.
	a = newAuditT()
	f, _ = os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(auditSSH[20:] + "\n")
	f.Close()
	a.poll(now.Add(10 * time.Second))
	a.poll(now.Add(20 * time.Second))
	if got := sink.serials(); len(got) != 3 || got[2] != "4260" {
		t.Fatalf("after restart: %v", got)
	}

	// Rotation: lines added to the old file before the switch are not lost.
	appendLines(t, logPath, auditOpen[0])
	a.poll(now.Add(21 * time.Second)) // read, still open
	appendLines(t, logPath, auditOpen[1])
	if err := os.Rename(logPath, logPath+".1"); err != nil {
		t.Fatal(err)
	}
	appendLines(t, logPath, strings.Replace(auditSudo, "4250", "4300", 1))
	a.poll(now.Add(22 * time.Second))
	a.poll(now.Add(30 * time.Second))
	got := sink.serials()
	if len(got) != 5 || got[3] != "4270" || got[4] != "4300" {
		t.Fatalf("after rotation: %v", got)
	}
	sink.mu.Lock()
	if n := len(sink.es[3].Extra["audit_raw"]); n == 0 || !strings.Contains(sink.es[3].Extra["audit_raw"], "/etc/shadow") {
		t.Errorf("rotated event missing its PATH record: %q", sink.es[3].Extra["audit_raw"])
	}
	sink.mu.Unlock()
}

func TestAuditBackfillAndConfigure(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")
	appendLines(t, logPath, auditExec...)
	appendLines(t, logPath, auditSudo)
	sink := &entrySink{}
	// Only events newer than the backfill window are sent on the first read.
	a := NewAudit(logPath, dir, "", dir, time.Hour, sink.add)
	a.cfg.Enabled = true
	a.poll(time.Unix(1759572001+7200, 0))
	a.emit(a.asm.flushAll())
	if got := sink.serials(); len(got) != 0 {
		t.Fatalf("too old: %v", got)
	}
	a = NewAudit(logPath, dir, "", filepath.Join(dir, "other"), time.Hour, sink.add)
	if err := os.Mkdir(filepath.Join(dir, "other"), 0o755); err != nil {
		t.Fatal(err)
	}
	a.cfg.Enabled = true
	a.poll(time.Unix(1759572001+60, 0))
	a.emit(a.asm.flushAll())
	if got := sink.serials(); len(got) != 2 {
		t.Fatalf("backfill: %v", got)
	}
	if e := sink.es[0]; e.Host != "localhost" {
		t.Errorf("host %q", e.Host)
	}

	// Run picks the configuration up; turning it off resets the position.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	if err := a.Configure(auditConfig{Enabled: false}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(filepath.Join(dir, "other", "audit-state.json")); err == nil && strings.Contains(string(b), `"off": 0`) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
	if st := a.Status(); st["enabled"] != false || st["defaultPath"] != logPath {
		t.Fatalf("status %v", st)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "other", "audit-state.json"))
	if !strings.Contains(string(b), `"off": 0`) {
		t.Fatalf("state %s", b)
	}
}
