package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Spool keeps on disk the batches VictoriaLogs could not take, so that they
// are sent later instead of being lost. Each batch is one NDJSON file named
// <unix nanoseconds>-<lines>.ndjson; files are sent back oldest first.
type Spool struct {
	dir string
	max int64 // maximum total size in bytes

	mu    sync.Mutex
	size  int64 // bytes on disk
	lines int64 // messages on disk
	seq   int64
}

// errSpoolFull is returned when a batch does not fit within SPOOL_MAX_MB.
var errSpoolFull = fmt.Errorf("disk buffer full")

// OpenSpool creates the directory if needed and counts the batches already
// there (left by a previous run).
func OpenSpool(dir string, max int64) (*Spool, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Spool{dir: dir, max: max}
	files, err := s.files()
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		s.size += f.size
		s.lines += f.lines
	}
	return s, nil
}

type spoolFile struct {
	path  string
	size  int64
	lines int64
}

// files lists the batches on disk, oldest first. Unfinished writes (.tmp)
// are removed.
func (s *Spool) files() ([]spoolFile, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var out []spoolFile
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".tmp") {
			_ = os.Remove(filepath.Join(s.dir, name))
			continue
		}
		base, ok := strings.CutSuffix(name, ".ndjson")
		if !ok {
			continue
		}
		_, n, _ := strings.Cut(base, "-")
		lines, _ := strconv.ParseInt(n, 10, 64)
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, spoolFile{path: filepath.Join(s.dir, name), size: info.Size(), lines: lines})
	}
	// The names start with a fixed-width timestamp: string order is time order.
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out, nil
}

// Write saves one batch of `lines` messages.
func (s *Spool) Write(body []byte, lines int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.size+int64(len(body)) > s.max {
		return errSpoolFull
	}
	s.seq++
	name := fmt.Sprintf("%020d%04d-%d.ndjson", time.Now().UnixNano(), s.seq%10000, lines)
	path := filepath.Join(s.dir, name)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	s.size += int64(len(body))
	s.lines += int64(lines)
	return nil
}

// Oldest returns the oldest batch, or ok=false when the spool is empty.
func (s *Spool) Oldest() (f spoolFile, body []byte, ok bool, err error) {
	files, err := s.files()
	if err != nil || len(files) == 0 {
		return f, nil, false, err
	}
	f = files[0]
	body, err = os.ReadFile(f.path)
	return f, body, err == nil, err
}

// Remove deletes a batch once VictoriaLogs has taken it.
func (s *Spool) Remove(f spoolFile) {
	if err := os.Remove(f.path); err != nil && !os.IsNotExist(err) {
		return
	}
	s.mu.Lock()
	s.size -= f.size
	s.lines -= f.lines
	s.mu.Unlock()
}

// Pending returns the number of messages and bytes waiting on disk.
func (s *Spool) Pending() (lines, size int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lines, s.size
}
