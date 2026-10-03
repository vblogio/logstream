package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DBStats describes what VictoriaLogs holds (Settings > Data). Sizes are in
// bytes; a field VictoriaLogs did not give stays at its zero value.
type DBStats struct {
	Rows       int64  `json:"rows"`       // stored log lines
	DiskBytes  int64  `json:"diskBytes"`  // compressed data + index, on disk
	IndexBytes int64  `json:"indexBytes"` // index part of DiskBytes
	RawBytes   int64  `json:"rawBytes"`   // data before compression
	FreeBytes  int64  `json:"freeBytes"`  // free space on the VictoriaLogs volume
	Days       int64  `json:"days"`       // per-day partitions
	Retention  string `json:"retention"`  // -retentionPeriod, e.g. "30d"
	Oldest     string `json:"oldest,omitempty"`
	Newest     string `json:"newest,omitempty"`
	Last1h     int64  `json:"last1h"`
	Last24h    int64  `json:"last24h"`
	Hosts      int64  `json:"hosts"`
	Apps       int64  `json:"apps"`
	QueryError string `json:"queryError,omitempty"` // the LogsQL part failed, the metrics are still valid
	Updated    string `json:"updated"`
}

// dbStatsCache keeps the last answer for a while: the LogsQL part reads the
// whole base, which is too heavy to repeat each time the tab is opened.
type dbStatsCache struct {
	mu   sync.Mutex
	at   time.Time
	last *DBStats
}

const dbStatsTTL = 30 * time.Second

func (c *dbStatsCache) get(ctx context.Context, s *Store, force bool) (*DBStats, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.last != nil && !force && time.Since(c.at) < dbStatsTTL {
		return c.last, nil
	}
	st, err := s.DBStats(ctx)
	if err != nil {
		return nil, err
	}
	c.last, c.at = st, time.Now()
	return st, nil
}

// DBStats reads the storage metrics of VictoriaLogs (/metrics), then counts
// with LogsQL what the metrics do not tell (period, recent volume, sources).
func (s *Store) DBStats(ctx context.Context) (*DBStats, error) {
	st, err := s.storageMetrics(ctx)
	if err != nil {
		return nil, err
	}
	st.Updated = time.Now().UTC().Format(time.RFC3339)
	if st.Rows == 0 {
		return st, nil
	}

	qctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	rows, err := s.Query(qctx, "* | stats min(_time) oldest, max(_time) newest, count_uniq(host) hosts, count_uniq(app) apps")
	if err == nil && len(rows) > 0 {
		r := rows[0]
		st.Oldest, _ = r["oldest"].(string)
		st.Newest, _ = r["newest"].(string)
		st.Hosts, st.Apps = toInt(r["hosts"]), toInt(r["apps"])
		rows, err = s.Query(qctx, "_time:24h | stats count() last24h, count() if (_time:1h) last1h")
		if err == nil && len(rows) > 0 {
			st.Last24h, st.Last1h = toInt(rows[0]["last24h"]), toInt(rows[0]["last1h"])
		}
	}
	if err != nil {
		st.QueryError = err.Error()
	}
	return st, nil
}

func (s *Store) storageMetrics(ctx context.Context) (*DBStats, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+"/metrics", nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("VictoriaLogs /metrics: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return parseStorageMetrics(resp.Body)
}

// parseStorageMetrics picks the storage figures out of the Prometheus text
// format of VictoriaLogs' /metrics (app/vlstorage).
func parseStorageMetrics(r io.Reader) (*DBStats, error) {
	st := &DBStats{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || line[0] == '#' {
			continue
		}
		name, labels, value := splitMetric(line)
		switch name {
		case "vl_storage_rows":
			st.Rows += int64(value)
		case "vl_data_size_bytes":
			st.DiskBytes += int64(value)
			if labels["type"] == "indexdb" {
				st.IndexBytes += int64(value)
			}
		case "vl_uncompressed_data_size_bytes":
			st.RawBytes += int64(value)
		case "vl_free_disk_space_bytes":
			st.FreeBytes = int64(value)
		case "vl_partitions":
			st.Days = int64(value)
		case "flag":
			if labels["name"] == "retentionPeriod" {
				st.Retention = labels["value"]
			}
		}
	}
	return st, sc.Err()
}

// splitMetric splits `name{a="x",b="y"} 12` into its parts. Label values with
// escaped quotes are not expected in the metrics read here.
func splitMetric(line string) (string, map[string]string, float64) {
	sp := strings.LastIndexByte(line, ' ')
	if sp < 0 {
		return "", nil, 0
	}
	value, _ := strconv.ParseFloat(line[sp+1:], 64)
	head := line[:sp]
	name, rest, ok := strings.Cut(head, "{")
	if !ok {
		return head, nil, value
	}
	labels := map[string]string{}
	rest = strings.TrimSuffix(rest, "}")
	for rest != "" {
		k, v, ok := strings.Cut(rest, `="`)
		if !ok {
			break
		}
		val, after, _ := strings.Cut(v, `"`)
		labels[strings.TrimSpace(k)] = val
		rest = strings.TrimPrefix(after, ",")
	}
	return name, labels, value
}
