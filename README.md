English | [Français](README.fr.md)

# Logstream

A simple syslog sink: receives logs over UDP/TCP on port 514, stores them in
[VictoriaLogs](https://docs.victoriametrics.com/victorialogs/), and serves a clean web
interface (light/dark theme, live search, live view, color tags, English/French UI).

```
devices ──514 udp/tcp──▶ logstream (Go) ──HTTP batches──▶ VictoriaLogs
                              ▲    └── SSE (live) ──▶ browser
                              └──── API / UI ◀─────────┘
```

## Architecture

![Schéma logique de Logstream](docs/architecture.png)

- **Ingestion**: syslog (UDP/TCP) and Docker container logs both go through `sink()` (reverse DNS
  on IP hosts, without holding up the listeners), then the `Store` queue, which sends them in
  batches to VictoriaLogs. When VictoriaLogs is unreachable, batches are kept on disk
  (`/data/spool`, up to `SPOOL_MAX_MB`) and sent again, oldest first, once it is back.
- **Live view**: `sink()` also publishes each message to the `Hub`, which streams it to the
  browsers over SSE.
- **Search**: the HTTP API turns the UI filters into LogsQL queries sent to VictoriaLogs.
- **Timeline**: `/api/histogram` (`histogram.go`) counts the logs per interval and severity,
  aligned on the local time; the zoom bounds (`from`/`to`) also apply to the list and the export.
- **State**: tags and source settings live in `/data` (`logstream-data` volume); the logs
  themselves in the `vlogs-data` volume.

The editable source of the diagram is
[`docs/architecture.excalidraw`](docs/architecture.excalidraw)
(open it on [excalidraw.com](https://excalidraw.com)).

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

**Settings > Log sources > Syslog** turns syslog reception on or off and chooses the protocols
(UDP, TCP) without restarting; the choice is saved in `/data/syslog.json`. It also shows the
listening state (and the error if the port is already in use). The port itself is published
by docker-compose: change `SYSLOG_PORT` in `.env`, then run `docker compose up -d`.

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
found in the message itself (`msg_time`), severity, host, app, codes of the tags found and message. Click a host or an
app to filter on it.

Logs are indexed, searched and sorted by **reception time**: devices with a wrong clock
(e.g. access points whose NTP fails) still show up in the right time range. Logs stored by
versions before this change are indexed by their message timestamp; purge them from
Settings to start clean.

Severity badges `err`/`crit` and `warning` use the colors of the `error` and `warning` tags.

Shortcuts: `/` focuses the search box, `Esc` clears it. Clicking a row shows all its fields.

The column headers stay visible while scrolling. Drag the edge of a header (received, message
time, severity, host, app) to resize the column, double-click it to go back to the automatic
width; the message takes the remaining space. Widths are remembered by the browser
(**Settings > Appearance > Reset column widths** restores them all). On phones the list keeps its
two-line layout without columns.

## Stream and Time range modes

The first control of the filter bar switches between two display modes:

- **Stream**: the latest logs over a sliding duration (5 min to 30 days, or all), with the live
  view.
- **Time range**: the logs between a start and an end date, typed in the time zone chosen in
  Settings. ◀ and ▶ move to the previous or next range of the same length, the magnifier doubles
  it around its middle. The live view pauses; the mode and the range are kept across reloads.

## Timeline

The timeline above the list shows the volume of logs per interval, counted by **reception
time** on the server clock (so it matches the times shown in the rows).

- Hover an interval: its bounds, total and detail per severity.
- Click a bar to show that interval in Time range mode, or drag across several bars to show the
  selection; "× Back to stream" returns to Stream mode. The list, the counters and the CSV export
  follow the range.
- In live mode the last interval grows as messages arrive, and the timeline reloads at each new
  interval. Nothing is refreshed while the browser tab is hidden; it catches up when shown again.

**Settings > Appearance > Timeline** (remembered per browser): scale (linear, √ by default, log),
height (S/M/L: 40/80/120 px), color (stacked by severity, intensity compared with the median of
the window: calm, burst above 3×, anomaly above 10×, or none), bars or area, division
(automatic, about 100 intervals, or fixed: 1 s, 10 s, 1 min, 5 min, 1 h, 1 day) and refresh
(off, 5 s, 15 s, 30 s, 1 min, or at each new interval). A fixed division that would exceed
300 intervals over the range is enlarged (shown as "enlarged"). Intervals are aligned on the
local time of the time zone chosen in Settings (days start at local midnight).

API: `curl 'localhost:8080/api/histogram?range=24h&step=auto&tz=Europe/Paris'` returns
`step` (ms), `start` (ms), `count`, `now` (server clock) and the non-empty `buckets`
(`i` = interval index, `n` = total, `sev` = count per severity). Every log endpoint also accepts
`from` / `to` (Unix milliseconds) instead of `range`.

## Docker container logs

Logstream also collects the logs of the Docker containers running on the machine where it is
installed (`DOCKER_LOGS=on`, the default in `docker-compose.yml`). They are searched, filtered,
colored and exported like syslog messages:

- **host** is the Docker host name, **app** the compose service (or the container name), and
  each log also carries `container`, `container_id`, `image`, `compose_project`,
  `compose_service` and `stream` (stdout/stderr), visible in the row details.
- The **Source** filter shows only syslog or only Docker logs; Docker rows have a small cube
  before the app name, colored like its compose project.
- The severity comes from the line itself when the application writes it: JSON
  (`"level":"error"`), logfmt (`level=warn`), `[ERROR]`, or an upper-case level word at the
  start of the line (`ERROR`, `WARN`…). Otherwise it is `info`. Terminal color codes are removed.
- **Settings > Log sources** shows one label per container (`project/service`) in a single field:
  followed containers in color, then not followed ones in grey; a click switches a label.
  The color identifies the compose project, and Docker app names in the log list use the
  same color. Stopped containers are hidden by default ("Show stopped containers" displays
  them, dashed, and they stay editable). A filter box and "Enable all" / "Disable all" (applied
  to the labels shown) help with many containers. New containers are followed automatically
  unless that option is turned off. Choices are saved per compose service (or container name)
  in `/data/docker.json`, so they survive re-creations.
- Logstream remembers the position of the last line stored for each container
  (`/data/docker-state.json`): after a restart it resumes without losing lines. The position
  only moves once a line is in VictoriaLogs or in the disk buffer, and a full queue slows the
  reading down instead of dropping lines. A container seen for the first time
  is read from `DOCKER_BACKFILL` ago (1 hour by default).
- Logstream itself and the proxy below are never collected; add the label
  `logstream.exclude=true` to any other container to exclude it for good.

**Security**: access to the Docker socket is equivalent to root on the machine. Logstream
therefore goes through [docker-socket-proxy](https://github.com/Tecnativa/docker-socket-proxy),
which only lets through listing containers, reading logs, events and engine info (`GET` only).

## Host system logs

Logstream can also collect the system logs of the machine hosting the stack, without
configuring anything on the host. The source is **off by default**: turn it on in
**Settings > Log sources > Host system logs** (saved in `/data/hostlogs.json`).

- `docker-compose.yml` mounts `/var/log` and `/run/log/journal` read-only under `/host`.
- When the host runs systemd, Logstream reads the **systemd journal** files directly (no
  `journalctl` needed in the image): **host** is the machine name, **app** the program
  (`SYSLOG_IDENTIFIER`), severity and facility come from the journal, and the systemd unit is
  kept in `unit`. Otherwise it follows the text files of `/var/log` (`syslog`, `messages`,
  `*.log`), parsed like syslog lines, with the file name in `log_file`.
- These logs have the `host` source: the **Source** filter shows them alone.
- When the source is turned on, the last hour is read first (`HOST_LOGS_BACKFILL`); the
  position reached is saved in `/data/hostlogs-state.json`, so a restart neither loses nor
  duplicates entries.
- **Permissions**: the container runs as an unprivileged user and gets the `adm` group
  (gid 4), which can read the journal and `/var/log` on Debian and Ubuntu. On other systems,
  set `HOST_LOGS_GID` in `.env` to the gid of `systemd-journal`
  (`getent group systemd-journal | cut -d: -f3`). Settings > Log sources shows a clear message when
  access is denied.
- Limits: journal fields compressed by journald (messages longer than 512 bytes, compressed
  with zstd/lz4/xz) cannot be decoded without extra libraries; they are counted in Settings and
  shown as "(compressed journal entry)". Rotated or compressed text files (`*.1`, `*.gz`) are
  not read.

## Kernel messages (dmesg)

Logstream can collect the kernel ring buffer of the host (what `dmesg` shows: hardware,
drivers, OOM killer, firewall…) by reading `/dev/kmsg`. The source is **off by default**: turn it
on in **Settings > Log sources > Kernel messages (dmesg)** (saved in `/data/dmesg.json`).

- **Device**: the container does not see `/dev/kmsg` by default. Uncomment the `devices`
  section of `docker-compose.yml` (`/dev/kmsg:/dev/kmsg:r`, read-only), then
  `docker compose up -d`.
- **Permissions**: when the host has `kernel.dmesg_restrict=1` (the default on Debian and
  Ubuntu), only processes with the `CAP_SYSLOG` capability can read the kernel log, and the
  container runs as an unprivileged user. Either:
  - allow it on the host: `sysctl -w kernel.dmesg_restrict=0`, kept across reboots with
    `echo kernel.dmesg_restrict=0 | sudo tee /etc/sysctl.d/50-dmesg.conf` (every user of the
    host can then run `dmesg`); this setting cannot be changed from a container;
  - or run Logstream as root with the `SYSLOG` capability: uncomment `user: "0:0"` and
    `cap_add: [SYSLOG]` in `docker-compose.yml`.

  Settings > Log sources says when the device is missing or access is denied; the rest of
  Logstream keeps working and the device is tried again every 10 seconds.
- **Fields**: **app** is `kernel` (or the program name for messages written by programs such
  as `systemd[1]: …`), severity and facility come from the record, **host** is `localhost`,
  `subsystem` and `device` are kept when the kernel gives them. These logs have the `kernel`
  source (`proto` `kmsg`): the **Source** filter shows them alone.
- **Time**: records carry the time since boot; Logstream turns it into a date using the boot
  time (`/proc/uptime`). As with `dmesg -T`, the time can be early after the host was suspended.
- When the source is turned on, the last hour is read first (`DMESG_BACKFILL`, `0` = only new
  messages). The sequence number of the last message and the boot ID are saved in
  `/data/dmesg-state.json`: after a restart of Logstream, reading resumes after the last
  message (no duplicates); after a reboot of the host, the new buffer is read from
  `DMESG_BACKFILL` ago. Messages overwritten in the kernel buffer before being read are counted
  in Settings.

## Linux audit (auditd)

Logstream can follow the log of the Linux audit daemon (`/var/log/audit/audit.log`): logins,
`sudo`, watched files, syscalls matched by your `auditctl` rules… The source is **off by
default**: turn it on in **Settings > Log sources > Linux audit (auditd)**, where the file path can
also be changed (saved in `/data/audit.json`; `AUDIT_LOG` sets the default path).

- **Mount**: uncomment `/var/log/audit:/var/log/audit:ro` in `docker-compose.yml`, then
  `docker compose up -d`. (`/var/log` is already mounted under `/host` for the host system
  logs, so `/host/var/log/audit/audit.log` also works as the path, without a new volume.)
- **Permissions** (host side): auditd writes its log as `root`, mode `0600`, in a `0700`
  directory, and Logstream runs as UID `10001`. The reliable way is the group the container
  already gets for the host logs (`HOST_LOGS_GID`, `adm` by default):
  ```bash
  sudo sed -i 's/^log_group.*/log_group = adm/' /etc/audit/auditd.conf
  sudo chgrp adm /var/log/audit && sudo chmod 750 /var/log/audit
  sudo service auditd restart        # systemctl refuses to restart auditd on some distributions
  ```
  auditd then creates every rotated file with mode `0640` and group `adm`. An ACL also works
  (`sudo setfacl -m u:10001:rx /var/log/audit && sudo setfacl -m u:10001:r /var/log/audit/audit.log`)
  but must be applied again to each new file after a rotation. Settings > Log sources says when the
  file is missing or access is denied; it is tried again every second.
- **Events**: the records of one event (`SYSCALL`, `EXECVE`, `CWD`, `PATH`, `PROCTITLE`…) share
  the same `msg=audit(time:serial)` and become **one** log, once no record came for 2 seconds.
  The message is a summary — who, what, result — such as
  `cedric (as root): exec apt install nginx [exec] → success` or
  `bob: PAM:authentication via /usr/sbin/sshd from 203.0.113.9 on ssh → failed`; the raw
  records are kept in `audit_raw` and shown under the summary (folded in compact density until
  the line is opened). Hexadecimal values (`proctitle`, `cmd`, arguments with spaces) are
  decoded.
- **Fields**: `audit_type` (type of the first record, also used as **app**), `audit_serial`,
  `auid` (login user ID), `user` (its name, or the uid when it cannot be resolved), `exe`, `res`
  (`success` / `failed`, from `res=` or `success=`) and `key` (rule key). Severity is `warning`
  when the result is a failure, `info` otherwise; facility `authpriv`; source `audit`. In the
  list, audit lines have a violet left edge, a light tint and an **AUDIT** badge before the app;
  the **Source** filter shows them alone.
- **Names**: `user` comes from the enriched format (`log_format = ENRICHED`, the default of
  recent auditd versions), else from the host's `/etc/passwd` when it is mounted as
  `/host/etc/passwd`. **host** is the `node=` of the records (`name_format = hostname` in
  `auditd.conf`), else `AUDIT_HOST`, else `/host/etc/hostname`, else `localhost`.
- When the source is turned on, the last hour is read first (`AUDIT_BACKFILL`, `0` = only new
  events). The inode and an offset before which every event was sent are saved in
  `/data/audit-state.json`: after a restart, reading resumes there; after a rotation by auditd,
  the end of `audit.log.1` is read before the new file.

## Log files (Traefik, nginx, applications…)

Logstream can follow the log files of other programs: Traefik, nginx, Apache, or any application
that writes to a file. Declare each file in **Settings > Log sources > Log files** (saved in
`/data/files.json`, up to 50): a **name** (the **app** of its entries, by default the file name),
the **path** of the file as mounted in the container, a **format** (*Automatic* by default), an
optional **host** (default: `/host/etc/hostname`, else `localhost`) and a switch.

- **Mount** the directory read-only, e.g. in `docker-compose.yml`:
  `- /var/log/traefik:/logs/traefik:ro`, then `docker compose up -d`. Files under `/var/log`
  are already reachable as `/host/var/log/…`. The files must be readable by UID `10001` or the
  group of `HOST_LOGS_GID` (`adm` by default); Settings > Log sources says when no file matches
  or access is denied.
- **Path**: one file, e.g. `/logs/traefik/access.log` (no wildcards). As soon as it is typed, the
  file is checked: found or not, readable or not, its detected format and its last line.
- **Format detection** (*Automatic*): from the last lines of the file (50 at most), the
  structured format that at least 80 % of them follow — Traefik access JSON, JSON lines, access
  log CLF, logfmt — else plain text. It is done again when the format is changed; the format
  found is shown in the status of the file. Choose another format to force it (a regular
  expression is never detected).
- **Formats**:
  - *Plain text*: one entry per line; the level is detected as for Docker logs, a timestamp at
    the start of the line (ISO 8601, `2006/01/02 15:04:05`, `[02/Jan/2006:15:04:05 +0200]`,
    `Jan  2 15:04:05`) becomes the message time and is removed from the message. Indented
    lines (stack traces) are added to the previous entry.
  - *JSON lines*: `msg` / `message`, `time` / `ts` / `@timestamp` (text or Unix time) and
    `level` (word or number, pino/bunyan included) are recognized; the other keys become
    fields, nested objects with dotted names (`req.id`).
  - *logfmt*: `key=value` pairs, as in Traefik's default logs or Go programs; same keys as JSON.
  - *Access log (CLF)*: Common / Combined Log Format of nginx and Apache, and Traefik's access
    log in this format (router, server URL and duration included). Fields `client_ip`,
    `http_method`, `http_path`, `http_status`, `http_size`, `user_agent`, `http_referer`,
    `router`, `backend`, `duration_ms`.
  - *Traefik access (JSON)*: the JSON access log of Traefik (`accessLog.format: json`),
    summarized as `GET app.example.com/ 200 · 12.5 ms · app@docker · 203.0.113.9`, every key in
    the fields. Traefik's own logs in the same file are read as JSON lines.
  - *Regular expression* (RE2 / Go syntax): the named groups `time`, `level`, `msg`, `host`
    and `app` fill these fields, `status` gives the level of an access log, the other groups
    become fields, e.g. `^(?P<time>\S+ \S+) \[(?P<level>\w+)\] (?P<module>\S+): (?P<msg>.*)$`.
    A line that does not match is added to the previous entry.

  For access logs, 5xx statuses are `err`, 4xx `warning`, the others `info`. A line that does
  not fit the chosen format is kept as plain text. Fields named like LogStream's own ones
  (`host`, `app`…) are renamed `field_host`, `field_app`…
- **Display**: source `file`, the path in `log_file`. In the list these lines have a green left
  edge, a light tint and a **FILE** badge before the app; the **Source** filter shows them alone.
- When a file is turned on, the lines of the last hour already in it are read first
  (`FILES_BACKFILL`, `0` = only new lines; lines without a timestamp are then left out). The
  start of that hour is found by bisection on the dates of the lines, so a large file that holds
  months of logs is not read from its start; a file created later is read from its start, up to
  64 MB per second, the status showing what is left to read. The inode and an offset are saved per file in
  `/data/files-state.json`: after a restart reading resumes there; after a rotation
  (`access.log` renamed `access.log.1`), the end of the old file is read before the new one;
  a file truncated in place (`copytruncate`) is read again from its start.

## LogStream internal logs

Logstream records its own events as logs, with **host** `logstream`, **app** the component
(`store`, `syslog`, `docker`, `host`, `dmesg`, `audit`, `files`, `main`) and the `logstream` source:
VictoriaLogs unreachable and back, lost batches, batches refused from the disk buffer, syslog
listener errors, Docker API or container streams lost, sources that cannot read, start and
stop. They are on by default from level **warning**; **Settings > Log sources > LogStream internal
logs** turns them off or changes the minimum level (saved in `/data/selflog.json`). The same
event (component and message) is recorded at most once every 30 seconds, with the number of
repeats left out; events go through a bounded queue and never wait for the storage, so an
unreachable VictoriaLogs cannot make them pile up or loop. Everything still goes to
`docker compose logs logstream` as before. In the list, these lines have a turquoise left edge
and an **LS** badge before the app; the **Source** filter shows them alone.

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

The gear icon opens the settings, organized in tabs. Changes show at once but are only kept
with the **Save** button, under the tab labels: it takes the accent color while changes are
waiting. **Cancel** goes back to the saved settings, and closing the window with unsaved changes
asks before dropping them. Localization and Appearance are remembered per browser; color tags
and log sources are saved on the server. Purging and resetting column widths act at once.

- **Localization**
  - *Language*: English or French.
  - *Date & time*: time zone (browser zone, UTC, or about 80 common zones) and the display
    format of the reception time: `DD/MM/YYYY HH:MM:SS` (default, the usual French display),
    with milliseconds, `YYYY-MM-DD`, 12-hour clock, ISO 8601 or Unix epoch. The time zone
    applies to every date shown.
- **Patterns**: color tags. Each tag has a keyword, a background color (the text automatically
  switches to black or white to stay readable) and options: whole word, match case,
  regular expression, active. Tags are stored on the server in `/data/tags.json`
  (`logstream-data` volume), so they are shared by every browser. Default tags (pastel):
  `warning` (orange) and `error` (red); `ok` (green) is in the *Log levels* preset. Default
  tags still using the colors of earlier versions are switched to the pastel ones
  automatically.
  The *+ Preset…* menu adds ready-made tags: HTTP/HTTPS access logs (status codes, methods,
  probes, bots, TLS and proxy errors), system logs (SSH, sudo, kernel, systemd, firewall),
  applications (Docker, databases) and general patterns (log levels, IPv4 addresses). Tags
  already in the list are skipped, and the added tags can be edited like any other. The list
  comes from a text file you can edit (`PRESETS_FILE`): see [docs/presets.md](docs/presets.md)
  for each preset and the file format. In a regular expression, a group named `hl`
  (`(?<hl>…)`) colors only that part of the match: the presets use it to color the status code
  or the method, not the text around it.
  Each tag gets a two-digit code (`01`, `02`…) assigned by the server: it stays with the tag
  until the tag is deleted (codes are also given to tags created by earlier versions). The
  *Pattern* column of the log list shows, as grey badges, the codes of the active tags found in
  each message; it has room for 3, beyond that it shows 2 and `+N`, and the tooltip lists them
  all.
- **Appearance**
  - *Theme*: System (follows the computer/phone preference), Light or Dark. The sun/moon
    button in the header switches between light and dark.
  - *Log display*: font size (tiny, small, medium, large), density (normal, or compact to
    fit about 50% more lines on screen) and font: the system monospace font, one of 3 narrow
    built-in fonts served by LogStream itself, which work offline (Inconsolata Condensed, the
    narrowest, Iosevka and Ubuntu Mono), or one of 11 free fonts made for dense text (JetBrains
    Mono, Fira Code, Source Code Pro, IBM Plex Mono, Cascadia Code, Roboto Mono, Inconsolata,
    Red Hat Mono, Noto Sans Mono, Victor Mono, DM Mono). These 11 are loaded by the browser from
    [Bunny Fonts](https://fonts.bunny.net), a privacy-friendly European font service; without
    internet access, the system font is used. Ligatures are disabled so `->` or `!=` show as
    typed. The densest setting is Tiny + Compact + Inconsolata Condensed.
- **Database**: database figures read from VictoriaLogs (refreshed at most every 30 s): stored
  lines, size on disk (index included), raw size and compression ratio, period covered with
  the retention, lines of the last 24 h and last hour, distinct hosts and apps, free disk
  space. Sizes use KB/MB/GB (Ko/Mo/Go in French). "Delete all logs" permanently erases every stored log (you must type
  `PURGE` to confirm). Tags and settings are kept. VictoriaLogs needs `-delete.enable`
  (already set in `docker-compose.yml`). The feature is off by default: set `ALLOW_PURGE=true`
  to allow it. Any admin can then purge: turn on [authentication](#authentication) if the UI
  is reachable by others.

## Authentication

`AUTH_MODE` picks how the UI and the API are protected (`/healthz` always stays open):

- **`local`** (default): a login page with the account `AUTH_USER` / `AUTH_PASS`; leave them
  empty to have no authentication (for instance behind a reverse proxy that already checks). The
  UI then shows a warning banner, which can be closed.
- **`oidc`**: login through an OpenID Connect provider (Keycloak, Authentik, Authelia, Zitadel…),
  authorization code flow with PKCE.

In `local` mode the login page follows the theme and language of the UI. The session lasts
`SESSION_TTL` (12 h by default), survives restarts (its signing key is in `/data/session.key`) and
ends when `AUTH_USER` or `AUTH_PASS` changes; the log out button (top right) ends it. Failed logins
are written in the logs with the client address (`auth: failed login for "bob" from 192.0.2.7`).
Scripts can still call the API with HTTP Basic credentials (`curl -u user:pass`).

An optional **read-only account**, `AUTH_VIEWER_USER` / `AUTH_VIEWER_PASS`, can search, follow
the live view and export, but cannot change tags, sources or purge: those settings are greyed
out in its UI and the API answers `403`.

To show your logo on the login page, mount a PNG in the container and point `LOGIN_LOGO` to it:

```yaml
# docker-compose.yml, logstream service
    volumes:
      - ./logo.png:/config/logo.png:ro
```
```bash
# .env
LOGIN_LOGO=/config/logo.png
```

To use OIDC:

1. In the provider, create a **confidential** client (with a secret) for logstream and register
   the redirect URL `https://logs.example.org/auth/callback` (your address).
2. In `.env`:
   ```bash
   AUTH_MODE=oidc
   OIDC_ISSUER=https://sso.example.org/realms/home      # exactly the "issuer" of the provider
   OIDC_CLIENT_ID=logstream
   OIDC_CLIENT_SECRET=...
   OIDC_REDIRECT_URL=https://logs.example.org/auth/callback
   ```
3. `docker compose up -d`. The logs show `oidc authentication enabled`, or the reason the
   provider could not be read (wrong issuer, unreachable…).

Opening the UI sends you to the provider's login page, then back to logstream. The session
lasts `SESSION_TTL` (12 h by default) and survives restarts (its signing key is in
`/data/session.key`); when it ends, the page goes through the login again. The log out button
(top right) ends the logstream session, then opens the provider's log out page if it has one.

Every user the provider accepts for this client can log in: restrict access in the provider
(Keycloak: client roles or a dedicated realm; Authentik: application bindings). Logins are written
in the logstream logs (`oidc: alice logged in`). With an `https` redirect URL, the cookies are
only sent over HTTPS: logstream must be reached through a TLS reverse proxy.

To give read-only access to some users, set `OIDC_ADMIN_GROUP` (for instance
`logstream-admins`): only its members are admins, the others are read-only. The groups are read
from the `groups` claim of the ID token (`OIDC_GROUPS_CLAIM` to use another one); in Keycloak,
add a "Group Membership" mapper to the client (a leading `/` is ignored).

Whatever the mode, every answer carries security headers (Content-Security-Policy,
X-Frame-Options…), and the API refuses changes sent from another site (cross-site requests).

## Host names (reverse DNS)

When a device sends its IP address as host name (or no host name at all), Logstream looks up
its DNS name (PTR record) and stores the name in `host` and the IP in `host_ip`. Results are
cached (1 hour, 10 minutes when there is no name). Logs stored earlier with an IP are resolved
on display, and the host filter shows `name (IP)`.

The container uses Docker's DNS, which forwards to the host's resolvers. If your local names
are only known by your router or a local DNS (Pi-hole, AdGuard, Unbound…), set
`DNS_SERVER=192.168.1.1` (its address). Set `RDNS=off` to disable lookups.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `SYSLOG_PORT` | `514` | syslog port published on the host |
| `HTTP_PORT` | `8080` | web UI port |
| `RETENTION` | `30d` | how long VictoriaLogs keeps logs |
| `AUTH_MODE` | `local` | `local` (login page) or `oidc`, see [Authentication](#authentication) |
| `AUTH_USER` / `AUTH_PASS` | empty | account of the login page (`local` mode); empty = no authentication |
| `AUTH_VIEWER_USER` / `AUTH_VIEWER_PASS` | empty | optional read-only account (`local` mode) |
| `LOGIN_LOGO` | empty | PNG shown on the login page, path inside the container (`local` mode) |
| `SESSION_TTL` | `12h` | session lifetime (both modes; `OIDC_SESSION_TTL` still works) |
| `OIDC_ISSUER` | empty | issuer URL of the OpenID Connect provider (`oidc` mode) |
| `OIDC_CLIENT_ID` / `OIDC_CLIENT_SECRET` | empty | client registered in the provider |
| `OIDC_REDIRECT_URL` | empty | callback URL of logstream, e.g. `https://logs.example.org/auth/callback` |
| `OIDC_SCOPES` | `openid profile email` | requested scopes |
| `OIDC_ADMIN_GROUP` | empty | only members of this group are admins, the others read-only (empty = everyone is admin) |
| `OIDC_GROUPS_CLAIM` | `groups` | ID token claim that lists the groups |
| `RDNS` | `on` | resolve IP hosts to DNS names |
| `DNS_SERVER` | empty | DNS server for reverse lookups (`ip` or `ip:port`) |
| `ALLOW_PURGE` | `false` | allow "Delete all logs" in Settings |
| `SYSLOG_TCP_MAX_CONNS` | `512` | syslog TCP connections open at once; more are refused |
| `SYSLOG_TCP_IDLE` | `30m` | a syslog TCP connection silent this long is closed (senders reconnect) |
| `EXPORT_MAX` | `100000` | maximum number of rows in a CSV export |
| `PRESETS_FILE` | `/data/presets.json` | color tag presets file; the built-in list when missing (see [docs/presets.md](docs/presets.md)) |
| `DOCKER_LOGS` | `on` in compose | collect the logs of the local Docker containers |
| `DOCKER_HOST` | `tcp://docker-proxy:2375` in compose | Docker API address (`unix:///var/run/docker.sock` outside compose) |
| `DOCKER_BACKFILL` | `1h` | history read from a container seen for the first time |
| `HOST_LOGS_GID` | `4` (adm) in compose | group given to the container to read the host logs |
| `HOST_LOGS_BACKFILL` | `1h` | history read when the host system logs source is turned on |
| `HOST_LOGS_ROOT` | `/host` | where the host directories are mounted |
| `DMESG_BACKFILL` | `1h` | history read when the kernel messages source is turned on |
| `DMESG_DEVICE` | `/dev/kmsg` | kernel log device read by the kernel messages source |
| `AUDIT_LOG` | `/var/log/audit/audit.log` | default path of the Linux audit log (can be changed in Settings > Log sources) |
| `AUDIT_BACKFILL` | `1h` | history read when the Linux audit source is turned on |
| `AUDIT_HOST` | empty | host name of the audit events when the records carry no `node=` |
| `FILES_BACKFILL` | `1h` | history read when a log file is turned on (Settings > Log sources > Log files) |
| `TZ` | `Europe/Paris` | time zone for RFC 3164 timestamps (which carry none) |
| `BATCH_SIZE`, `FLUSH_MS`, `QUEUE_SIZE` | `1000`, `1000`, `100000` | ingestion tuning |
| `SPOOL_MAX_MB` | `1024` | disk buffer size for batches VictoriaLogs could not take (`0` = no buffer: retried for 15 s, then dropped) |

## Debugging

- `docker compose logs -f logstream`: receive errors and errors sending to VictoriaLogs.
- The bottom bar shows received / stored / dropped counters, the messages waiting in the disk
  buffer, and the last storage error.
- <http://localhost:9428/select/vmui>: VictoriaLogs' own UI to try LogsQL queries.
- API:
  ```bash
  curl 'localhost:8080/api/logs?q=error&range=1h&limit=5'   # the response includes the generated LogsQL query
  curl localhost:8080/api/stats
  curl localhost:8080/api/tags
  ```
- Running outside Docker (Go 1.22+): `VLOGS_URL=http://localhost:9428 DATA_DIR=./data SYSLOG_ADDR=:5514 go run .`

## Updating

Every image version is pinned, so a `docker compose pull` or a rebuild never changes a
component behind your back:

| Where | Image | Version |
|---|---|---|
| `docker-compose.yml` | `victoriametrics/victoria-logs` | `v1.52.0` |
| `docker-compose.yml` | `tecnativa/docker-socket-proxy` | `v0.5.0` |
| `Dockerfile` (build) | `golang` | `1.27.1-alpine3.24` |
| `Dockerfile` (runtime) | `alpine` | `3.24.2` |

To update one of them:

1. Check the release notes: [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/changelog/),
   [docker-socket-proxy](https://github.com/Tecnativa/docker-socket-proxy/releases),
   [Go](https://go.dev/doc/devel/release), [Alpine](https://alpinelinux.org/releases/).
   VictoriaLogs keeps its storage format across minor versions; read the changelog before a
   major version change.
2. Change the version in the file above, then run `docker compose up -d --build`.
3. Check the bottom bar (received / stored / dropped) and **Settings > Log sources**. To go back,
   restore the previous version and run the same command.

## Code layout

| File | Contents |
|---|---|
| `main.go` | configuration, startup |
| `auth.go` | authentication: OpenID Connect (discovery, PKCE, ID token checks) and the signed session cookie |
| `auth_local.go` | `local` mode: login page (`web/login.html`), session cookie, `LOGIN_LOGO` |
| `syslog.go` | UDP/TCP listeners and RFC 3164 / 5424 parsing |
| `store.go` | batched inserts into VictoriaLogs and LogsQL queries |
| `query.go` | turns UI filters into LogsQL; live-view filter |
| `histogram.go` | timeline: division, interval alignment, `/api/histogram` |
| `hub.go` | pushes new messages to browsers (SSE) |
| `rdns.go` | cached reverse DNS lookups |
| `export.go` | streamed CSV export |
| `docker.go` | Docker container logs (API, followers, positions, level detection) |
| `syslogserver.go` | syslog listeners opened and closed from Settings > Log sources |
| `hostlogs.go`, `journal.go` | host system logs: systemd journal reader (no `journalctl`) and `/var/log` follower |
| `dmesg.go` | kernel messages: `/dev/kmsg` reader, record parsing, position by boot ID and sequence number |
| `audit.go` | Linux audit: `audit.log` follower (rotation, saved offset), record parsing, grouping by event, summary |
| `filesource.go`, `fileformats.go` | log files: follower (rotation, saved offsets, continuation lines) and formats (detection, text, JSON, logfmt, CLF, Traefik, regular expression) |
| `selflog.go` | LogStream internal logs: `logEvent`, minimum level, throttling, bounded queue |
| `tags.go` | color tag storage |
| `presets.go`, `presets.json` | color tag presets (`/api/presets`), built-in list |
| `api.go` | `/api/*` HTTP routes |
| `web/` | UI (HTML, CSS, plain JavaScript, no build step), embedded in the binary; translations live in `web/app.js` (`I18N`), and in `web/login.html` for the login page |

## Note

With Docker Desktop (macOS/Windows), the source IP seen by the container for UDP packets is
the Docker gateway. The `host` field still comes from the syslog header, which normally
carries the sender's real name.
