package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Excerpt of VictoriaLogs v1.52 /metrics.
const vlMetrics = `# a comment
vl_free_disk_space_bytes{path="/vlogs"} 52428800000
vl_storage_rows{type="storage/inmemory"} 120
vl_storage_rows{type="storage/small"} 3000
vl_storage_rows{type="storage/big"} 1000000
vl_partitions 12
vl_data_size_bytes{type="indexdb"} 2048
vl_data_size_bytes{type="storage"} 1048576
vl_compressed_data_size_bytes{type="storage/big"} 1000000
vl_uncompressed_data_size_bytes{type="storage/inmemory"} 4096
vl_uncompressed_data_size_bytes{type="storage/small"} 100000
vl_uncompressed_data_size_bytes{type="storage/big"} 20000000
flag{name="retentionPeriod", value="30d", is_set="true"} 1
flag{name="httpListenAddr", value=":9428", is_set="true"} 1
`

func TestParseStorageMetrics(t *testing.T) {
	st, err := parseStorageMetrics(strings.NewReader(vlMetrics))
	if err != nil {
		t.Fatal(err)
	}
	want := DBStats{Rows: 1003120, DiskBytes: 1050624, IndexBytes: 2048, RawBytes: 20104096, FreeBytes: 52428800000, Days: 12, Retention: "30d"}
	if *st != want {
		t.Fatalf("got %+v\nwant %+v", *st, want)
	}
}

// statsVL answers /metrics and the LogsQL queries of DBStats.
type statsVL struct{ queries int }

func (v *statsVL) RoundTrip(r *http.Request) (*http.Response, error) {
	body := vlMetrics
	if r.URL.Path == "/select/logsql/query" {
		v.queries++
		_ = r.ParseForm()
		if strings.HasPrefix(r.PostForm.Get("query"), "_time:24h") {
			body = `{"last24h":"5000","last1h":"300"}` + "\n"
		} else {
			body = `{"oldest":"2026-09-21T08:00:00Z","newest":"2026-10-03T15:00:00Z","hosts":"4","apps":"17"}` + "\n"
		}
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
}

func TestDBStatsHandler(t *testing.T) {
	vl := &statsVL{}
	store := NewStore("http://vl", 10, 10, time.Second, nil)
	store.client.Transport = vl
	store.streamClient.Transport = vl
	a := &API{store: store}

	get := func(url string) DBStats {
		rec := httptest.NewRecorder()
		a.dbStats(rec, httptest.NewRequest("GET", url, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		var st DBStats
		if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
			t.Fatal(err)
		}
		return st
	}
	st := get("/api/dbstats")
	if st.Rows != 1003120 || st.Oldest != "2026-09-21T08:00:00Z" || st.Hosts != 4 || st.Apps != 17 || st.Last24h != 5000 || st.Last1h != 300 || st.QueryError != "" {
		t.Fatalf("unexpected stats: %+v", st)
	}
	get("/api/dbstats")
	if vl.queries != 2 {
		t.Fatalf("second call within the TTL should be cached, got %d queries", vl.queries)
	}
	get("/api/dbstats?refresh=1")
	if vl.queries != 4 {
		t.Fatalf("refresh=1 should query again, got %d queries", vl.queries)
	}
}
