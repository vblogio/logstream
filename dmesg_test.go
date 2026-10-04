package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseKmsg(t *testing.T) {
	rec, ok := parseKmsg([]byte("6,339,5140900,-;NET: Registered protocol family 10\n SUBSYSTEM=net\n DEVICE=n2"))
	if !ok || rec.Prio != 6 || rec.Seq != 339 || rec.Usec != 5140900 || rec.Msg != "NET: Registered protocol family 10" {
		t.Fatalf("record %+v", rec)
	}
	if rec.Dict["SUBSYSTEM"] != "net" || rec.Dict["DEVICE"] != "n2" {
		t.Fatalf("dict %+v", rec.Dict)
	}
	// Extra header fields (newer kernels: caller id) and a ';' in the message.
	rec, ok = parseKmsg([]byte("4,12,99,c,caller=T1;a;b \\x1b[31mred\\x5c"))
	if !ok || rec.Seq != 12 || rec.Msg != "a;b \x1b[31mred\\" {
		t.Fatalf("record %+v", rec)
	}
	for _, bad := range []string{"", "no header", "6,1;short", "x,1,2,-;msg", "6,-1,2,-;msg", "-1,1,2,-;msg"} {
		if _, ok := parseKmsg([]byte(bad)); ok {
			t.Errorf("%q parsed", bad)
		}
	}
}

func TestUnescapeKmsg(t *testing.T) {
	for in, want := range map[string]string{
		`plain`:       "plain",
		`a\x0ab`:      "a\nb",
		`end\x4`:      `end\x4`,
		`bad\xzz`:     `bad\xzz`,
		`\x41\x42\x4`: `AB\x4`,
	} {
		if got := unescapeKmsg(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

func TestSplitKmsg(t *testing.T) {
	got := splitKmsg([]byte("6,1,0,-;one\n SUBSYSTEM=pci\n6,2,0,-;two\n6,3,0,-;three"))
	if len(got) != 3 || string(got[0]) != "6,1,0,-;one\n SUBSYSTEM=pci" || string(got[2]) != "6,3,0,-;three" {
		t.Fatalf("got %q", got)
	}
}

func TestKmsgEntry(t *testing.T) {
	boot := time.Date(2026, 10, 4, 6, 0, 0, 0, time.UTC)
	now := boot.Add(time.Hour)
	rec, _ := parseKmsg([]byte("3,7,2500000,-;usb 1-1: device descriptor read/64, error -71\n SUBSYSTEM=usb\n DEVICE=c189:1"))
	e := kmsgEntry(rec, boot.Add(2500*time.Millisecond), now)
	if e.App != "kernel" || e.Facility != "kern" || e.Severity != "err" || e.SevNum != 3 || e.SourceType != "kernel" || e.Proto != "kmsg" || !e.Wait {
		t.Fatalf("entry %+v", e)
	}
	if e.Message != "usb 1-1: device descriptor read/64, error -71" || e.Extra["subsystem"] != "usb" || e.Extra["device"] != "c189:1" {
		t.Fatalf("entry %+v", e)
	}
	if !e.Time.Equal(boot.Add(2500*time.Millisecond)) || !e.Received.Equal(now) {
		t.Fatalf("times %v %v", e.Time, e.Received)
	}
	// Written by a program (facility daemon = 3<<3 | info).
	rec, _ = parseKmsg([]byte("30,8,3000000,-;systemd[1]: Started Journal Service."))
	e = kmsgEntry(rec, now, now)
	if e.App != "systemd" || e.ProcID != "1" || e.Facility != "daemon" || e.Severity != "info" || e.Message != "Started Journal Service." || e.Extra != nil {
		t.Fatalf("entry %+v", e)
	}
	// Facility out of range.
	rec, _ = parseKmsg([]byte("255,9,0,-;odd"))
	if e = kmsgEntry(rec, now, now); e.Facility != "user" || e.Severity != "debug" {
		t.Fatalf("entry %+v", e)
	}
}

// fakeHost writes a kmsg file, a boot id and an uptime in a temp dir.
func fakeHost(t *testing.T, uptime float64, records ...string) (*Dmesg, *[]*Entry, *sync.Mutex) {
	t.Helper()
	dir := t.TempDir()
	write := func(name, s string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	var mu sync.Mutex
	var got []*Entry
	d := NewDmesg(write("kmsg", strings.Join(records, "\n")+"\n"), filepath.Join(dir, "data"), time.Hour, func(e *Entry) {
		mu.Lock()
		got = append(got, e)
		mu.Unlock()
	})
	d.bootIDPath = write("boot_id", "b1\n")
	d.uptimePath = write("uptime", strconv.FormatFloat(uptime, 'f', 2, 64)+" 1234.00\n")
	d.retry = 20 * time.Millisecond
	return d, &got, &mu
}

func TestDmesgBackfillAndResume(t *testing.T) {
	// Booted 3 h ago: the record at 1 s is outside the 1 h backfill window,
	// the ones at 2.5 h and 2.9 h are inside.
	d, got, mu := fakeHost(t, 3*3600,
		"6,1,1000000,-;Linux version 6.12",
		"4,2,9000000000,-;eth0: link down",
		"6,3,10440000000,-;eth0: link up",
	)
	s, err := d.start()
	if err != nil {
		t.Fatal(err)
	}
	<-s.done
	mu.Lock()
	if len(*got) != 2 || (*got)[0].Message != "eth0: link down" || (*got)[1].Message != "eth0: link up" {
		t.Fatalf("got %+v", *got)
	}
	if age := time.Since((*got)[1].Time); age < 5*time.Minute || age > 7*time.Minute {
		t.Fatalf("record dated %v ago", age)
	}
	mu.Unlock()
	d.saveState()

	// Restart with the saved position: same boot, nothing is sent again,
	// and a new record is.
	f, _ := os.OpenFile(d.device, os.O_APPEND|os.O_WRONLY, 0)
	_, _ = f.WriteString("6,6,10500000000,-;eth0: renamed\n")
	f.Close()
	d2 := NewDmesg(d.device, filepath.Dir(d.statePath), time.Hour, d.sink)
	d2.bootIDPath, d2.uptimePath = d.bootIDPath, d.uptimePath
	if d2.pos != (dmesgPos{BootID: "b1", Seq: 3}) {
		t.Fatalf("saved position %+v", d2.pos)
	}
	s, err = d2.start()
	if err != nil {
		t.Fatal(err)
	}
	<-s.done
	mu.Lock()
	defer mu.Unlock()
	if len(*got) != 3 || (*got)[2].Message != "eth0: renamed" || (*got)[2].App != "kernel" {
		t.Fatalf("after restart %+v", (*got)[2:])
	}
	if st := d2.Status(); st["lost"] != uint64(2) || st["read"] != uint64(1) {
		t.Fatalf("status %+v", st) // records 4 and 5 were overwritten
	}
}

func TestDmesgNewBoot(t *testing.T) {
	d, got, mu := fakeHost(t, 60, "6,1,1000000,-;booted")
	d.pos = dmesgPos{BootID: "old", Seq: 500}
	s, err := d.start()
	if err != nil {
		t.Fatal(err)
	}
	<-s.done
	mu.Lock()
	defer mu.Unlock()
	if len(*got) != 1 || d.pos != (dmesgPos{BootID: "b1", Seq: 1}) {
		t.Fatalf("got %d entries, position %+v", len(*got), d.pos)
	}
}

func TestDmesgNoBackfill(t *testing.T) {
	d, got, mu := fakeHost(t, 60, "6,1,1000000,-;old")
	d.backfill = 0
	s, err := d.start()
	if err != nil {
		t.Fatal(err)
	}
	<-s.done
	mu.Lock()
	defer mu.Unlock()
	if len(*got) != 0 {
		t.Fatalf("got %+v", *got)
	}
}

func TestDmesgRun(t *testing.T) {
	d, got, mu := fakeHost(t, 60, "6,1,1000000,-;first", "6,2,2000000,-;second")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		d.Run(ctx)
		close(done)
	}()
	if err := d.Configure(dmesgConfig{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		n := len(*got)
		mu.Unlock()
		if n >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("got %d entries", n)
		}
		time.Sleep(10 * time.Millisecond)
	}
	// The file is re-opened after each end of file: still no duplicate.
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done
	mu.Lock()
	defer mu.Unlock()
	if len(*got) != 2 {
		t.Fatalf("got %d entries", len(*got))
	}
	b, err := os.ReadFile(d.statePath)
	if err != nil || !strings.Contains(string(b), `"seq": 2`) {
		t.Fatalf("state %s %v", b, err)
	}
	b, _ = os.ReadFile(d.cfgPath)
	if !strings.Contains(string(b), `"enabled": true`) {
		t.Fatalf("config %s", b)
	}
}

func TestDmesgMissingDevice(t *testing.T) {
	d := NewDmesg(filepath.Join(t.TempDir(), "kmsg"), t.TempDir(), time.Hour, func(*Entry) {})
	d.retry = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		d.Run(ctx)
		close(done)
	}()
	_ = d.Configure(dmesgConfig{Enabled: true})
	deadline := time.Now().Add(5 * time.Second)
	for d.Status()["code"] != "not_found" {
		if time.Now().After(deadline) {
			t.Fatalf("status %+v", d.Status())
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Turning it off clears the error.
	_ = d.Configure(dmesgConfig{Enabled: false})
	if st := d.Status(); st["code"] != "" || st["enabled"] != false {
		t.Fatalf("status %+v", st)
	}
	cancel()
	<-done
}

func TestDmesgPermission(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads everything")
	}
	p := filepath.Join(t.TempDir(), "kmsg")
	_ = os.WriteFile(p, []byte("6,1,0,-;x\n"), 0o000)
	d := NewDmesg(p, t.TempDir(), time.Hour, func(*Entry) {})
	_, err := d.start()
	d.fail(err)
	if st := d.Status(); st["code"] != "permission" {
		t.Fatalf("status %+v", st)
	}
}
