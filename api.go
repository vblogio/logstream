package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sort"
	"strconv"
	"time"
)

type API struct {
	store      *Store
	hub        *Hub
	tags       *TagStore
	presets    string // presets file, built-in presets when missing
	rdns       *ReverseDNS
	allowPurge bool
	exportMax  int
	docker     *DockerManager // nil when DOCKER_LOGS is off
	syslog     *SyslogServer
	host       *HostLogs
	dbstats    dbStatsCache
}

func (a *API) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("GET /api/logs", a.logs)
	mux.HandleFunc("GET /api/histogram", a.histogram)
	mux.HandleFunc("GET /api/facets", a.facets)
	mux.HandleFunc("GET /api/stream", a.stream)
	mux.HandleFunc("GET /api/stats", a.stats)
	mux.HandleFunc("GET /api/dbstats", a.dbStats)
	mux.HandleFunc("GET /api/tags", a.listTags)
	mux.HandleFunc("POST /api/tags", a.createTag)
	mux.HandleFunc("POST /api/tags/reset", a.resetTags)
	mux.HandleFunc("GET /api/presets", a.listPresets)
	mux.HandleFunc("PUT /api/tags/{id}", a.updateTag)
	mux.HandleFunc("DELETE /api/tags/{id}", a.deleteTag)
	mux.HandleFunc("GET /api/purge", a.purgeStatus)
	mux.HandleFunc("POST /api/purge", a.purge)
	mux.HandleFunc("GET /api/export.csv", a.exportCSV)
	mux.HandleFunc("GET /api/syslog", a.syslogStatus)
	mux.HandleFunc("PUT /api/syslog", a.syslogConfigure)
	mux.HandleFunc("GET /api/docker", a.dockerStatus)
	mux.HandleFunc("PUT /api/docker", a.dockerConfigure)
	mux.HandleFunc("GET /api/hostlogs", a.hostLogsStatus)
	mux.HandleFunc("PUT /api/hostlogs", a.hostLogsConfigure)
}

// GET /api/hostlogs: state of the host system logs source (Settings > Sources).
func (a *API) hostLogsStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.host.Status())
}

// PUT /api/hostlogs {"enabled": bool}
func (a *API) hostLogsConfigure(w http.ResponseWriter, r *http.Request) {
	var cfg hostLogsConfig
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&cfg); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := a.host.Configure(cfg); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	log.Printf("host system logs changed from %s: %+v", r.RemoteAddr, cfg)
	writeJSON(w, http.StatusOK, a.host.Status())
}

// GET /api/syslog: syslog reception state (Settings > Sources).
func (a *API) syslogStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.syslog.Status())
}

// PUT /api/syslog {"enabled": bool, "udp": bool, "tcp": bool}
func (a *API) syslogConfigure(w http.ResponseWriter, r *http.Request) {
	var cfg syslogConfig
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&cfg); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := a.syslog.Configure(cfg); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	log.Printf("syslog reception changed from %s: %+v", r.RemoteAddr, cfg)
	writeJSON(w, http.StatusOK, a.syslog.Status())
}

// GET /api/docker: Docker source state and containers (Settings > Sources).
func (a *API) dockerStatus(w http.ResponseWriter, r *http.Request) {
	if a.docker == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
		return
	}
	writeJSON(w, http.StatusOK, a.docker.Status())
}

// PUT /api/docker {"defaultEnabled": bool, "containers": {"project/service": bool}}
func (a *API) dockerConfigure(w http.ResponseWriter, r *http.Request) {
	if a.docker == nil {
		writeErr(w, http.StatusConflict, &codedError{code: "docker_off", msg: "the Docker source is disabled (DOCKER_LOGS=off)"})
		return
	}
	var body struct {
		DefaultEnabled *bool           `json:"defaultEnabled"`
		Containers     map[string]bool `json:"containers"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256*1024)).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := a.docker.Configure(body.DefaultEnabled, body.Containers); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, a.docker.Status())
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
		writeErr(w, queryStatus(err), err)
		return
	}
	a.annotateHosts(rows)
	writeJSON(w, http.StatusOK, map[string]any{
		"query":   q,
		"rows":    rows,
		"took_ms": time.Since(start).Milliseconds(),
	})
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

// annotateHosts adds "host_name" to rows whose host is still an IP address
// (logs stored before DNS resolution, or resolved too slowly at reception).
func (a *API) annotateHosts(rows []map[string]any) {
	seen := map[string]bool{}
	var ips []string
	for _, row := range rows {
		if h, _ := row["host"].(string); h != "" && !seen[h] && net.ParseIP(h) != nil {
			seen[h] = true
			ips = append(ips, h)
		}
	}
	if len(ips) == 0 {
		return
	}
	if len(ips) > 100 {
		ips = ips[:100]
	}
	names := a.rdns.LookupMany(ips, time.Second)
	for _, row := range rows {
		if h, _ := row["host"].(string); names[h] != "" {
			row["host_name"] = names[h]
		}
	}
}

type facet struct {
	Value string `json:"value"` // stored value, used for filtering
	Label string `json:"label"` // displayed text ("name (ip)" for resolved IPs)
}

// GET /api/facets: hosts and apps seen over 7 days, for the filter dropdowns.
func (a *API) facets(w http.ResponseWriter, r *http.Request) {
	res := map[string][]facet{}
	for _, field := range []string{"host", "app"} {
		q := "_time:7d | stats by (" + field + ") count() hits | sort by (hits desc) | limit 300"
		rows, err := a.store.Query(r.Context(), q)
		if err != nil {
			writeErr(w, queryStatus(err), err)
			return
		}
		vals := make([]string, 0, len(rows))
		var ips []string
		for _, row := range rows {
			if v, _ := row[field].(string); v != "" {
				vals = append(vals, v)
				if field == "host" && net.ParseIP(v) != nil {
					ips = append(ips, v)
				}
			}
		}
		names := a.rdns.LookupMany(ips, time.Second)
		list := make([]facet, 0, len(vals))
		for _, v := range vals {
			label := v
			if n := names[v]; n != "" {
				label = n + " (" + v + ")"
			}
			list = append(list, facet{Value: v, Label: label})
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Label < list[j].Label })
		res[field] = list
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
	s["exportMax"] = a.exportMax
	writeJSON(w, http.StatusOK, s)
}

// GET /api/dbstats[?refresh=1]: size and content of the VictoriaLogs base
// (Settings > Data), cached for 30 s.
func (a *API) dbStats(w http.ResponseWriter, r *http.Request) {
	st, err := a.dbstats.get(r.Context(), a.store, r.URL.Query().Get("refresh") == "1")
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (a *API) listTags(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.tags.List())
}

func (a *API) listPresets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, loadPresets(a.presets))
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

// GET /api/purge: whether purging is allowed and how many deletions are running.
func (a *API) purgeStatus(w http.ResponseWriter, r *http.Request) {
	res := map[string]any{"allowed": a.allowPurge, "running": 0}
	if a.allowPurge {
		n, err := a.store.PurgeRunning(r.Context())
		if err != nil {
			res["error"] = err.Error()
			var ce *codedError
			if errors.As(err, &ce) {
				res["code"] = ce.code
			}
		}
		res["running"] = n
	}
	writeJSON(w, http.StatusOK, res)
}

// POST /api/purge {"confirm":"PURGE"}: deletes every stored log.
func (a *API) purge(w http.ResponseWriter, r *http.Request) {
	if !a.allowPurge {
		writeErr(w, http.StatusForbidden, &codedError{code: "purge_forbidden", msg: "purging is disabled (ALLOW_PURGE=false)"})
		return
	}
	var body struct {
		Confirm string `json:"confirm"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body)
	if body.Confirm != "PURGE" {
		writeErr(w, http.StatusBadRequest, &codedError{code: "purge_confirm", msg: `type "PURGE" to confirm`})
		return
	}
	id, err := a.store.Purge(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	log.Printf("purge of all logs requested from %s (task %s)", r.RemoteAddr, id)
	writeJSON(w, http.StatusOK, map[string]string{"task_id": id})
}

func (a *API) resetTags(w http.ResponseWriter, r *http.Request) {
	tags, err := a.tags.Reset()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, tags)
}
