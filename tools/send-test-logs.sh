#!/usr/bin/env bash
# Sends test syslog messages (RFC 3164 and RFC 5424) over UDP.
# Usage: ./tools/send-test-logs.sh [host] [port] [count]
set -eu

HOST=${1:-127.0.0.1}
PORT=${2:-514}
COUNT=${3:-100}

hosts=(web-01 web-02 db-01 fw-edge nas-home)
apps=(nginx sshd postgres kernel backup cron)

# "severity|message" (3 = err, 4 = warning, 5 = notice, 6 = info, 7 = debug)
samples=(
  "6|GET /api/health 200 in 3ms - ok"
  "6|Accepted publickey for admin from 192.168.1.20 port 51422"
  "6|Backup job finished: ok (12.4 GB, 00:14:32)"
  "4|warning: disk usage at 91% on /var"
  "4|WARNING slow query (2.3s): SELECT * FROM events WHERE ts > now() - interval '1 day'"
  "3|error: connection refused to upstream 10.0.0.12:5432"
  "3|Error while rotating logs: permission denied (/var/log/app.log)"
  "5|Configuration reloaded, 42 rules loaded"
  "7|debug: cache hit ratio 0.93, token refreshed"
  "4|Temperature warning: CPU at 84°C"
  "6|link eth0 up, 1000Mbps full duplex, status ok"
  "3|Out of memory: Killed process 4121 (java)"
)

for ((i = 0; i < COUNT; i++)); do
  h=${hosts[RANDOM % ${#hosts[@]}]}
  a=${apps[RANDOM % ${#apps[@]}]}
  s=${samples[RANDOM % ${#samples[@]}]}
  sev=${s%%|*}
  msg=${s#*|}
  pri=$((16 * 8 + sev)) # facility local0

  if ((i % 2)); then
    ts=$(LC_ALL=C date '+%b %e %H:%M:%S')
    line="<$pri>$ts $h $a[$((RANDOM % 9000 + 1000))]: $msg"
  else
    ts=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
    line="<$pri>1 $ts $h $a $((RANDOM)) - - $msg"
  fi
  printf '%s' "$line" > "/dev/udp/$HOST/$PORT"
  sleep 0.05
done
echo "$COUNT messages sent to $HOST:$PORT (udp)"
