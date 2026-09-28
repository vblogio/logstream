package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Columns of the CSV export: stored field -> column name.
var exportColumns = []struct{ field, name string }{
	{"received", "received"},
	{"msg_time", "message_time"},
	{"severity", "severity"},
	{"facility", "facility"},
	{"host", "host"},
	{"host_ip", "host_ip"},
	{"app", "app"},
	{"procid", "pid"},
	{"source", "source_ip"},
	{"proto", "proto"},
	{"_msg", "message"},
}

// UTF-8 byte order mark: lets Excel detect the encoding (accents).
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// GET /api/export.csv?<filters>&tz=Europe/Paris&excel=1&limit=N
// Streams every stored log matching the filters (newest first, at most
// EXPORT_MAX rows) as CSV. excel=1 uses ";" as separator (French Excel),
// adds a BOM and neutralizes cells Excel would run as formulas.
func (a *API) exportCSV(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := FilterFromRequest(r)
	filter, pipes := f.LogsQL()
	if strings.TrimSpace(pipes) != "" {
		writeErr(w, http.StatusBadRequest, &codedError{code: "export_pipes", msg: "export does not support LogsQL pipes (| ...): keep only the filter part"})
		return
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > a.exportMax {
		limit = a.exportMax
	}
	loc := time.UTC
	if tz := q.Get("tz"); tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}
	excel := q.Get("excel") == "1"
	query := fmt.Sprintf("%s | sort by (_time desc) | limit %d", filter, limit)

	// The response starts with the first row, so that a query error can
	// still be returned as a proper HTTP error.
	var cw *csv.Writer
	start := func() {
		name := "logstream-" + time.Now().In(loc).Format("20060102-150405") + ".csv"
		h := w.Header()
		h.Set("Content-Type", "text/csv; charset=utf-8")
		h.Set("Content-Disposition", `attachment; filename="`+name+`"`)
		h.Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		if excel {
			_, _ = w.Write(utf8BOM)
		}
		cw = csv.NewWriter(w)
		if excel {
			cw.Comma = ';'
		}
		zone := loc.String()
		header := make([]string, len(exportColumns))
		for i, c := range exportColumns {
			header[i] = c.name
			if c.field == "received" || c.field == "msg_time" {
				header[i] += " (" + zone + ")"
			}
		}
		_ = cw.Write(header)
	}

	rows := 0
	record := make([]string, len(exportColumns))
	err := a.store.QueryStream(r.Context(), query, func(row map[string]any) error {
		if cw == nil {
			start()
		}
		for i, c := range exportColumns {
			v, _ := row[c.field].(string)
			switch c.field {
			case "received":
				v = exportTime(v, loc)
			case "msg_time":
				if v == "" { // logs stored before msg_time existed
					v, _ = row["_time"].(string)
				}
				v = exportTime(v, loc)
			}
			if excel {
				v = excelSafe(v)
			}
			record[i] = v
		}
		if err := cw.Write(record); err != nil {
			return err
		}
		rows++
		if rows%1000 == 0 {
			cw.Flush()
		}
		return cw.Error()
	})
	if err != nil {
		if cw == nil {
			writeErr(w, queryStatus(err), err)
			return
		}
		// Headers are already sent: the file is truncated, log why.
		log.Printf("export interrupted after %d rows: %v", rows, err)
	}
	if cw == nil {
		start() // no matching log: header only
	}
	cw.Flush()
}

// exportTime converts an RFC 3339 timestamp to "2006-01-02 15:04:05.000" in loc,
// a format spreadsheets recognize as a date.
func exportTime(v string, loc *time.Location) string {
	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		return v
	}
	return t.In(loc).Format("2006-01-02 15:04:05.000")
}

// excelSafe prevents spreadsheet formula injection: log messages come from the
// network, and a cell starting with = + - @ would be run as a formula.
func excelSafe(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		return "'" + v
	}
	return v
}
