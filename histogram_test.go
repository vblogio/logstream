package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPlanHistogram(t *testing.T) {
	now := time.Date(2026, 10, 1, 18, 7, 42, 0, time.UTC)
	paris := 2 * time.Hour
	kolkata := 5*time.Hour + 30*time.Minute
	day := 24 * time.Hour
	tests := []struct {
		name   string
		span   time.Duration
		want   string
		offset time.Duration
		step   time.Duration
		capped bool
	}{
		{"5m auto", 5 * time.Minute, "auto", paris, 5 * time.Second, false},
		{"1h auto", time.Hour, "auto", paris, time.Minute, false},
		{"24h auto", 24 * time.Hour, "", paris, 15 * time.Minute, false},
		{"7d auto", 7 * day, "auto", paris, 2 * time.Hour, false},
		{"30d auto", 30 * day, "auto", paris, 12 * time.Hour, false},
		{"unknown step = auto", time.Hour, "3s", 0, time.Minute, false},
		{"1s fixed fits", 5 * time.Minute, "1s", 0, time.Second, false},
		{"1s fixed on 1h is enlarged", time.Hour, "1s", 0, 15 * time.Second, true},
		{"1s fixed on 24h is enlarged", 24 * time.Hour, "1s", 0, 5 * time.Minute, true},
		{"1m fixed on 6h", 6 * time.Hour, "1m", 0, 2 * time.Minute, true},
		{"5m fixed on 24h", 24 * time.Hour, "5m", paris, 5 * time.Minute, false},
		{"1h fixed on 30d", 30 * day, "1h", paris, 3 * time.Hour, true},
		{"1d fixed on 30d", 30 * day, "1d", paris, day, false},
		{"1d fixed on 1h", time.Hour, "1d", paris, day, false},
		{"years: whole days", 3 * 365 * day, "auto", 0, 30 * day, false},
		{"beyond the ladder", 30 * 365 * day, "1d", 0, 37 * day, true},
		{"half-hour zone", 24 * time.Hour, "1h", kolkata, time.Hour, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := planHistogram(now.Add(-tc.span), now, tc.want, tc.offset)
			if p.step != tc.step || p.capped != tc.capped {
				t.Errorf("step %v capped %v, want %v capped %v", p.step, p.capped, tc.step, tc.capped)
			}
			if p.count < 1 || p.count > histMaxBuckets {
				t.Errorf("count %d out of [1, %d]", p.count, histMaxBuckets)
			}
			if p.start.After(now.Add(-tc.span)) {
				t.Errorf("start %v after the range start", p.start)
			}
			if end := p.start.Add(time.Duration(p.count) * p.step); end.Before(now) {
				t.Errorf("intervals end at %v, before %v", end, now)
			}
			if local := p.start.Add(tc.offset).UnixNano() % int64(p.step); local != 0 {
				t.Errorf("start %v not aligned on local time (%v)", p.start, time.Duration(local))
			}
		})
	}
}

func TestHistAlign(t *testing.T) {
	paris := 2 * time.Hour
	tests := []struct {
		t      time.Time
		step   time.Duration
		offset time.Duration
		want   time.Time
	}{
		{time.Date(2026, 10, 1, 18, 7, 42, 0, time.UTC), time.Minute, 0, time.Date(2026, 10, 1, 18, 7, 0, 0, time.UTC)},
		// Local midnight in Paris (UTC+2) is 22:00 UTC the day before.
		{time.Date(2026, 10, 1, 18, 7, 42, 0, time.UTC), 24 * time.Hour, paris, time.Date(2026, 9, 30, 22, 0, 0, 0, time.UTC)},
		{time.Date(2026, 10, 1, 23, 0, 0, 0, time.UTC), 24 * time.Hour, paris, time.Date(2026, 10, 1, 22, 0, 0, 0, time.UTC)},
		// New York (UTC-4): midnight is 04:00 UTC.
		{time.Date(2026, 10, 1, 3, 0, 0, 0, time.UTC), 24 * time.Hour, -4 * time.Hour, time.Date(2026, 9, 30, 4, 0, 0, 0, time.UTC)},
		// Before 1970: floor, not truncation toward zero.
		{time.Date(1969, 12, 31, 23, 59, 30, 0, time.UTC), time.Minute, 0, time.Date(1969, 12, 31, 23, 59, 0, 0, time.UTC)},
	}
	for _, tc := range tests {
		if got := histAlign(tc.t, tc.step, tc.offset); !got.Equal(tc.want) {
			t.Errorf("histAlign(%v, %v, %v) = %v, want %v", tc.t, tc.step, tc.offset, got.UTC(), tc.want)
		}
	}
}

func TestQueryStep(t *testing.T) {
	h := time.Hour
	tests := []struct{ step, offset, want time.Duration }{
		{time.Minute, 2 * h, time.Minute},
		{24 * h, 2 * h, 2 * h},
		{24 * h, -4 * h, 4 * h},
		{3 * h, 2 * h, h},
		{24 * h, 5*h + 30*time.Minute, 30 * time.Minute},
		{24 * h, 0, 24 * h},
	}
	for _, tc := range tests {
		if got := queryStep(tc.step, tc.offset); got != tc.want {
			t.Errorf("queryStep(%v, %v) = %v, want %v", tc.step, tc.offset, got, tc.want)
		}
	}
}

func TestLogsqlDuration(t *testing.T) {
	for d, want := range map[time.Duration]string{
		time.Second: "1s", 90 * time.Second: "90s", 5 * time.Minute: "5m", 2 * time.Hour: "2h", 48 * time.Hour: "2d",
	} {
		if got := logsqlDuration(d); got != want {
			t.Errorf("logsqlDuration(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestFilterTimeRange(t *testing.T) {
	from := time.Date(2026, 10, 1, 17, 52, 0, 0, time.UTC)
	to := from.Add(15 * time.Minute)
	f := Filter{Range: "1h", From: from, To: to, Severity: -1}
	if got, want := f.filterExpr(), "_time:[2026-10-01T17:52:00Z, 2026-10-01T18:07:00Z)"; got != want {
		t.Errorf("zoomed filter = %q, want %q", got, want)
	}
	f = Filter{Range: "1h", Severity: -1}
	if got := f.filterExpr(); got != "_time:1h" {
		t.Errorf("relative filter = %q", got)
	}
}

// fakeVL answers LogsQL queries like VictoriaLogs would, without a network.
type fakeVL struct {
	queries []string
	answer  func(query string) string
}

func (f *fakeVL) RoundTrip(r *http.Request) (*http.Response, error) {
	_ = r.ParseForm()
	q := r.PostForm.Get("query")
	f.queries = append(f.queries, q)
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(f.answer(q))), Header: http.Header{}}, nil
}

func TestHistogramHandler(t *testing.T) {
	now := time.Now()
	// Two intervals of the last hour: one with errors and infos, one with a warning.
	recent := now.Add(-3 * time.Minute).UTC().Truncate(time.Minute)
	older := now.Add(-40 * time.Minute).UTC().Truncate(time.Minute)
	vl := &fakeVL{answer: func(string) string {
		return fmt.Sprintf(`{"_time":%q,"severity":"err","hits":"2"}
{"_time":%q,"severity":"info","hits":"5"}
{"_time":%q,"severity":"warning","hits":"1"}
{"_time":"2000-01-01T00:00:00Z","severity":"info","hits":"9"}
`, recent.Format(time.RFC3339), recent.Format(time.RFC3339), older.Format(time.RFC3339))
	}}
	store := NewStore("http://vl", 10, 10, time.Second, nil)
	store.streamClient.Transport = vl
	a := &API{store: store}

	rec := httptest.NewRecorder()
	a.histogram(rec, httptest.NewRequest("GET", "/api/histogram?range=1h&step=auto&tz=Europe/Paris&severity=4", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var res struct {
		Step, Start, Now int64
		Count            int
		Buckets          []struct {
			I   int
			N   int64
			Sev map[string]int64
		}
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Step != 60000 || res.Count != 61 {
		t.Errorf("step %d count %d, want 60000 and 61", res.Step, res.Count)
	}
	q := vl.queries[0]
	for _, want := range []string{"_time:[", `severity:in("emerg","alert","crit","err","warning")`, "| stats by (_time:1m, severity) count() hits"} {
		if !strings.Contains(q, want) {
			t.Errorf("query %q lacks %q", q, want)
		}
	}
	if len(res.Buckets) != 2 {
		t.Fatalf("buckets %+v, want 2 (the one outside the range is dropped)", res.Buckets)
	}
	b := res.Buckets[1]
	if want := int((recent.UnixMilli() - res.Start) / res.Step); b.I != want || b.N != 7 || b.Sev["err"] != 2 || b.Sev["info"] != 5 {
		t.Errorf("recent bucket %+v, want i=%d n=7 err=2 info=5", b, want)
	}
	if res.Buckets[0].Sev["warning"] != 1 {
		t.Errorf("older bucket %+v", res.Buckets[0])
	}
}

func TestHistogramDayInParis(t *testing.T) {
	// 30 days, 1 day intervals in Paris: VictoriaLogs is asked for 2h
	// intervals, summed here into local days (22:00 UTC boundaries in summer).
	vl := &fakeVL{answer: func(q string) string {
		if !strings.Contains(q, "stats by (_time:2h, severity)") {
			return ""
		}
		return `{"_time":"2026-09-20T20:00:00Z","severity":"info","hits":"1"}
{"_time":"2026-09-20T22:00:00Z","severity":"info","hits":"10"}
{"_time":"2026-09-21T20:00:00Z","severity":"info","hits":"100"}
`
	}}
	store := NewStore("http://vl", 10, 10, time.Second, nil)
	store.streamClient.Transport = vl
	a := &API{store: store}
	from := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	rec := httptest.NewRecorder()
	a.histogram(rec, httptest.NewRequest("GET", fmt.Sprintf("/api/histogram?from=%d&to=%d&step=1d&tz=Europe/Paris", from.UnixMilli(), to.UnixMilli()), nil))
	var res struct {
		Start   int64
		Buckets []struct{ I, N int }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("%v: %s (queries %q)", err, rec.Body, vl.queries)
	}
	if got := time.UnixMilli(res.Start).UTC(); !got.Equal(time.Date(2026, 9, 9, 22, 0, 0, 0, time.UTC)) {
		t.Errorf("start %v, want the Paris midnight 2026-09-09T22:00Z", got)
	}
	want := map[int]int{10: 1, 11: 110} // index 0 = Sept 10 in Paris; 20:00Z on the 20th is still the 20th
	if len(res.Buckets) != len(want) {
		t.Fatalf("buckets %+v, want %v", res.Buckets, want)
	}
	for _, b := range res.Buckets {
		if want[b.I] != b.N {
			t.Errorf("bucket %d = %d, want %d", b.I, b.N, want[b.I])
		}
	}
}
