#!/usr/bin/env bash
# Sourced by test-systemd-recovery.sh in the disposable VM.
exercise_cross_generation_migration() {
  local target old_state
  target=$("$artifacts/h-ui-linux-amd64" version)
  target=${target#Better HY2 version }
  [[ $target == v2.* ]]
  cp "$unitfile" "$root/old-unit"
  old_state=$(systemctl is-enabled "$unit")
  migration_inspect baseline
  mkdir "$root/wrappers"
  cat >"$root/fail-after-migration" <<'PYFAIL'
#!/usr/bin/env python3
from pathlib import Path
import sqlite3
import sys
import time

for attempt in range(50):
    try:
        with sqlite3.connect("file:/usr/local/h-ui/data/h_ui.db?mode=ro", uri=True, timeout=0.1) as db:
            if db.execute("SELECT count(*) FROM panel_admin").fetchone()[0]:
                Path(sys.argv[1], "candidate-applied-db-migration").touch()
                sys.exit(1)
    except sqlite3.Error:
        pass
    time.sleep(0.1)
sys.exit(2)
PYFAIL
  chmod +x "$root/fail-after-migration"
  cat >"$root/wrappers/systemctl" <<'SH'
#!/usr/bin/env bash
if [[ $1 == start && ${2:-} == h-ui.service && -f $HUI_MIGRATION_FIXTURE/fail-next-start ]]; then
  rm "$HUI_MIGRATION_FIXTURE/fail-next-start"
  mkdir -p /run/systemd/system/h-ui.service.d
  dropin=/run/systemd/system/h-ui.service.d/98-hui-fixture-fail.conf
  trap 'rm -f "$dropin"; /usr/bin/systemctl daemon-reload' EXIT
  printf '[Service]\nExecStartPost=%s/fail-after-migration %s\n' "$HUI_MIGRATION_FIXTURE" "$HUI_MIGRATION_FIXTURE" >"$dropin"
  /usr/bin/systemctl daemon-reload
  # Starting the actual staged service fails in systemd.
  if /usr/bin/systemctl "$@"; then exit 99; fi
  touch "$HUI_MIGRATION_FIXTURE/injected"
  exit 1
fi
exec /usr/bin/systemctl "$@"
SH
  chmod +x "$root/wrappers/systemctl"
  export HUI_MIGRATION_FIXTURE="$root"
  # Serve exactly the built workflow artifacts through the installer curl
  # shim. Requests to the local panel and proxy use the real curl.
  curl() {
    local dest="" last="${*: -1}" previous=""
    if [[ $last == https://github.com/GlareGlimmering/better-hy2/releases/download/"$target"/* ]]; then
      for arg in "$@"; do
        if [[ $previous == --output ]]; then dest=$arg; break; fi
        previous=$arg
      done
      [[ -n $dest ]] && cp "$artifacts/${last##*/}" "$dest"
    else
      command curl "$@"
    fi
  }

  touch "$root/fail-next-start"
  # Run main in a fresh Bash so its EXIT trap and strict mode are independent.
  if PATH="$root/wrappers:$PATH" run_fixture_installer "$target" "$root/failed-migration.log"; then
    echo "migration should have failed and rolled back" >&2
    exit 1
  fi
  if [[ ! -f $root/injected || ! -f $root/candidate-applied-db-migration ]] ||
      ! grep -Fq 'rollback successful' "$root/failed-migration.log"; then
    tail -n 20 "$root/failed-migration.log"
    exit 1
  fi
  cmp "$live/h-ui" "$artifacts/old-panel"
  cmp "$live/bin/hysteria-linux-amd64" "$artifacts/old-kernel"
  cmp "$unitfile" "$root/old-unit"
  [[ $(systemctl is-enabled "$unit") == "$old_state" ]]
  check_panel
  migration_inspect rollback
  check_proxy "$artifacts/old-kernel"

  if ! run_fixture_installer "$target" "$root/successful-migration.log"; then
    tail -n 20 "$root/successful-migration.log"
    exit 1
  fi
  grep -Fq 'Migration successful' "$root/successful-migration.log"
  cmp "$live/h-ui" "$artifacts/h-ui-linux-amd64"
  cmp "$live/bin/hysteria-linux-amd64" "$artifacts/hysteria-linux-amd64"
  cmp "$unitfile" "$root/old-unit"
  [[ $(systemctl is-enabled "$unit") == "$old_state" ]]
  check_panel
  migration_inspect migrated
  check_proxy "$artifacts/hysteria-linux-amd64"
  echo "Verified actual installer migration, failed start rollback, admin login, unchanged account and subscription, and real HY2 traffic."
}

run_fixture_installer() {
  export -f curl
  export artifacts target
  # A separate Bash preserves errexit even when the caller checks exit status.
  bash "$(dirname "$0")/../install.sh" --version "$1" --migrate-v1 >"$2" 2>&1
}

migration_inspect() {
  python3 - "$live" "$root" "$1" <<'PY'
import json
from pathlib import Path
import re
import sqlite3
import sys
import urllib.request

live, root = map(Path, sys.argv[1:3])
phase = sys.argv[3]
endpoint = "http://127.0.0.1:8000/hui"
auth = "smokeuser.fixture-credential"
legacy = {
    key: re.search(rf"^Better HY2 Login {label}: (.+)$", (root / "reset.log").read_text(), re.M)[1]
    for key, label in (("username", "Username"), ("pass", "Password"))
}

def call(path, data=None, token=None):
    headers = {"User-Agent": "Clash", "Content-Type": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    request = urllib.request.Request(
        endpoint + path, None if data is None else json.dumps(data).encode(), headers
    )
    with urllib.request.urlopen(request, timeout=5) as response:
        raw = response.read()
        return raw if path == "/" + auth else json.loads(raw)

with sqlite3.connect(live / "data" / "h_ui.db") as conn:
    assert conn.execute("PRAGMA integrity_check").fetchone() == ("ok",)
    row = conn.execute("SELECT id,username,con_pass,outbound,quota,upload,download FROM account WHERE username='smokeuser'").fetchone()
    assert row and row[2:5] == (auth, "DIRECT", 1000000000)
    if phase == "baseline":
        (root / "migration-account.json").write_text(json.dumps(row))
    else:
        old = json.loads((root / "migration-account.json").read_text())
        assert list(row[:5]) == old[:5] and row[5] >= old[5] and row[6] >= old[6]
    if phase == "rollback":
        assert conn.execute("SELECT count(*) FROM sqlite_master WHERE name='panel_admin'").fetchone() == (0,)
    if phase == "migrated":
        assert conn.execute("SELECT count(*) FROM panel_admin WHERE username=?", (legacy["username"],)).fetchone() == (1,)
assert auth.encode() in call("/" + auth)
login = call("/auth/login", legacy)
if phase == "migrated":
    assert login["data"] == {"passwordChangeRequired": True}
    login = call("/auth/changePassword", {**legacy, "newPassword": "fixture-panel-new-password-2026"})
assert call("/account/getAccountInfo", token=login["data"]["accessToken"])["code"] == 20000
print(f"v1 migration fixture: {phase}")
PY
}
