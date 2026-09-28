# Logstream

A simple syslog sink: receives logs over UDP/TCP on port 514, stores them in
[VictoriaLogs](https://docs.victoriametrics.com/victorialogs/), and serves a clean web
interface (light/dark theme, live search, live view, color tags, English/French UI).

```
devices ──514 udp/tcp──▶ logstream (Go) ──HTTP batches──▶ VictoriaLogs
                              ▲    └── SSE (live) ──▶ browser
                              └──── API / UI ◀─────────┘
```

## Getting started

```bash
cp .env.example .env      # optional
docker compose up -d --build
./tools/send-test-logs.sh # sends 100 test messages
```

Then open <http://localhost:8080>.

## Sending logs

- **rsyslog** (Linux): add `*.* @SERVER_IP:514` (UDP) or `*.* @@SERVER_IP:514` (TCP)
  to `/etc/rsyslog.d/90-logstream.conf`, then run `systemctl restart rsyslog`.
- **Network gear, NAS, firewalls**: set the server IP and port 514 in their "remote syslog" settings.
- **Manual test**: `logger -n 127.0.0.1 -P 514 -d "hello error"` (util-linux) or
  `echo "<14>test ok" | nc -u -w1 127.0.0.1 514`.

Supported formats: RFC 3164 (BSD) and RFC 5424. Over TCP, both "one message per line"
and "octet counting" (RFC 6587) framing are accepted.

## Search

**Simple mode** (default): each word is matched as a case-insensitive substring of the
message, host and app. Words are combined with AND.

| Input | Meaning |
|---|---|
| `error disk` | contains "error" **and** "disk" |
| `"disk full"` | contains the exact phrase |
| `error -timeout` | contains "error" but not "timeout" |

**LogsQL mode** (`Simple` / `LogsQL` button): the full VictoriaLogs query language, e.g.
`error AND host:web-01`, `app:~"ssh|nginx"`, or `* | stats by (host) count()`.
The live view is disabled in this mode.

Each row shows, from left to right: the **reception time** (server clock), the timestamp
found in the message itself (`msg_time`), severity, host, app and message. Click a host or an
app to filter on it.

Logs are indexed, searched and sorted by **reception time**: devices with a wrong clock
(e.g. access points whose NTP fails) still show up in the right time range. Logs stored by
versions before this change are indexed by their message timestamp; purge them from
Settings to start clean.

Severity badges `err`/`crit` and `warning` use the colors of the `error` and `warning` tags.

Shortcuts: `/` focuses the search box, `Esc` clears it. Clicking a row shows all its fields.

## CSV export

The **Export** button (next to the log count) downloads every stored log matching the
current filters (search, time range, severity, host, app), newest first, up to `EXPORT_MAX`
rows (100,000 by default), not only the rows on screen. Two variants:

- **CSV**: comma separated, UTF-8.
- **CSV for Excel**: semicolon separated with a UTF-8 BOM, so French Excel opens it directly
  with accents. Cells starting with `=`, `+`, `-` or `@` are prefixed with `'` so that a
  crafted log message cannot run as a formula.

Columns: `received`, `message_time` (both as `YYYY-MM-DD HH:MM:SS.mmm` in the time zone
chosen in Settings), `severity`, `facility`, `host`, `host_ip`, `app`, `pid`, `source_ip`,
`proto`, `message`. In LogsQL mode, only the filter part is supported (no `| pipes`).

From the command line: `curl -o logs.csv 'localhost:8080/api/export.csv?q=error&range=24h&tz=Europe/Paris'`.

## Settings

The gear icon opens the settings, organized in tabs. Everything except color tags is
remembered per browser.

- **Localization**
  - *Language*: English or French.
  - *Date & time*: time zone (browser zone, UTC, or about 80 common zones) and the display
    format of the reception time: `DD/MM/YYYY HH:MM:SS` (default, the usual French display),
    with milliseconds, `YYYY-MM-DD`, 12-hour clock, ISO 8601 or Unix epoch. The time zone
    applies to every date shown.
- **Filters**: color tags. Each tag has a keyword, a background color (the text automatically
  switches to black or white to stay readable) and options: whole word, match case,
  regular expression, active. Tags are stored on the server in `/data/tags.json`
  (`logstream-data` volume), so they are shared by every browser. Default tags (pastel):
  `warning` (orange), `error` (red), `ok` (green). Default tags still using the colors of
  earlier versions are switched to the pastel ones automatically.
- **Interface**
  - *Theme*: System (follows the computer/phone preference), Light or Dark. The sun/moon
    button in the header switches between light and dark.
  - *Log display*: font size (tiny, small, medium, large) and font: the system monospace
    font, or one of 12 free fonts made for dense text (JetBrains Mono, Fira Code, Source
    Code Pro, IBM Plex Mono, Cascadia Code, Roboto Mono, Ubuntu Mono, Inconsolata, Red Hat
    Mono, Noto Sans Mono, Victor Mono, DM Mono). They are loaded by the browser from
    [Bunny Fonts](https://fonts.bunny.net), a privacy-friendly European font service; without
    internet access, the system font is used. Ligatures are disabled so `->` or `!=` show as typed.
- **Data**: "Delete all logs" permanently erases every stored log (you must type
  `PURGE` to confirm). Tags and settings are kept. VictoriaLogs needs `-delete.enable`
  (already set in `docker-compose.yml`); set `ALLOW_PURGE=false` to disable the feature.
  Anyone who can open the UI can purge: set `AUTH_USER` / `AUTH_PASS` if the UI is reachable
  by others.

## Host names (reverse DNS)

When a device sends its IP address as host name (or no host name at all), Logstream looks up
its DNS name (PTR record) and stores the name in `host` and the IP in `host_ip`. Results are
cached (1 hour, 10 minutes when there is no name). Logs stored earlier with an IP are resolved
on display, and the host filter shows `name (IP)`.

The container uses Docker's DNS, which forwards to the host's resolvers. If your local names
are only known by your router or a local DNS (Pi-hole, AdGuard, Unbound…), set
`DNS_SERVER=192.168.1.1` (its address). Set `RDNS=off` to disable lookups.
- **Color tags**: each tag has a keyword, a background color (the text automatically
  switches to black or white to stay readable) and options: whole word, match case,
  regular expression, active. Tags are stored in `/data/tags.json` (`logstream-data` volume).
  Default tags (pastel): `warning` (orange), `error` (red), `ok` (green). Default tags
  still using the colors of earlier versions are switched to the pastel ones automatically.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `SYSLOG_PORT` | `514` | syslog port published on the host |
| `HTTP_PORT` | `8080` | web UI port |
| `RETENTION` | `30d` | how long VictoriaLogs keeps logs |
| `AUTH_USER` / `AUTH_PASS` | empty | HTTP Basic authentication for the UI |
| `RDNS` | `on` | resolve IP hosts to DNS names |
| `DNS_SERVER` | empty | DNS server for reverse lookups (`ip` or `ip:port`) |
| `ALLOW_PURGE` | `true` | allow "Delete all logs" in Settings |
| `EXPORT_MAX` | `100000` | maximum number of rows in a CSV export |
| `TZ` | `Europe/Paris` | time zone for RFC 3164 timestamps (which carry none) |
| `BATCH_SIZE`, `FLUSH_MS`, `QUEUE_SIZE` | `1000`, `1000`, `100000` | ingestion tuning |

## Debugging

- `docker compose logs -f logstream`: receive errors and errors sending to VictoriaLogs.
- The bottom bar shows received / stored / dropped counters and the last storage error.
- <http://localhost:9428/select/vmui>: VictoriaLogs' own UI to try LogsQL queries.
- API:
  ```bash
  curl 'localhost:8080/api/logs?q=error&range=1h&limit=5'   # the response includes the generated LogsQL query
  curl localhost:8080/api/stats
  curl localhost:8080/api/tags
  ```
- Running outside Docker (Go 1.22+): `VLOGS_URL=http://localhost:9428 DATA_DIR=./data SYSLOG_ADDR=:5514 go run .`

## Code layout

| File | Contents |
|---|---|
| `main.go` | configuration, startup, authentication |
| `syslog.go` | UDP/TCP listeners and RFC 3164 / 5424 parsing |
| `store.go` | batched inserts into VictoriaLogs and LogsQL queries |
| `query.go` | turns UI filters into LogsQL; live-view filter |
| `hub.go` | pushes new messages to browsers (SSE) |
| `rdns.go` | cached reverse DNS lookups |
| `export.go` | streamed CSV export |
| `tags.go` | color tag storage |
| `api.go` | `/api/*` HTTP routes |
| `web/` | UI (HTML, CSS, plain JavaScript, no build step), embedded in the binary; translations live in `web/app.js` (`I18N`) |

## Note

With Docker Desktop (macOS/Windows), the source IP seen by the container for UDP packets is
the Docker gateway. The `host` field still comes from the syslog header, which normally
carries the sender's real name.
