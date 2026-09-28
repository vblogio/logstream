package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"
)

type API struct {
	store *Store
	hub   *Hub
	tags  *TagStore
}

func (a *API) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("GET /api/logs", a.logs)
	mux.HandleFunc("GET /api/histogram", a.histogram)
	mux.HandleFunc("GET /api/facets", a.facets)
	mux.HandleFunc("GET /api/stream", a.stream)
	mux.HandleFunc("GET /api/stats", a.stats)
	mux.HandleFunc("GET /api/tags", a.listTags)
	mux.HandleFunc("POST /api/tags", a.createTag)
	mux.HandleFunc("POST /api/tags/reset", a.resetTags)
	mux.HandleFunc("PUT /api/tags/{id}", a.updateTag)
	mux.HandleFunc("DELETE /api/tags/{id}", a.deleteTag)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr returns {"error": "...", "code": "...", "detail": "..."}; the UI
// translates known codes and falls back to the English message.
func writeErr(w http.ResponseWriter, status int, err error) {
	body := map[string]string{"error": err.Error()}
	var ce *codedError
	if errors.As(err, &ce) {
		body["code"] = ce.code
		if ce.detail != "" {
			body["detail"] = ce.detail
		}
	}
	writeJSON(w, status, body)
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("ok"))
}

// GET /api/logs?q=&mode=&range=&host=&app=&severity=&limit=
func (a *API) logs(w http.ResponseWriter, r *http.Request) {
	f := FilterFromRequest(r)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 5000 {
		limit = 500
	}
	filter, pipes := f.LogsQL()
	q := fmt.Sprintf("%s%s | sort by (_time desc) | limit %d", filter, pipes, limit)

	start := time.Now()
	rows, err := a.store.Query(r.Context(), q)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"query":   q,
		"rows":    rows,
		"took_ms": time.Since(start).Milliseconds(),
	})
}

var histSteps = map[string]struct {
	expr string
	sec  int
}{
	"5m": {"10s", 10}, "15m": {"30s", 30}, "1h": {"1m", 60}, "6h": {"5m", 300},
	"24h": {"15m", 900}, "7d": {"2h", 7200}, "30d": {"12h", 43200},
}

// GET /api/histogram: message count per time bucket.
func (a *API) histogram(w http.ResponseWriter, r *http.Request) {
	f := FilterFromRequest(r)
	step, ok := histSteps[f.Range]
	if !ok {
		step.expr, step.sec = "1d", 86400
	}
	filter, _ := f.LogsQL()
	q := filter + " | stats by (_time:" + step.expr + ") count() hits | sort by (_time)"
	rows, err := a.store.Query(r.Context(), q)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	type bucket struct {
		T string `json:"t"`
		N int64  `json:"n"`
	}
	out := make([]bucket, 0, len(rows))
	for _, row := range rows {
		t, _ := row["_time"].(string)
		out = append(out, bucket{T: t, N: toInt(row["hits"])})
	}
	writeJSON(w, http.StatusOK, map[string]any{"step": step.sec, "buckets": out})
}

func toInt(v any) int64 {
	switch x := v.(type) {
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	case float64:
		return int64(x)
	}
	return 0
}

// GET /api/facets: hosts and apps seen over 7 days, for the filter dropdowns.
func (a *API) facets(w http.ResponseWriter, r *http.Request) {
	res := map[string][]string{}
	for _, field := range []string{"host", "app"} {
		q := "_time:7d | stats by (" + field + ") count() hits | sort by (hits desc) | limit 300"
		rows, err := a.store.Query(r.Context(), q)
		if err != nil {
			writeErr(w, http.StatusBadGateway, err)
			return
		}
		vals := make([]string, 0, len(rows))
		for _, row := range rows {
			if v, _ := row[field].(string); v != "" {
				vals = append(vals, v)
			}
		}
		sort.Strings(vals)
		res[field] = vals
	}
	writeJSON(w, http.StatusOK, res)
}

// GET /api/stream: SSE stream of new messages matching the filter,
// sent in batches every 250 ms.
func (a *API) stream(w http.ResponseWriter, r *http.Request) {
	f := FilterFromRequest(r)
	if f.Mode == "logsql" && f.Text != "" {
		writeErr(w, http.StatusBadRequest, &codedError{code: "live_logsql", msg: "live view is not available in LogsQL mode"})
		return
	}
	m := f.Matcher()
	rc := http.NewResponseController(w)

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, ": connected\n\n")
	_ = rc.Flush()

	ch := a.hub.Subscribe()
	defer a.hub.Unsubscribe(ch)
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()

	const maxBatch = 500
	var pending []map[string]string
	skipped, idle := 0, 0
	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-ch:
			if !m.Match(e) {
				continue
			}
			if len(pending) < maxBatch {
				pending = append(pending, e.Record())
			} else {
				skipped++
			}
		case <-tick.C:
			var err error
			if len(pending) == 0 && skipped == 0 {
				// SSE comment every 15 s to keep the connection open.
				if idle++; idle < 60 {
					continue
				}
				idle = 0
				_, err = fmt.Fprint(w, ": ping\n\n")
			} else {
				b, _ := json.Marshal(map[string]any{"rows": pending, "skipped": skipped})
				pending, skipped, idle = pending[:0], 0, 0
				_, err = fmt.Fprintf(w, "data: %s\n\n", b)
			}
			if err == nil {
				err = rc.Flush()
			}
			if err != nil {
				return
			}
		}
	}
}

func (a *API) stats(w http.ResponseWriter, r *http.Request) {
	s := a.store.Stats()
	s["clients"] = a.hub.Count()
	writeJSON(w, http.StatusOK, s)
}

func (a *API) listTags(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.tags.List())
}

func decodeTag(w http.ResponseWriter, r *http.Request) (Tag, error) {
	var t Tag
	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&t)
	return t, err
}

func (a *API) createTag(w http.ResponseWriter, r *http.Request) {
	t, err := decodeTag(w, r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if t, err = a.tags.Create(t); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (a *API) updateTag(w http.ResponseWriter, r *http.Request) {
	t, err := decodeTag(w, r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	t, err = a.tags.Update(r.PathValue("id"), t)
	switch {
	case errors.Is(err, errTagNotFound):
		writeErr(w, http.StatusNotFound, err)
	case err != nil:
		writeErr(w, http.StatusBadRequest, err)
	default:
		writeJSON(w, http.StatusOK, t)
	}
}

func (a *API) deleteTag(w http.ResponseWriter, r *http.Request) {
	err := a.tags.Delete(r.PathValue("id"))
	switch {
	case errors.Is(err, errTagNotFound):
		writeErr(w, http.StatusNotFound, err)
	case err != nil:
		writeErr(w, http.StatusInternalServerError, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *API) resetTags(w http.ResponseWriter, r *http.Request) {
	tags, err := a.tags.Reset()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, tags)
}
