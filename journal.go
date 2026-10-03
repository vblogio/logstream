package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"time"
)

// Minimal reader of systemd journal files (*.journal), enough to follow them
// without journalctl: objects are walked in file order, which is the order
// entries were written. Format: https://systemd.io/JOURNAL_FILE_FORMAT/

const (
	jHeaderMin     = 208 // header fields used below end at offset 208
	jObjHeaderSize = 16

	jObjData  = 1
	jObjEntry = 3

	jIncompatCompact = 1 << 4
	jStateArchived   = 2
	jObjCompressed   = 1<<0 | 1<<1 | 1<<2 // XZ, LZ4, ZSTD: not decoded (no stdlib codec)
)

var jSignature = []byte("LPKSHHRH")

// jHeader holds the header fields the reader needs.
type jHeader struct {
	compact        bool
	archived       bool
	fileID         string
	headerSize     uint64
	tailObject     uint64 // offset of the last object
	tailSeqnum     uint64 // seqnum of the last complete entry
	tailRealtimeUS uint64 // realtime of the last entry (µs since the epoch)
}

func readJournalHeader(f io.ReaderAt) (jHeader, error) {
	b := make([]byte, jHeaderMin)
	if _, err := f.ReadAt(b, 0); err != nil {
		return jHeader{}, err
	}
	if !bytes.Equal(b[:8], jSignature) {
		return jHeader{}, errors.New("not a journal file")
	}
	le := binary.LittleEndian
	h := jHeader{
		compact:        le.Uint32(b[12:])&jIncompatCompact != 0,
		archived:       b[16] == jStateArchived,
		fileID:         hex.EncodeToString(b[24:40]),
		headerSize:     le.Uint64(b[88:]),
		tailObject:     le.Uint64(b[136:]),
		tailSeqnum:     le.Uint64(b[160:]),
		tailRealtimeUS: le.Uint64(b[192:]),
	}
	if h.headerSize < jHeaderMin {
		return h, errors.New("journal header too small")
	}
	return h, nil
}

// journalEntry is one entry: its time and its "FIELD=value" pairs.
type journalEntry struct {
	Realtime   time.Time
	Fields     map[string]string
	Compressed int // fields that could not be read (compressed data)
}

// readJournal reads the entries complete after offset `from` (0 = start of
// the file) and returns the offset to resume from. Entries older than
// `notBefore` are skipped without decoding their fields.
func readJournal(path string, from int64, notBefore time.Time, emit func(*journalEntry)) (next int64, h jHeader, err error) {
	f, err := os.Open(path)
	if err != nil {
		return from, h, err
	}
	defer f.Close()
	if h, err = readJournalHeader(f); err != nil {
		return from, h, err
	}
	if from < int64(h.headerSize) {
		from = int64(h.headerSize)
	}
	end := int64(h.tailObject)
	r := bufio.NewReaderSize(io.NewSectionReader(f, from, 1<<62), 64*1024)
	le := binary.LittleEndian
	hdr := make([]byte, jObjHeaderSize)
	off := from
	for off <= end {
		if _, err := io.ReadFull(r, hdr); err != nil {
			return off, h, nil // object still being written
		}
		typ, size := hdr[0], int64(le.Uint64(hdr[8:]))
		if size < jObjHeaderSize {
			return off, h, nil
		}
		body := size - jObjHeaderSize
		if typ == jObjEntry && body >= 48 && body < 1<<20 {
			obj := make([]byte, body)
			if _, err := io.ReadFull(r, obj); err != nil {
				return off, h, nil
			}
			seq := le.Uint64(obj[0:])
			if seq == 0 || seq > h.tailSeqnum {
				return off, h, nil // not linked yet: retry at the next poll
			}
			rt := time.UnixMicro(int64(le.Uint64(obj[8:])))
			if !rt.Before(notBefore) {
				emit(readEntryFields(f, h.compact, rt, obj[48:]))
			}
		} else if _, err := r.Discard(int(body)); err != nil {
			return off, h, nil
		}
		next := (off + size + 7) &^ 7 // objects are 8-byte aligned
		if pad := next - off - size; pad > 0 {
			if _, err := r.Discard(int(pad)); err != nil {
				return next, h, nil // the object is complete: resume after it
			}
		}
		off = next
	}
	return off, h, nil
}

// readEntryFields resolves the data objects referenced by an entry.
func readEntryFields(f io.ReaderAt, compact bool, rt time.Time, items []byte) *journalEntry {
	le := binary.LittleEndian
	e := &journalEntry{Realtime: rt, Fields: make(map[string]string, 16)}
	step := 16
	if compact {
		step = 4
	}
	payloadAt := int64(64)
	if compact {
		payloadAt = 72
	}
	hdr := make([]byte, jObjHeaderSize)
	for i := 0; i+step <= len(items); i += step {
		var off int64
		if compact {
			off = int64(le.Uint32(items[i:]))
		} else {
			off = int64(le.Uint64(items[i:]))
		}
		if off == 0 {
			continue
		}
		if _, err := f.ReadAt(hdr, off); err != nil || hdr[0] != jObjData {
			continue
		}
		if hdr[1]&jObjCompressed != 0 {
			e.Compressed++
			continue
		}
		size := int64(le.Uint64(hdr[8:]))
		n := size - payloadAt
		if n <= 0 || n > 1<<20 {
			continue
		}
		p := make([]byte, n)
		if _, err := f.ReadAt(p, off+payloadAt); err != nil {
			continue
		}
		if k := bytes.IndexByte(p, '='); k > 0 {
			e.Fields[string(p[:k])] = string(p[k+1:])
		}
	}
	return e
}
