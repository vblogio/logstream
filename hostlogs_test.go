package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeJournal builds a minimal journal file: header, data objects, entries.
type fakeJournal struct {
	compact bool
	buf     []byte
	seq     uint64
	tail    uint64
}

func newFakeJournal(compact bool) *fakeJournal {
	j := &fakeJournal{compact: compact, buf: make([]byte, 272)}
	copy(j.buf, jSignature)
	if compact {
		binary.LittleEndian.PutUint32(j.buf[12:], jIncompatCompact)
	}
	j.buf[16] = 1 // online
	copy(j.buf[24:40], "0123456789abcdef")
	binary.LittleEndian.PutUint64(j.buf[88:], 272)
	return j
}

func (j *fakeJournal) object(typ, flags byte, body []byte) uint64 {
	for len(j.buf)%8 != 0 {
		j.buf = append(j.buf, 0)
	}
	off := uint64(len(j.buf))
	h := make([]byte, jObjHeaderSize)
	h[0], h[1] = typ, flags
	binary.LittleEndian.PutUint64(h[8:], uint64(jObjHeaderSize+len(body)))
	j.buf = append(append(j.buf, h...), body...)
	j.tail = off
	binary.LittleEndian.PutUint64(j.buf[136:], off)
	return off
}

func (j *fakeJournal) data(field string, flags byte) uint64 {
	n := 48
	if j.compact {
		n = 56
	}
	return j.object(jObjData, flags, append(make([]byte, n), field...))
}

// entry appends an entry; linked=false leaves it unfinished (tail seqnum not updated).
func (j *fakeJournal) entry(t time.Time, linked bool, fields ...string) {
	var items []byte
	for _, f := range fields {
		flags := byte(0)
		if strings.HasPrefix(f, "!") { // compressed data object
			f, flags = f[1:], 4
		}
		off := j.data(f, flags)
		if j.compact {
			items = binary.LittleEndian.AppendUint32(items, uint32(off))
		} else {
			items = binary.LittleEndian.AppendUint64(items, off)
			items = binary.LittleEndian.AppendUint64(items, 0)
		}
	}
	j.seq++
	body := make([]byte, 48)
	binary.LittleEndian.PutUint64(body[0:], j.seq)
	binary.LittleEndian.PutUint64(body[8:], uint64(t.UnixMicro()))
	j.object(jObjEntry, 0, append(body, items...))
	if linked {
		binary.LittleEndian.PutUint64(j.buf[160:], j.seq)
		binary.LittleEndian.PutUint64(j.buf[192:], uint64(t.UnixMicro()))
	}
}

func (j *fakeJournal) write(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, j.buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadJournal(t *testing.T) {
	for _, compact := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "system.journal")
		now := time.Now()
		j := newFakeJournal(compact)
		j.entry(now.Add(-2*time.Hour), true, "MESSAGE=too old", "PRIORITY=6")
		j.entry(now.Add(-time.Minute), true, "MESSAGE=Started cron.", "PRIORITY=5", "SYSLOG_IDENTIFIER=systemd", "_HOSTNAME=srv1", "_PID=1")
		j.write(t, path)

		var got []*journalEntry
		emit := func(e *journalEntry) { got = append(got, e) }
		next, _, err := readJournal(path, 0, now.Add(-time.Hour), emit)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Fields["MESSAGE"] != "Started cron." || got[0].Fields["_HOSTNAME"] != "srv1" {
			t.Fatalf("compact=%v: got %+v", compact, got)
		}

		// A new complete entry and one still being written.
		j.entry(now, true, "MESSAGE=second", "!MESSAGE=big")
		j.entry(now, false, "MESSAGE=unfinished")
		j.write(t, path)
		got = nil
		next2, _, err := readJournal(path, next, time.Time{}, emit)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Fields["MESSAGE"] != "second" || got[0].Compressed != 1 {
			t.Fatalf("compact=%v: resumed read got %d entries", compact, len(got))
		}

		// Once linked, the unfinished entry is read from where reading stopped.
		binary.LittleEndian.PutUint64(j.buf[160:], j.seq)
		j.write(t, path)
		got = nil
		if _, _, err := readJournal(path, next2, time.Time{}, emit); err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Fields["MESSAGE"] != "unfinished" {
			t.Fatalf("compact=%v: unfinished entry got %+v", compact, got)
		}
	}
}

func TestHostLogsJournal(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "var/log/journal/machine")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	j := newFakeJournal(true)
	j.entry(time.Now(), true, "MESSAGE=Accepted publickey", "PRIORITY=6", "SYSLOG_FACILITY=10", "SYSLOG_IDENTIFIER=sshd", "_PID=42", "_HOSTNAME=srv1", "_SYSTEMD_UNIT=ssh.service")
	j.entry(time.Now(), true, "MESSAGE=oops", "PRIORITY=3", "_TRANSPORT=kernel", "_HOSTNAME=srv1")
	j.write(t, filepath.Join(dir, "system.journal"))

	var got []*Entry
	h := NewHostLogs(root, t.TempDir(), time.Hour, func(e *Entry) { got = append(got, e) })
	h.scan(time.Now())
	if len(got) != 2 {
		t.Fatalf("got %d entries", len(got))
	}
	e := got[0]
	if e.App != "sshd" || e.ProcID != "42" || e.Facility != "authpriv" || e.Severity != "info" || e.Host != "srv1" || e.SourceType != "host" || e.Extra["unit"] != "ssh.service" {
		t.Fatalf("entry %+v", e)
	}
	if got[1].Facility != "kern" || got[1].Severity != "err" {
		t.Fatalf("kernel entry %+v", got[1])
	}
	h.scan(time.Now()) // nothing new
	if len(got) != 2 {
		t.Fatalf("re-read: %d entries", len(got))
	}
	if st := h.Status(); st["mode"] != "journal" || st["files"] != 1 {
		t.Fatalf("status %+v", st)
	}
}

func TestHostLogsFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "var/log")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	auth := filepath.Join(dir, "auth.log")
	if err := os.WriteFile(auth, []byte("Oct  3 07:00:00 srv1 sshd[1]: old line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "auth.log.1"), []byte("rotated\n"), 0o644)

	var got []*Entry
	h := NewHostLogs(root, t.TempDir(), time.Hour, func(e *Entry) { got = append(got, e) })
	h.scan(time.Now()) // existing content is skipped
	if len(got) != 0 {
		t.Fatalf("first scan read %d lines", len(got))
	}
	f, _ := os.OpenFile(auth, os.O_APPEND|os.O_WRONLY, 0)
	_, _ = f.WriteString("Oct  3 08:00:00 srv1 sshd[7]: Failed password for root\nOct  3 08:00:01 srv1 sshd[7]: partial")
	f.Close()
	h.scan(time.Now())
	if len(got) != 1 {
		t.Fatalf("got %d lines", len(got))
	}
	e := got[0]
	if e.Host != "srv1" || e.App != "sshd" || e.ProcID != "7" || e.Facility != "auth" || e.Extra["log_file"] != "/var/log/auth.log" || e.Proto != "file" {
		t.Fatalf("entry %+v", e)
	}
	if st := h.Status(); st["mode"] != "files" || st["files"] != 1 {
		t.Fatalf("status %+v", st)
	}
}

func TestHostLogsNotMounted(t *testing.T) {
	h := NewHostLogs(filepath.Join(t.TempDir(), "none"), t.TempDir(), time.Hour, func(*Entry) {})
	h.scan(time.Now())
	if st := h.Status(); st["code"] != "not_mounted" {
		t.Fatalf("status %+v", st)
	}
}
