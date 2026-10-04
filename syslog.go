package main

import (
	"bufio"
	"context"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var severityNames = [8]string{"emerg", "alert", "crit", "err", "warning", "notice", "info", "debug"}

var facilityNames = [24]string{
	"kern", "user", "mail", "daemon", "auth", "syslog", "lpr", "news",
	"uucp", "cron", "authpriv", "ftp", "ntp", "security", "console", "solaris-cron",
	"local0", "local1", "local2", "local3", "local4", "local5", "local6", "local7",
}

// Entry is a decoded syslog message.
type Entry struct {
	Time     time.Time // timestamp from the message header (or reception time if absent)
	Received time.Time // reception time, from the server clock
	Host     string
	App      string
	ProcID   string
	Facility string
	Severity string
	SevNum   int
	SD       string // raw RFC 5424 structured data
	Message  string
	Source   string // sender IP address
	Proto    string // udp or tcp
	HostIP   string // original host value when it was an IP resolved through DNS

	SourceType string            // "syslog", "docker", "host", "kernel", "audit" or "logstream"
	Extra      map[string]string // additional fields (docker: container, image, …)

	// Not stored. Wait: the producer can be slowed down when the queue is full
	// (Docker, host logs) instead of losing the message. Done: called once the
	// message is stored in VictoriaLogs or in the disk spool.
	Wait bool
	Done func()
}

// Record returns the entry in the shape stored in VictoriaLogs and returned by the API.
// The time axis (_time) is the reception time: device clocks can be wrong, and
// logs indexed in the past would escape time ranges. The device timestamp is
// kept in msg_time.
func (e *Entry) Record() map[string]string {
	rcv := e.Received.UTC().Format(time.RFC3339Nano)
	r := map[string]string{
		"_time":    rcv,
		"received": rcv,
		"msg_time": e.Time.UTC().Format(time.RFC3339Nano),
		"_msg":     e.Message,
		"host":     e.Host,
		"app":      e.App,
		"facility": e.Facility,
		"severity": e.Severity,
		"sevnum":   strconv.Itoa(e.SevNum),
		"source":   e.Source,
		"proto":    e.Proto,
	}
	if e.ProcID != "" {
		r["procid"] = e.ProcID
	}
	if e.SD != "" {
		r["sd"] = e.SD
	}
	if e.HostIP != "" {
		r["host_ip"] = e.HostIP
	}
	if e.SourceType != "" {
		r["source_type"] = e.SourceType
	}
	for k, v := range e.Extra {
		if v != "" {
			r[k] = v
		}
	}
	return r
}

func (e *Entry) setPri(pri int) {
	e.SevNum = pri % 8
	e.Severity = severityNames[pri%8]
	e.Facility = facilityNames[pri/8]
}

// ParseSyslog decodes an RFC 5424 or RFC 3164 (BSD) message. It never fails:
// a non-conforming message is kept as is.
func ParseSyslog(raw []byte, source, proto string, now time.Time) *Entry {
	s := strings.TrimRight(string(raw), "\r\n\x00 ")
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "\uFFFD")
	}
	e := &Entry{Time: now, Received: now, Host: source, Source: source, Proto: proto, SourceType: "syslog"}
	e.setPri(13) // user.notice, the RFC 3164 default

	if len(s) > 2 && s[0] == '<' {
		if end := strings.IndexByte(s, '>'); end > 1 && end <= 4 {
			if pri, err := strconv.Atoi(s[1:end]); err == nil && pri >= 0 && pri <= 191 {
				e.setPri(pri)
				s = s[end+1:]
			}
		}
	}
	if strings.HasPrefix(s, "1 ") {
		parse5424(e, s[2:])
	} else {
		parse3164(e, s)
	}
	if e.Message == "" {
		e.Message = "(empty)"
	}
	return e
}

// parse5424: TIMESTAMP HOSTNAME APP-NAME PROCID MSGID STRUCTURED-DATA MSG
func parse5424(e *Entry, s string) {
	f := strings.SplitN(s, " ", 6)
	if len(f) < 6 {
		e.Message = s
		return
	}
	if t, err := time.Parse(time.RFC3339Nano, f[0]); err == nil {
		e.Time = t
	}
	if f[1] != "-" {
		e.Host = f[1]
	}
	if f[2] != "-" {
		e.App = f[2]
	}
	if f[3] != "-" {
		e.ProcID = f[3]
	}
	rest := f[5]
	switch {
	case rest == "-" || strings.HasPrefix(rest, "- "):
		rest = strings.TrimPrefix(rest[1:], " ")
	case strings.HasPrefix(rest, "["):
		n := sdLen(rest)
		e.SD = rest[:n]
		rest = strings.TrimPrefix(rest[n:], " ")
	}
	e.Message = strings.TrimPrefix(rest, "\uFEFF") // optional UTF-8 BOM
}

// sdLen returns the length of the structured data block "[..][..]".
func sdLen(s string) int {
	i := 0
	for i < len(s) && s[i] == '[' {
		inQuote := false
		for i++; i < len(s); i++ {
			c := s[i]
			if inQuote && c == '\\' {
				i++
				continue
			}
			if c == '"' {
				inQuote = !inQuote
				continue
			}
			if c == ']' && !inQuote {
				i++
				break
			}
		}
	}
	if i > len(s) {
		i = len(s)
	}
	return i
}

// parse3164: "Mmm dd hh:mm:ss HOST TAG[PID]: MSG", or an ISO timestamp (rsyslog).
func parse3164(e *Entry, s string) {
	if len(s) >= 16 && s[15] == ' ' {
		if t, err := time.ParseInLocation(time.Stamp, s[:15], time.Local); err == nil {
			// No year in the timestamp: use the current one, unless that lands in the future.
			now := e.Time
			t = time.Date(now.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local)
			if t.After(now.Add(24 * time.Hour)) {
				t = t.AddDate(-1, 0, 0)
			}
			e.Time = t
			parseHostTag(e, s[16:], true)
			return
		}
	}
	if sp := strings.IndexByte(s, ' '); sp >= 19 {
		if t, err := time.Parse(time.RFC3339Nano, s[:sp]); err == nil {
			e.Time = t
			parseHostTag(e, s[sp+1:], true)
			return
		}
	}
	parseHostTag(e, s, false)
}

func parseHostTag(e *Entry, s string, withHost bool) {
	if withHost {
		if sp := strings.IndexByte(s, ' '); sp > 0 {
			tok := s[:sp]
			if !strings.HasSuffix(tok, ":") && !strings.Contains(tok, "[") {
				e.Host = tok
				s = s[sp+1:]
			}
		}
	}
	if i := strings.IndexAny(s, ":[ "); i > 0 && i <= 48 {
		switch s[i] {
		case ':':
			if i+1 == len(s) || s[i+1] == ' ' {
				e.App = s[:i]
				s = strings.TrimPrefix(s[i+1:], " ")
			}
		case '[':
			if j := strings.IndexByte(s[i:], ']'); j > 1 {
				e.App = s[:i]
				e.ProcID = s[i+1 : i+j]
				s = strings.TrimPrefix(strings.TrimPrefix(s[i+j+1:], ":"), " ")
			}
		}
	}
	e.Message = s
}

func serveUDP(ctx context.Context, pc net.PacketConn, sink func(*Entry)) {
	buf := make([]byte, 65536)
	for {
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logEvent("syslog", 4, "UDP read error: %v", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		sink(ParseSyslog(buf[:n], hostOf(addr), "udp", time.Now()))
	}
}

// TCP limits: connections open at once, and how long a connection may stay
// silent before it is closed (senders reconnect on their own).
var (
	tcpMaxConns = 512
	tcpIdle     = 30 * time.Minute
)

func serveTCP(ctx context.Context, ln net.Listener, sink func(*Entry)) {
	slots := make(chan struct{}, tcpMaxConns)
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logEvent("syslog", 4, "TCP accept error: %v", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		select {
		case slots <- struct{}{}:
		default:
			logEvent("syslog", 4, "%d TCP connections already open, refusing %s", tcpMaxConns, conn.RemoteAddr())
			conn.Close()
			continue
		}
		go func() {
			defer func() { <-slots }()
			handleTCP(ctx, conn, sink)
		}()
	}
}

// idleConn pushes the read deadline back before each read, so only a
// connection that stays silent for tcpIdle is closed.
type idleConn struct {
	net.Conn
	idle time.Duration
}

func (c idleConn) Read(p []byte) (int, error) {
	_ = c.Conn.SetReadDeadline(time.Now().Add(c.idle))
	return c.Conn.Read(p)
}

const maxFrame = 1 << 20

// handleTCP supports both RFC 6587 framings: octet counting
// ("123 <34>1 ...") and one message per line.
func handleTCP(ctx context.Context, conn net.Conn, sink func(*Entry)) {
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	src := hostOf(conn.RemoteAddr())
	r := bufio.NewReaderSize(idleConn{conn, tcpIdle}, 64*1024)
	for {
		c, err := r.ReadByte()
		if err != nil {
			return
		}
		switch {
		case c == '\n' || c == '\r' || c == ' ' || c == 0:
			continue
		case c >= '1' && c <= '9':
			n := int(c - '0')
			for {
				c, err = r.ReadByte()
				if err != nil {
					return
				}
				if c == ' ' {
					break
				}
				if c < '0' || c > '9' || n > maxFrame {
					log.Printf("syslog tcp %s: invalid frame, closing connection", src)
					return
				}
				n = n*10 + int(c-'0')
			}
			if n > maxFrame {
				return
			}
			msg := make([]byte, n)
			if _, err := io.ReadFull(r, msg); err != nil {
				return
			}
			sink(ParseSyslog(msg, src, "tcp", time.Now()))
		default:
			_ = r.UnreadByte()
			line, err := r.ReadSlice('\n')
			if len(line) > 0 {
				sink(ParseSyslog(line, src, "tcp", time.Now()))
			}
			// Line longer than the buffer: skip the rest of it.
			for err == bufio.ErrBufferFull {
				_, err = r.ReadSlice('\n')
			}
			if err != nil {
				return
			}
		}
	}
}

func hostOf(a net.Addr) string {
	if a == nil {
		return ""
	}
	h, _, err := net.SplitHostPort(a.String())
	if err != nil {
		return a.String()
	}
	return h
}
