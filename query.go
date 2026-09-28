package main

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// Filter décrit les critères de recherche envoyés par l'interface.
type Filter struct {
	Mode     string // "simple" ou "logsql"
	Text     string
	Range    string // 5m, 15m, 1h, 6h, 24h, 7d, 30d ; autre valeur = pas de limite
	Host     string
	App      string
	Severity int // sévérité maximale incluse (0 = emerg … 7 = debug), -1 = toutes
}

var validRanges = map[string]bool{"5m": true, "15m": true, "1h": true, "6h": true, "24h": true, "7d": true, "30d": true}

func FilterFromRequest(r *http.Request) Filter {
	q := r.URL.Query()
	f := Filter{
		Mode:     q.Get("mode"),
		Text:     strings.TrimSpace(q.Get("q")),
		Range:    q.Get("range"),
		Host:     q.Get("host"),
		App:      q.Get("app"),
		Severity: -1,
	}
	if v, err := strconv.Atoi(q.Get("severity")); err == nil && v >= 0 && v <= 7 {
		f.Severity = v
	}
	return f
}

type term struct {
	text string
	neg  bool
}

// parseTerms découpe la recherche simple : mots, "phrases entre guillemets", -exclusions.
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

// filterExpr traduit les critères (hors texte LogsQL brut) en filtre LogsQL.
func (f Filter) filterExpr() string {
	var parts []string
	if validRanges[f.Range] {
		parts = append(parts, "_time:"+f.Range)
	}
	if f.Host != "" {
		parts = append(parts, "host:="+strconv.Quote(f.Host))
	}
	if f.App != "" {
		parts = append(parts, "app:="+strconv.Quote(f.App))
	}
	if f.Severity >= 0 && f.Severity < 7 {
		names := make([]string, 0, 8)
		for i := 0; i <= f.Severity; i++ {
			names = append(names, strconv.Quote(severityNames[i]))
		}
		parts = append(parts, "severity:in("+strings.Join(names, ",")+")")
	}
	if f.Mode != "logsql" {
		// Recherche de sous-chaîne insensible à la casse dans le message, l'hôte et l'appli.
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

// LogsQL renvoie la partie filtre et les éventuels pipes (« | … ») saisis en mode LogsQL.
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

// splitPipes sépare le filtre des pipes, en ignorant les « | » entre guillemets.
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

// Matcher applique le même filtre que filterExpr aux messages du flux direct.
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
