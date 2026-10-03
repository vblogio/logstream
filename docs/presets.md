**English** · [Français](presets.fr.md)

# Color tag presets

In **Settings › Filters**, the **+ Preset…** menu adds a group of ready-made color tags in one
click. Added tags are ordinary tags: you can change their color, pattern or options, or delete
them. A tag whose pattern is already in the list is not added twice.

The list comes from a text file, [`presets.json`](../presets.json), built into LogStream. You
can replace it with your own file (see [Using your own file](#using-your-own-file)).

## Built-in presets

### HTTP/HTTPS

These presets read access logs from nginx and Apache (common and combined formats), Traefik
(CLF and JSON), Caddy (JSON) and HAProxy (`option httplog`).

| Preset | Tags | What gets colored |
| --- | --- | --- |
| HTTP status codes | `HTTP 2xx` green, `HTTP 3xx` blue, `HTTP 4xx` orange, `HTTP 5xx` red | only the status code, e.g. `404` in `"GET /x HTTP/1.1" 404 153`, `"status":404` or `"DownstreamStatus":404`. Other numbers on the line (size, path) are left alone. |
| HTTP methods | `GET/HEAD/OPTIONS` grey, `POST/PUT/PATCH` purple, `DELETE` pink | only the method in `"GET /path` or `"method":"GET"` (upper case only) |
| Probes and attacks | `probes / attacks` | `wp-login.php`, `xmlrpc.php`, `wp-admin`, `phpmyadmin`, `/.env`, `/.git`, `/.aws`, `/cgi-bin/`, `../`, `%2e%2e`, `/etc/passwd`, `<script`, `union select` |
| Bots and scripts | `bots / scripts` | words ending in `bot` (`Googlebot`, `bingbot`…), `crawler`, `spider`, `curl`, `wget`, `python-requests`, `Go-http-client`, `zgrab`, `masscan`, `nmap`, `sqlmap`, `nikto` |
| TLS/HTTPS and proxy errors | `TLS errors`, `proxy errors` | TLS handshake failures, expired or rejected certificates, `x509:`; `upstream timed out`, `no live upstreams`, `connect() failed`, `connection refused`, `bad gateway`, `gateway timeout`, `service unavailable` |

### System

| Preset | Tags | What gets colored |
| --- | --- | --- |
| SSH and logins | `login failures` red, `logins` green | sshd/PAM: `Failed password`, `Invalid user`, `authentication failure`, `incorrect password attempts`, `NOT in sudoers`…; `Accepted publickey`, `session opened for user`, `New session … of user` |
| sudo commands | `sudo commands` | the command run, e.g. `COMMAND=/usr/bin/apt` |
| Kernel: OOM, crashes, disks | `out of memory`, `kernel errors` | `Out of memory`, `oom-killer`, `Killed process 4242`; `Kernel panic`, `BUG:`, `Oops`, `Call Trace`, `segfault at`, `I/O error`, `EXT4-fs error`, `blocked for more than 120 seconds`, `soft lockup` |
| systemd services | `failed services` red, `service start/stop` green | `Failed to start`, `Failed with result`, `Main process exited, code=killed`, `Start request repeated too quickly`; `Started`, `Stopping`, `Reloaded`, `Reached target` |
| Firewall and fail2ban | `firewall` | `[UFW BLOCK]`, `[UFW ALLOW]`, `DROP`, `REJECT`, `Ban 203.0.113.9`, `Unban …`, `Found …` |

### Applications

| Preset | Tags | What gets colored |
| --- | --- | --- |
| Docker and containers | `container problems` | `exited with code 137` (non-zero codes only), `OOMKilled`, `unhealthy`, `Back-off restarting`, `CrashLoopBackOff`, `container die/kill/oom` |
| Databases | `database errors` | PostgreSQL and MySQL/MariaDB: `deadlock detected`, `duplicate key`, `too many connections`, `lock wait timeout`, `slow query`, `server has gone away`, `Access denied for user`, `password authentication failed`… |

### General

| Preset | Tags | What gets colored |
| --- | --- | --- |
| Log levels | `fatal / critical` red, `info / notice` blue, `debug / trace` grey, `ok` green | these words as whole words, any case (the default `warning` and `error` tags cover the rest) |
| IPv4 addresses | `IPv4 addresses` | `192.168.1.20`, `203.0.113.9`… Four-part version numbers such as `1.2.3.4` are colored too. |

When tags overlap, the one highest in the tag list wins, so presets added after the default
tags never hide them.

## Using your own file

LogStream reads the file named by `PRESETS_FILE`, `/data/presets.json` by default (in the
`logstream-data` volume). When the file does not exist, the built-in list is used. The file is
read again each time Settings is opened: no restart is needed after an edit.

With docker-compose, the simplest is to keep the file next to `docker-compose.yml`:

1. Copy [`presets.json`](../presets.json) from this repository and edit it.
2. In `docker-compose.yml`, uncomment the line `- ./presets.json:/config/presets.json:ro`.
3. In `.env`, set `PRESETS_FILE=/config/presets.json`, then run `docker compose up -d`.

If the file is invalid (JSON error, bad regular expression or color, duplicate id), Settings
shows the error and the built-in list is used until the file is fixed.

## File format

The file is a JSON list of groups. Each group has a name and a list of presets; each preset has
an `id`, a name and its tags.

```json
[
  {
    "group": { "en": "My apps", "fr": "Mes applis" },
    "presets": [
      {
        "id": "myapp",
        "name": "My app",
        "tags": [
          { "label": "payment failed", "color": "#fca5a5", "pattern": "payment (?:failed|refused)" },
          { "label": "order", "color": "#86efac", "pattern": "order #\\d+" },
          { "label": "slow", "color": "#fde68a", "pattern": "SLOW", "regex": false, "caseSensitive": true }
        ]
      }
    ]
  }
]
```

| Field | Required | Meaning |
| --- | --- | --- |
| `group` | yes | name of the group in the menu |
| `id` | yes | unique identifier of the preset |
| `name` | yes | name of the preset in the menu |
| `tags[].pattern` | yes | what to color: a regular expression, or plain text with `"regex": false` |
| `tags[].color` | yes | background color, `#rrggbb` |
| `tags[].label` | no | name shown in the tag list instead of the pattern |
| `tags[].regex` | no | `true` by default |
| `tags[].wholeWord` | no | only match whole words, `false` by default |
| `tags[].caseSensitive` | no | match case, `false` by default |

Names and labels are either one text for every language (`"My app"`) or one text per language
(`{ "en": "My app", "fr": "Mon appli" }`); a missing language falls back to English.

Regular expressions must work both in the browser (JavaScript) and in Go, which checks them:
avoid look-behind `(?<=…)`, look-ahead `(?=…)` and back-references `\1`. In JSON, every
backslash is written twice: `\d` becomes `"\\d"`. A group named `hl`, `(?<hl>…)`, colors only
that part of the match, as the HTTP presets do with `(?<hl>5\\d\\d)`.
