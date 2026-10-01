package main

import (
	"net/http"
	"strconv"
	"time"
)

const (
	histAutoBuckets = 100 // target number of intervals with the automatic division
	histMaxBuckets  = 300 // hard limit, whatever the division asked for
)

// Divisions the automatic mode picks from; a fixed division too fine for the
// span is enlarged to the first of them that fits in histMaxBuckets.
var histLadder = []time.Duration{
	time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 15 * time.Second, 30 * time.Second,
	time.Minute, 2 * time.Minute, 5 * time.Minute, 10 * time.Minute, 15 * time.Minute, 30 * time.Minute,
	time.Hour, 2 * time.Hour, 3 * time.Hour, 6 * time.Hour, 12 * time.Hour,
	24 * time.Hour, 2 * 24 * time.Hour, 7 * 24 * time.Hour, 30 * 24 * time.Hour,
}

// Fixed divisions offered in Settings > Interface > Timeline.
var histFixedSteps = map[string]time.Duration{
	"1s": time.Second, "10s": 10 * time.Second, "1m": time.Minute,
	"5m": 5 * time.Minute, "1h": time.Hour, "1d": 24 * time.Hour,
}

// histPlan is the bucketing of a time range: count intervals of step,
// starting at start, aligned on the local time (offset east of UTC).
type histPlan struct {
	step   time.Duration
	start  time.Time
	count  int
	capped bool // the fixed division asked for was enlarged
}

// histAlign returns the start of the interval containing t; intervals are
// aligned so that local midnight, local hours… fall on boundaries.
func histAlign(t time.Time, step, offset time.Duration) time.Time {
	ns := t.UnixNano() + int64(offset)
	s := int64(step)
	k := ns / s
	if ns%s < 0 {
		k--
	}
	return time.Unix(0, k*s-int64(offset))
}

// histBuckets returns the plan for [from, to) with this step.
func histBuckets(from, to time.Time, step, offset time.Duration) histPlan {
	start := histAlign(from, step, offset)
	n := int((to.Sub(start) + step - 1) / step)
	if n < 1 {
		n = 1
	}
	return histPlan{step: step, start: start, count: n}
}

// planHistogram chooses the division of [from, to): `want` is "auto" (about
// histAutoBuckets intervals) or one of histFixedSteps (enlarged when it would
// exceed histMaxBuckets intervals).
func planHistogram(from, to time.Time, want string, offset time.Duration) histPlan {
	if !to.After(from) {
		to = from.Add(time.Second)
	}
	limit := histAutoBuckets
	var fixed time.Duration
	if d, ok := histFixedSteps[want]; ok {
		fixed, limit = d, histMaxBuckets
		if p := histBuckets(from, to, fixed, offset); p.count <= histMaxBuckets {
			return p
		}
	}
	for _, step := range histLadder {
		if step < fixed {
			continue
		}
		if p := histBuckets(from, to, step, offset); p.count <= limit {
			p.capped = fixed > 0
			return p
		}
	}
	// Longer than the ladder allows (years): whole days, as many as needed.
	day := 24 * time.Hour
	step := (to.Sub(from)/time.Duration(histMaxBuckets-1) + day - 1) / day * day
	p := histBuckets(from, to, step, offset)
	p.capped = fixed > 0
	return p
}

// queryStep is the division asked to VictoriaLogs: its intervals are aligned
// on UTC, so when the local offset does not fall on a boundary (1d in Paris),
// finer intervals are summed here into the local ones.
func queryStep(step, offset time.Duration) time.Duration {
	a, b := step, (offset%step+step)%step
	for b != 0 {
		a, b = b, a%b
	}
	if a < time.Second {
		return time.Second
	}
	return a
}

// logsqlDuration writes a step the way LogsQL expects it (1d, 2h, 5m, 10s).
func logsqlDuration(d time.Duration) string {
	switch {
	case d%(24*time.Hour) == 0:
		return strconv.FormatInt(int64(d/(24*time.Hour)), 10) + "d"
	case d%time.Hour == 0:
		return strconv.FormatInt(int64(d/time.Hour), 10) + "h"
	case d%time.Minute == 0:
		return strconv.FormatInt(int64(d/time.Minute), 10) + "m"
	}
	return strconv.FormatInt(int64(d/time.Second), 10) + "s"
}

// GET /api/histogram?<filters>&step=auto|1s|10s|1m|5m|1h|1d&tz=Europe/Paris
// Message count per interval, with the detail per severity. Every bound is
// returned in Unix milliseconds, from the server clock.
func (a *API) histogram(w http.ResponseWriter, r *http.Request) {
	f := FilterFromRequest(r)
	now := time.Now()
	from, to := f.From, f.To
	switch {
	case !from.IsZero():
		if to.IsZero() {
			to = now
		}
	case rangeDurations[f.Range] > 0:
		from, to = now.Add(-rangeDurations[f.Range]), now
	default: // all logs: from the oldest one matching the filters
		from, to = now.Add(-time.Hour), now
		filter, _ := f.LogsQL()
		rows, err := a.store.Query(r.Context(), filter+" | stats min(_time) first")
		if err != nil {
			writeErr(w, queryStatus(err), err)
			return
		}
		if len(rows) > 0 {
			if s, _ := rows[0]["first"].(string); s != "" {
				if t, err := time.Parse(time.RFC3339Nano, s); err == nil && t.Before(from) {
					from = t
				}
			}
		}
	}

	loc := time.Local
	if tz := r.URL.Query().Get("tz"); tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}
	_, offSec := to.In(loc).Zone()
	offset := time.Duration(offSec) * time.Second
	p := planHistogram(from, to, r.URL.Query().Get("step"), offset)

	// Whole intervals, so that the first bar is not cut by the range start.
	hf := f
	hf.From, hf.To = p.start, p.start.Add(time.Duration(p.count)*p.step)
	filter, _ := hf.LogsQL()
	rows, err := a.store.Query(r.Context(), filter+" | stats by (_time:"+logsqlDuration(queryStep(p.step, offset))+", severity) count() hits")
	if err != nil {
		writeErr(w, queryStatus(err), err)
		return
	}

	type bucket struct {
		I   int              `json:"i"`   // interval index, from 0
		N   int64            `json:"n"`   // total
		Sev map[string]int64 `json:"sev"` // count per severity name
	}
	byIdx := map[int]*bucket{}
	for _, row := range rows {
		s, _ := row["_time"].(string)
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			continue
		}
		i := int(histAlign(t, p.step, offset).Sub(p.start) / p.step)
		if i < 0 || i >= p.count {
			continue
		}
		b := byIdx[i]
		if b == nil {
			b = &bucket{I: i, Sev: map[string]int64{}}
			byIdx[i] = b
		}
		n := toInt(row["hits"])
		sev, _ := row["severity"].(string)
		if sev == "" {
			sev = "info"
		}
		b.N += n
		b.Sev[sev] += n
	}
	out := make([]*bucket, 0, len(byIdx))
	for i := 0; i < p.count; i++ {
		if b := byIdx[i]; b != nil {
			out = append(out, b)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"step":    int64(p.step / time.Millisecond),
		"start":   p.start.UnixMilli(),
		"count":   p.count,
		"from":    from.UnixMilli(),
		"to":      to.UnixMilli(),
		"now":     now.UnixMilli(),
		"capped":  p.capped,
		"buckets": out,
	})
}
