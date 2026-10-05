package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Formats of the "Files" source (Settings > Sources), one per declared file.
const (
	fmtText    = "text"    // one line per entry, level and leading timestamp detected
	fmtJSON    = "json"    // JSON lines: msg/time/level recognized, other keys as fields
	fmtLogfmt  = "logfmt"  // key=value pairs (Traefik "common" logs, Go programs…)
	fmtRegex   = "regex"   // regular expression with named groups
	fmtCLF     = "clf"     // access logs in Common/Combined Log Format (nginx, Apache, Traefik)
	fmtTraefik = "traefik" // Traefik access log in JSON
)

var fileFormats = map[string]bool{fmtText: true, fmtJSON: true, fmtLogfmt: true, fmtRegex: true, fmtCLF: true, fmtTraefik: true}

// fileRec is one parsed line (or several, for continuation lines).
type fileRec struct {
	t      time.Time
	dated  bool // t comes from the line
	sev    int
	msg    string
	host   string
	app    string
	fields map[string]string
}

const (
	maxFileFields   = 64
	maxFieldValue   = 4096
	maxFileMsg      = 64 << 10 // message with its continuation lines
	maxContinuation = 500      // continuation lines added to one entry
)

// fileParser turns a line of a file into an entry, for one format.
type fileParser struct {
	format string
	re     *regexp.Regexp // fmtRegex
}

func newFileParser(format, expr string) (*fileParser, error) {
	if !fileFormats[format] {
		return nil, fmt.Errorf("unknown format %q", format)
	}
	p := &fileParser{format: format}
	if format == fmtRegex {
		if strings.TrimSpace(expr) == "" {
			return nil, fmt.Errorf("the regex format needs a regular expression")
		}
		re, err := regexp.Compile(expr)
		if err != nil {
			return nil, fmt.Errorf("invalid regular expression: %v", err)
		}
		p.re = re
	}
	return p, nil
}

// multiline tells whether lines that do not start an entry are added to the
// previous one (stack traces…). Structured formats are one entry per line.
func (p *fileParser) multiline() bool {
	return p.format == fmtText || p.format == fmtRegex
}

// continuation reports a line that belongs to the previous entry: indented,
// or not matching the regular expression.
func (p *fileParser) continuation(line string) bool {
	if line != "" && (line[0] == ' ' || line[0] == '\t') {
		return true
	}
	return p.format == fmtRegex && !p.re.MatchString(line)
}

// parse decodes one line; it never fails (a line in another format is kept as text).
func (p *fileParser) parse(line string, now time.Time) *fileRec {
	line = cleanLine(line)
	switch p.format {
	case fmtJSON, fmtTraefik:
		if obj, ok := decodeJSONLine(line); ok {
			if p.format == fmtTraefik {
				if r := traefikRec(obj, line, now); r != nil {
					return r
				}
			}
			return jsonRec(obj, line, now)
		}
	case fmtLogfmt:
		if kv := parseLogfmt(line); len(kv) > 0 {
			return logfmtRec(kv, line, now)
		}
	case fmtRegex:
		if m := p.re.FindStringSubmatch(line); m != nil {
			return p.regexRec(m, line, now)
		}
	case fmtCLF:
		if r := clfRec(line, now); r != nil {
			return r
		}
	}
	return textRec(line, now)
}

func cleanLine(s string) string {
	s = strings.TrimRight(s, "\r")
	s = ansiRe.ReplaceAllString(s, "")
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "�")
	}
	return s
}

// textRec: the line itself, without its leading timestamp when there is one.
func textRec(line string, now time.Time) *fileRec {
	r := &fileRec{t: now, sev: 6, msg: line}
	if m := leadTimeRe.FindStringSubmatchIndex(line); m != nil {
		if t, ok := parseLogTime(line[m[2]:m[3]], now); ok {
			r.t, r.dated = t, true
			if rest := line[m[1]:]; strings.TrimSpace(rest) != "" {
				r.msg = rest
			}
		}
	}
	if n, ok := detectLevel(r.msg); ok {
		r.sev = n
	}
	return r
}

/* ---------- Timestamps ---------- */

// leadTimeRe: a timestamp at the start of a line, possibly in brackets
// (ISO 8601 / RFC 3339, 2006/01/02 15:04:05, CLF, syslog "Jan _2 15:04:05").
var leadTimeRe = regexp.MustCompile(`^\[?(\d{4}[-/]\d{2}[-/]\d{2}[T ]\d{2}:\d{2}:\d{2}(?:[.,]\d+)?(?: ?(?:Z|[+-]\d{2}:?\d{2}))?|\d{2}/[A-Z][a-z]{2}/\d{4}:\d{2}:\d{2}:\d{2} [+-]\d{4}|[A-Z][a-z]{2} +\d{1,2} \d{2}:\d{2}:\d{2})\]?[ \t]*(?:[-|:][ \t]+)?`)

var timeLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05Z0700",
	"2006-01-02T15:04:05",
	"02/Jan/2006:15:04:05 -0700",
	"Jan _2 15:04:05",
	time.RFC1123Z,
	time.RFC1123,
	time.UnixDate,
	time.ANSIC,
}

// parseLogTime reads the usual log timestamps; times without a zone are local.
func parseLogTime(s string, now time.Time) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if s[0] >= '0' && s[0] <= '9' && !strings.ContainsAny(s, ":/-") {
		if whole, _, _ := strings.Cut(s, "."); len(whole) < 9 {
			return time.Time{}, false // not a Unix time of this century
		}
		return epochTime(s)
	}
	// 2006/01/02 15:04:05,123 +02:00 -> 2006-01-02T15:04:05.123+02:00
	if len(s) >= 19 && s[4] == '/' && s[7] == '/' {
		s = s[:4] + "-" + s[5:7] + "-" + s[8:]
	}
	if len(s) >= 19 && s[4] == '-' && s[10] == ' ' {
		s = s[:10] + "T" + s[11:]
	}
	if len(s) > 19 && s[4] == '-' {
		s = s[:19] + strings.Replace(strings.Replace(s[19:], ",", ".", 1), " ", "", 1)
	}
	s = strings.Join(strings.Fields(s), " ") // "Jan  5" -> "Jan 5"
	for _, layout := range timeLayouts {
		if layout == "Jan _2 15:04:05" {
			layout = "Jan 2 15:04:05"
		}
		t, err := time.ParseInLocation(layout, s, time.Local)
		if err != nil {
			continue
		}
		if t.Year() == 0 { // syslog timestamps have no year
			t = t.AddDate(now.Year(), 0, 0)
			if t.After(now.Add(24 * time.Hour)) {
				t = t.AddDate(-1, 0, 0)
			}
		}
		return t, true
	}
	return time.Time{}, false
}

// epochTime reads a Unix time in seconds, milliseconds, microseconds or
// nanoseconds (guessed from its size), with an optional fraction.
func epochTime(s string) (time.Time, bool) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f <= 0 {
		return time.Time{}, false
	}
	switch {
	case f < 1e11:
		return time.Unix(0, int64(f*1e9)), true
	case f < 1e14:
		return time.UnixMilli(int64(f)), true
	case f < 1e17:
		return time.UnixMicro(int64(f)), true
	}
	return time.Unix(0, int64(f)), true
}

/* ---------- Fields ---------- */

// reservedFields are stored by LogStream itself: fields of the logs with the
// same name are renamed field_<name>.
var reservedFields = map[string]bool{
	"_time": true, "_msg": true, "_stream": true, "_stream_id": true, "received": true, "msg_time": true,
	"host": true, "host_name": true, "host_ip": true, "app": true, "facility": true, "severity": true,
	"sevnum": true, "source": true, "proto": true, "procid": true, "sd": true, "source_type": true, "log_file": true,
}

func fieldName(k string) string {
	k = strings.Map(func(r rune) rune {
		if r == '_' || r == '.' || r == '-' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			return r
		}
		return '_'
	}, k)
	if k == "" || reservedFields[k] || k[0] == '_' {
		k = "field_" + strings.TrimLeft(k, "_")
	}
	return k
}

func (r *fileRec) set(k, v string) {
	if v == "" {
		return
	}
	if r.fields == nil {
		r.fields = map[string]string{}
	}
	k = fieldName(k)
	if _, ok := r.fields[k]; !ok && len(r.fields) >= maxFileFields {
		return
	}
	if len(v) > maxFieldValue {
		v = strings.ToValidUTF8(v[:maxFieldValue], "") + "…"
	}
	r.fields[k] = v
}

/* ---------- JSON lines ---------- */

func decodeJSONLine(line string) (map[string]any, bool) {
	s := strings.TrimSpace(line)
	if len(s) < 2 || s[0] != '{' {
		return nil, false
	}
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var obj map[string]any
	if dec.Decode(&obj) != nil {
		return nil, false
	}
	return obj, true
}

var (
	jsonMsgKeys   = []string{"msg", "message", "@message", "log", "text"}
	jsonTimeKeys  = []string{"time", "ts", "timestamp", "@timestamp", "@t", "t", "datetime", "date"}
	jsonLevelKeys = []string{"level", "lvl", "severity", "log.level", "@level", "@l", "loglevel", "levelname"}
)

// takeString removes the first of keys found in obj and returns it as text.
func takeString(obj map[string]any, keys []string) (string, bool) {
	for _, k := range keys {
		if v, ok := obj[k]; ok {
			if s := scalarString(v); s != "" {
				delete(obj, k)
				return s, true
			}
		}
	}
	return "", false
}

func scalarString(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case bool:
		return strconv.FormatBool(v)
	}
	return ""
}

func jsonRec(obj map[string]any, line string, now time.Time) *fileRec {
	r := &fileRec{t: now, sev: 6, msg: line}
	for _, k := range jsonTimeKeys {
		if t, ok := parseLogTime(scalarString(obj[k]), now); ok {
			r.t, r.dated = t, true
			delete(obj, k)
			break
		}
	}
	lvl, hasLevel := takeString(obj, jsonLevelKeys)
	if hasLevel {
		if n, ok := levelNum(lvl); ok {
			r.sev = n
		} else if n, err := strconv.Atoi(lvl); err == nil {
			r.sev = numericLevel(n)
		} else {
			r.set("level", lvl)
		}
	}
	if s, ok := takeString(obj, jsonMsgKeys); ok {
		r.msg = strings.TrimRight(s, "\n")
	}
	flattenJSON("", obj, r)
	if !hasLevel {
		if n, ok := detectLevel(r.msg); ok {
			r.sev = n
		}
	}
	return r
}

// numericLevel maps numeric levels: syslog (0-7) or pino/bunyan (10-60).
func numericLevel(n int) int {
	switch {
	case n >= 0 && n <= 7:
		return n
	case n >= 60:
		return 2
	case n >= 50:
		return 3
	case n >= 40:
		return 4
	case n >= 30:
		return 6
	}
	return 7
}

// flattenJSON stores the remaining keys as fields; nested objects give
// dotted names, arrays stay in JSON.
func flattenJSON(prefix string, obj map[string]any, r *fileRec) {
	for k, v := range obj {
		name := prefix + k
		switch v := v.(type) {
		case map[string]any:
			flattenJSON(name+".", v, r)
		case []any:
			b, _ := json.Marshal(v)
			r.set(name, string(b))
		case nil:
		default:
			r.set(name, scalarString(v))
		}
	}
}

/* ---------- logfmt ---------- */

// parseLogfmt splits `k=v k2="v 2"` (with \" escapes); words without a value
// are skipped. Nil when the line has no pair.
func parseLogfmt(s string) [][2]string {
	var out [][2]string
	for i := 0; i < len(s); {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		start := i
		for i < len(s) && s[i] != '=' && s[i] != ' ' && s[i] != '\t' {
			i++
		}
		if i >= len(s) || s[i] != '=' || i == start {
			for i < len(s) && s[i] != ' ' && s[i] != '\t' {
				i++
			}
			continue
		}
		k := s[start:i]
		i++
		var v string
		if i < len(s) && s[i] == '"' {
			var b strings.Builder
			i++
			for i < len(s) && s[i] != '"' {
				if s[i] == '\\' && i+1 < len(s) {
					i++
					switch s[i] {
					case 'n':
						b.WriteByte('\n')
					case 't':
						b.WriteByte('\t')
					default:
						b.WriteByte(s[i])
					}
				} else {
					b.WriteByte(s[i])
				}
				i++
			}
			i++ // closing quote
			v = b.String()
		} else {
			end := i
			for end < len(s) && s[end] != ' ' && s[end] != '\t' {
				end++
			}
			v, i = s[i:end], end
		}
		out = append(out, [2]string{k, v})
	}
	return out
}

func logfmtRec(kv [][2]string, line string, now time.Time) *fileRec {
	obj := make(map[string]any, len(kv))
	for _, p := range kv {
		obj[p[0]] = p[1]
	}
	return jsonRec(obj, line, now)
}

/* ---------- Regular expression ---------- */

func (p *fileParser) regexRec(m []string, line string, now time.Time) *fileRec {
	r := &fileRec{t: now, sev: 6, msg: line}
	hasLevel := false
	for i, name := range p.re.SubexpNames() {
		if i == 0 || name == "" || m[i] == "" {
			continue
		}
		v := m[i]
		switch strings.ToLower(name) {
		case "time", "timestamp", "ts":
			if t, ok := parseLogTime(v, now); ok {
				r.t, r.dated = t, true
			}
		case "level", "severity", "lvl":
			if n, ok := levelNum(v); ok {
				r.sev, hasLevel = n, true
			}
		case "msg", "message":
			r.msg = v
		case "host":
			r.host = v
		case "app":
			r.app = v
		case "status":
			r.set(name, v)
			if n, ok := statusLevel(v); ok && !hasLevel {
				r.sev, hasLevel = n, true
			}
		default:
			r.set(name, v)
		}
	}
	if !hasLevel {
		if n, ok := detectLevel(r.msg); ok {
			r.sev = n
		}
	}
	return r
}

/* ---------- Access logs ---------- */

// statusLevel: 5xx are errors, 4xx warnings, the rest info.
func statusLevel(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 100 || n > 999 {
		return 0, false
	}
	switch {
	case n >= 500:
		return 3, true
	case n >= 400:
		return 4, true
	}
	return 6, true
}

// clfRe: Common Log Format, the combined extension (referer, user agent) and
// the Traefik one (request count, router, server URL, duration).
var clfRe = regexp.MustCompile(`^(\S+) (\S+) (\S+) \[([^\]]+)\] "((?:[^"\\]|\\.)*)" (\d{3}) (\d+|-)(?: "((?:[^"\\]|\\.)*)" "((?:[^"\\]|\\.)*)")?(?: (\d+) "([^"]*)" "([^"]*)" (\d+)ms)?`)

func clfRec(line string, now time.Time) *fileRec {
	m := clfRe.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	r := &fileRec{t: now, sev: 6, msg: line}
	if t, ok := parseLogTime(m[4], now); ok {
		r.t, r.dated = t, true
	}
	dash := func(s string) string {
		if s == "-" {
			return ""
		}
		return s
	}
	r.set("client_ip", m[1])
	r.set("http_user", dash(m[3]))
	if req := strings.Fields(m[5]); len(req) == 3 {
		r.set("http_method", req[0])
		r.set("http_path", req[1])
		r.set("http_proto", req[2])
	} else {
		r.set("http_request", m[5])
	}
	r.set("http_status", m[6])
	r.set("http_size", dash(m[7]))
	r.set("http_referer", dash(m[8]))
	r.set("user_agent", dash(m[9]))
	r.set("router", dash(m[11]))
	r.set("backend", dash(m[12]))
	r.set("duration_ms", m[13])
	r.sev, _ = statusLevel(m[6])
	return r
}

// traefikRec reads a Traefik access log line in JSON; nil for the other
// lines (Traefik's own logs), read as generic JSON.
func traefikRec(obj map[string]any, line string, now time.Time) *fileRec {
	status := scalarString(obj["DownstreamStatus"])
	if status == "" {
		status = scalarString(obj["OriginStatus"])
	}
	method := scalarString(obj["RequestMethod"])
	if status == "" && method == "" {
		return nil
	}
	r := &fileRec{t: now, sev: 6}
	if t, ok := parseLogTime(firstNonEmpty(scalarString(obj["StartUTC"]), scalarString(obj["time"])), now); ok {
		r.t, r.dated = t, true
	}
	if n, ok := statusLevel(status); ok {
		r.sev = n
	}
	rename := map[string]string{
		"ClientHost": "client_ip", "ClientUsername": "http_user", "RequestMethod": "http_method",
		"RequestHost": "http_host", "RequestPath": "http_path", "RequestProtocol": "http_proto",
		"DownstreamStatus": "http_status", "DownstreamContentSize": "http_size", "OriginStatus": "origin_status",
		"RouterName": "router", "ServiceName": "service", "ServiceURL": "backend", "entryPointName": "entrypoint",
		"request_User-Agent": "user_agent", "request_Referer": "http_referer",
	}
	dur := ""
	if d, err := strconv.ParseFloat(scalarString(obj["Duration"]), 64); err == nil {
		dur = strconv.FormatFloat(d/1e6, 'f', -1, 64)
		r.set("duration_ms", dur)
	}
	for _, k := range []string{"Duration", "StartUTC", "StartLocal", "time", "level", "msg"} {
		delete(obj, k)
	}
	for k, v := range obj {
		if to, ok := rename[k]; ok {
			r.set(to, scalarString(v))
			delete(obj, k)
		}
	}
	flattenJSON("", obj, r)
	// "GET example.com/path 404 · 12.5 ms · router@docker"
	var b strings.Builder
	b.WriteString(firstNonEmpty(method, "-"))
	b.WriteByte(' ')
	b.WriteString(r.fields["http_host"])
	b.WriteString(r.fields["http_path"])
	b.WriteByte(' ')
	b.WriteString(firstNonEmpty(status, "-"))
	if dur != "" {
		b.WriteString(" · " + dur + " ms")
	}
	if rt := r.fields["router"]; rt != "" {
		b.WriteString(" · " + rt)
	}
	if ip := r.fields["client_ip"]; ip != "" {
		b.WriteString(" · " + ip)
	}
	r.msg = b.String()
	return r
}
