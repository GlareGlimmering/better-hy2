#!/usr/bin/env bash
# Run only on a disposable Linux VM with real systemd.
set -euo pipefail
trap 'printf "Recovery check failed at line %s\n" "$LINENO" >&2' ERR

[[ $EUID == 0 && ${HUI_DISPOSABLE_VM:-} == 1 ]]
[[ $(cat /proc/1/comm) == systemd ]]
systemctl show --property=SystemState >/dev/null
[[ ${2:-} == active || ${2:-} == stopped ]]
artifacts=$(realpath "$1")
(cd "$artifacts" && sha256sum --check SHA256SUMS)
root=$(mktemp -d /var/tmp/hui-recovery.XXXXXX)
live=$root/live
unit=better-hy2-recovery-test.service
unitfile=/etc/systemd/system/$unit
if [[ -e $unitfile ]] || systemctl cat "$unit" >/dev/null 2>&1; then
  rmdir "$root"
  exit 1
fi
client=
echo_server=
cleanup() {
  local status=$?
  trap - EXIT
  if [[ -n $client ]]; then kill "$client" 2>/dev/null || true; fi
  if [[ -n $echo_server ]]; then kill "$echo_server" 2>/dev/null || true; fi
  if ((status != 0)); then
    systemctl status "$unit" --no-pager --lines=0 || true
    printf 'Recovery test failed. Fixture preserved at %s for this job.\n' "$root" >&2
  fi
  systemctl stop "$unit" || true
  systemctl disable "$unit" 2>/dev/null || true
  rm -f "$unitfile"
  systemctl daemon-reload || true
  if ((status == 0)); then rm -rf -- "$root"; fi
  exit "$status"
}
trap cleanup EXIT

check_proxy() {
  local kernel=$1 i result
  "$kernel" client -c "$root/client.yaml" >"$root/client.log" 2>&1 &
  client=$!
  result=1
  for ((i=0; i<40; i++)); do
    kill -0 "$client" || break
    if [[ $(curl -fsS --max-time 2 --noproxy '' --socks5-hostname 127.0.0.1:11080 \
      http://127.0.0.1:18080/probe 2>/dev/null) == recovery-fixture ]]; then
      result=0
      break
    fi
    sleep .25
  done
  kill "$client" 2>/dev/null || true
  wait "$client" 2>/dev/null || true
  client=
  return "$result"
}

check_panel() {
  local i
  for ((i=0; i<40; i++)); do
    if systemctl is-active --quiet "$unit" &&
       curl -fsS --max-time 2 http://127.0.0.1:8000/ 2>/dev/null | grep -F 'id="app"' >/dev/null; then
      return
    fi
    sleep .25
  done
  return 1
}

install_pair() {
  local panel=$1 kernel=$2
  [[ $(systemctl show -p ActiveState --value "$unit") == inactive ]]
  install -m 0755 "$panel" "$live/h-ui"
  install -m 0755 "$kernel" "$live/bin/hysteria-linux-amd64"
}

restore() {
  local state=$1
  systemctl stop "$unit"
  if systemctl is-failed --quiet "$unit"; then systemctl reset-failed "$unit"; fi
  [[ $(systemctl show -p ActiveState --value "$unit") == inactive ]]
  (cd "$root/snapshot" && sha256sum --check "$root/snapshot.sha256")
  cp -a "$root/snapshot/live" "$root/restored"
  mv "$live" "$root/displaced"
  mv "$root/restored" "$live"
  cp -a "$root/snapshot/unit" "$unitfile"
  systemctl daemon-reload
  if [[ $state == active ]]; then systemctl enable "$unit"; else systemctl disable "$unit"; fi
  diff -r "$root/snapshot/live" "$live"
  cmp "$root/snapshot/unit" "$unitfile"
  [[ $(systemctl show -p ActiveState --value "$unit") == inactive ]]
  if [[ $state == active ]]; then systemctl start "$unit"; fi
}

mkdir -p "$live/bin" "$root/http"
printf '%s\n' recovery-fixture >"$root/http/probe"
python3 -m http.server 18080 --bind 127.0.0.1 --directory "$root/http" >"$root/http.log" 2>&1 &
echo_server=$!
cat >"$root/client.yaml" <<'EOF'
server: 127.0.0.1:18443
auth: smokeuser.fixture-credential
tls:
  insecure: true
socks5:
  listen: 127.0.0.1:11080
EOF
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj /CN=localhost \
  -keyout "$root/key.pem" -out "$root/cert.pem" >"$root/openssl.log" 2>&1

cat >"$unitfile" <<EOF
[Service]
Type=exec
WorkingDirectory=$live
Environment=HUI_DATA=$live/
Environment=GIN_MODE=release
Environment=HYSTERIA_DISABLE_UPDATE_CHECK=1
ExecStart=$live/h-ui -p 8000
[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
install_pair "$artifacts/old-panel" "$artifacts/old-kernel"
[[ $("$live/h-ui" version) == 'Better HY2 version v1.0.0' ]]
(cd "$live" && HUI_DATA="$live/" ./h-ui reset >"$root/reset.log")
python3 - "$live/data/h_ui.db" "$root" <<'PY'
import sqlite3
import sys

db, root = sys.argv[1:]
config = f"""listen: 127.0.0.1:18443
tls:
  cert: {root}/cert.pem
  key: {root}/key.pem
trafficStats:
  listen: 127.0.0.1:19090
"""
with sqlite3.connect(db) as conn:
    assert conn.execute("UPDATE config SET value=? WHERE key='HYSTERIA2_CONFIG'", (config,)).rowcount == 1
    assert conn.execute("UPDATE config SET value='1' WHERE key='HYSTERIA2_ENABLE'").rowcount == 1
    conn.execute("INSERT INTO account(username, con_pass, quota, expire_time) VALUES(?, ?, -1, 253370736000000)",
                 ("smokeuser", "smokeuser.fixture-credential"))
PY
systemctl start "$unit"
check_panel
check_proxy "$artifacts/old-kernel"
if [[ $2 == active ]]; then systemctl enable "$unit"; else systemctl disable "$unit"; fi
if [[ $2 == stopped ]]; then systemctl stop "$unit"; fi
[[ $(systemctl show -p ActiveState --value "$unit") == "$(if [[ $2 == active ]]; then echo active; else echo inactive; fi)" ]]
systemctl stop "$unit"
[[ $(systemctl show -p ActiveState --value "$unit") == inactive ]]

# Model a stopped database with committed, uncheckpointed WAL data. The
# standalone writer exits without a SQLite close, leaving real sidecars.
python3 - "$live/data/h_ui.db" <<'PY'
import os
import sqlite3
import sys

conn = sqlite3.connect(sys.argv[1])
assert conn.execute("PRAGMA journal_mode=WAL").fetchone()[0] == "wal"
conn.execute("CREATE TABLE recovery_fixture (marker TEXT NOT NULL)")
conn.execute("INSERT INTO recovery_fixture VALUES ('committed-in-wal')")
conn.commit()
os._exit(0)
PY
test -s "$live/data/h_ui.db-wal"
test -s "$live/data/h_ui.db-shm"
mkdir "$root/snapshot"
cp -a "$live" "$root/snapshot/live"
cp -a "$unitfile" "$root/snapshot/unit"
(cd "$root/snapshot" && find live unit -type f -print0 | sort -z | xargs -0 sha256sum) >"$root/snapshot.sha256"
(cd "$root/snapshot" && sha256sum --check "$root/snapshot.sha256")
install_pair "$artifacts/h-ui-linux-amd64" "$artifacts/hysteria-linux-amd64"
systemctl start "$unit"
check_panel
check_proxy "$artifacts/hysteria-linux-amd64"
python3 - "$live/data/h_ui.db" <<'PY'
import json
import sqlite3
import sys
import urllib.request

with sqlite3.connect(sys.argv[1]) as db:
    assert db.execute("PRAGMA integrity_check").fetchone() == ("ok",)
    assert db.execute("SELECT marker FROM recovery_fixture").fetchone() == ("committed-in-wal",)
    account_id = db.execute("SELECT id FROM account WHERE username='smokeuser'").fetchone()[0]
    secret = db.execute("SELECT value FROM config WHERE key='HYSTERIA2_TRAFFIC_STATS_SECRET'").fetchone()[0]
for path in ("capabilities", "traffic/batch"):
    req = urllib.request.Request(f"http://127.0.0.1:19090/{path}", headers={"Authorization": secret})
    with urllib.request.urlopen(req, timeout=5) as response:
        result = json.load(response)
    if path == "capabilities":
        assert result == {"accountConsistency": 2, "accountBandwidth": 1}
    else:
        assert f"account:{account_id}" in result["users"]
PY

systemctl stop "$unit"
# Inject a real systemd start failure after the candidate has migrated SQLite.
printf '\n[Service]\nExecStartPost=/bin/false\n' >>"$unitfile"
systemctl daemon-reload
if systemctl start "$unit"; then
  echo "expected injected systemd failure" >&2
  exit 1
fi
restore "$2"
[[ $("$live/h-ui" version) == 'Better HY2 version v1.0.0' ]]
cmp "$live/bin/hysteria-linux-amd64" "$artifacts/old-kernel"
if [[ $2 == active ]]; then
  systemctl is-enabled --quiet "$unit"
else
  [[ $(systemctl show -p ActiveState --value "$unit") == inactive ]]
  [[ $(systemctl is-enabled "$unit" || true) == disabled ]]
  systemctl start "$unit"
fi
check_panel
check_proxy "$artifacts/old-kernel"
python3 - "$live/data/h_ui.db" <<'PY'
import sqlite3
import sys

with sqlite3.connect(sys.argv[1]) as db:
    assert db.execute("PRAGMA integrity_check").fetchone() == ("ok",)
    assert db.execute("SELECT marker FROM recovery_fixture").fetchone() == ("committed-in-wal",)
    assert db.execute("SELECT username FROM account WHERE username='smokeuser'").fetchone() == ("smokeuser",)
PY
if [[ $2 == stopped ]]; then
  systemctl stop "$unit"
  [[ $(systemctl show -p ActiveState --value "$unit") == inactive ]]
fi
echo "Verified real systemd, both binary pairs, SQLite/WAL, offline recovery and HY2 traffic ($2)."
