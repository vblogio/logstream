package main

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Filter holds the search criteria sent by the UI.
type Filter struct {
	Mode     string // "simple" or "logsql"
	Text     string
	Range    string    // 5m, 15m, 1h, 6h, 24h, 7d, 30d; anything else = no time limit
	From, To time.Time // absolute range (timeline zoom), replaces Range when From is set
	Host     string
	App      string
	Severity int    // highest severity number included (0 = emerg … 7 = debug), -1 = all
	Source   string // "syslog", "docker", "host" or "" for all
}

var rangeDurations = map[string]time.Duration{
	"5m": 5 * time.Minute, "15m": 15 * time.Minute, "1h": time.Hour, "6h": 6 * time.Hour,
	"24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour, "30d": 30 * 24 * time.Hour,
}

// msParam reads a Unix time in milliseconds (zero time when absent or invalid).
func msParam(v string) time.Time {
	ms, err := strconv.ParseInt(v, 10, 64)
	if err != nil || ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

func FilterFromRequest(r *http.Request) Filter {
	q := r.URL.Query()
	f := Filter{
		Mode:     q.Get("mode"),
		Text:     strings.TrimSpace(q.Get("q")),
		Range:    q.Get("range"),
		Host:     q.Get("host"),
		App:      q.Get("app"),
		Source:   q.Get("source"),
		Severity: -1,
	}
	if v, err := strconv.Atoi(q.Get("severity")); err == nil && v >= 0 && v <= 7 {
		f.Severity = v
	}
	// from/to: Unix milliseconds, sent when the timeline is zoomed.
	if from := msParam(q.Get("from")); !from.IsZero() {
		f.From, f.To = from, msParam(q.Get("to"))
		if !f.To.IsZero() && !f.To.After(f.From) {
			f.To = time.Time{}
		}
	}
	return f
}

type term struct {
	text string
	neg  bool
}

// parseTerms splits a simple search into words, "quoted phrases" and -exclusions.
func parseTerms(s string) []term {
	var out []term
	for i := 0; i < len(s); {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		if i >= len(s) {
			break
		}
		neg := false
		if s[i] == '-' && i+1 < len(s) && s[i+1] != ' ' {
			neg = true
			i++
		}
		var t string
		if s[i] == '"' {
			end := strings.IndexByte(s[i+1:], '"')
			if end < 0 {
				t, i = s[i+1:], len(s)
			} else {
				t, i = s[i+1:i+1+end], i+end+2
			}
		} else {
			end := strings.IndexAny(s[i:], " \t")
			if end < 0 {
				t, i = s[i:], len(s)
			} else {
				t, i = s[i:i+end], i+end
			}
		}
		if t != "" {
			out = append(out, term{t, neg})
		}
	}
	return out
}

// filterExpr turns the criteria (except raw LogsQL text) into a LogsQL filter.
func (f Filter) filterExpr() string {
	var parts []string
	switch {
	case !f.From.IsZero() && !f.To.IsZero():
		parts = append(parts, "_time:["+logsqlTime(f.From)+", "+logsqlTime(f.To)+")")
	case !f.From.IsZero():
		parts = append(parts, "_time:>="+logsqlTime(f.From))
	case rangeDurations[f.Range] > 0:
		parts = append(parts, "_time:"+f.Range)
	}
	if f.Host != "" {
		parts = append(parts, "host:="+strconv.Quote(f.Host))
	}
	if f.App != "" {
		parts = append(parts, "app:="+strconv.Quote(f.App))
	}
	switch f.Source {
	case "docker", "host", "kernel", "audit", "logstream", "file":
		parts = append(parts, `source_type:=`+strconv.Quote(f.Source))
	case "syslog": // also matches logs stored before source_type existed
		parts = append(parts, `!(source_type:in("docker","host","kernel","audit","logstream","file"))`)
	}
	if f.Severity >= 0 && f.Severity < 7 {
		names := make([]string, 0, 8)
		for i := 0; i <= f.Severity; i++ {
			names = append(names, strconv.Quote(severityNames[i]))
		}
		parts = append(parts, "severity:in("+strings.Join(names, ",")+")")
	}
	if f.Mode != "logsql" {
		// Case-insensitive substring search in the message, host and app.
		for _, t := range parseTerms(f.Text) {
			re := strconv.Quote("(?i)" + regexp.QuoteMeta(t.text))
			expr := "(_msg:~" + re + " or host:~" + re + " or app:~" + re + ")"
			if t.neg {
				expr = "!" + expr
			}
			parts = append(parts, expr)
		}
	}
	return strings.Join(parts, " ")
}

func logsqlTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

// LogsQL returns the filter part and any pipes ("| …") typed in LogsQL mode.
func (f Filter) LogsQL() (filter, pipes string) {
	filter = f.filterExpr()
	if f.Mode == "logsql" && f.Text != "" {
		userFilter, userPipes := splitPipes(f.Text)
		if userFilter != "" && userFilter != "*" {
			filter = strings.TrimSpace(filter + " (" + userFilter + ")")
		}
		pipes = userPipes
	}
	if filter == "" {
		filter = "*"
	}
	return filter, pipes
}

// splitPipes separates the filter from the pipes, ignoring "|" inside quotes.
func splitPipes(q string) (filter, pipes string) {
	var quote byte
	for i := 0; i < len(q); i++ {
		c := q[i]
		switch {
		case quote != 0:
			if c == '\\' && quote != '`' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'' || c == '`':
			quote = c
		case c == '|':
			return strings.TrimSpace(q[:i]), " " + q[i:]
		}
	}
	return strings.TrimSpace(q), ""
}

// Matcher applies the same filter as filterExpr to live messages.
type Matcher struct {
	f    Filter
	res  []*regexp.Regexp
	negs []bool
}

func (f Filter) Matcher() *Matcher {
	m := &Matcher{f: f}
	if f.Mode != "logsql" {
		for _, t := range parseTerms(f.Text) {
			m.res = append(m.res, regexp.MustCompile("(?i)"+regexp.QuoteMeta(t.text)))
			m.negs = append(m.negs, t.neg)
		}
	}
	return m
}

func (m *Matcher) Match(e *Entry) bool {
	if m.f.Host != "" && e.Host != m.f.Host {
		return false
	}
	if m.f.App != "" && e.App != m.f.App {
		return false
	}
	if m.f.Source != "" && m.f.Source != e.SourceType {
		return false
	}
	if m.f.Severity >= 0 && e.SevNum > m.f.Severity {
		return false
	}
	for i, re := range m.res {
		hit := re.MatchString(e.Message) || re.MatchString(e.Host) || re.MatchString(e.App)
		if hit == m.negs[i] {
			return false
		}
	}
	return true
}
