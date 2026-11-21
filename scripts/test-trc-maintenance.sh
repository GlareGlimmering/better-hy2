#!/usr/bin/env bash
# Sourced by test-systemd-recovery.sh in a disposable systemd VM.
exercise_trc_maintenance() {
  local helper="$artifacts/trc-maintenance" fixture_account baseline_pid boot_state
  systemctl enable "$unit"
  boot_state=$(systemctl is-enabled "$unit")
  baseline_pid=$(systemctl show -p MainPID --value "$unit")
  cp "$unitfile" "$root/old-unit"
  fixture_account=$(trc_fixture_account)

  mkdir "$root/incorrect-assets"
  cp "$artifacts"/{h-ui-linux-amd64,hysteria-linux-amd64,HY2-PAIR.json,SHA256SUMS} "$root/incorrect-assets/"
  printf 'invalid' >>"$root/incorrect-assets/hysteria-linux-amd64"
  if "$helper" "$root/incorrect-assets" >"$root/preflight.log" 2>&1; then
    echo "expected changed kernel digest to be rejected" >&2
    return 1
  fi
  grep -Fq 'verify pinned release asset: SHA256 mismatch for hysteria-linux-amd64' "$root/preflight.log"
  [[ $(systemctl show -p MainPID --value "$unit") == "$baseline_pid" ]]

  # Put the candidate on the wrong HTTP port for one start. The real systemd
  # unit and candidate run, but the required panel health check must fail.
  mkdir "$root/wrappers"
  cat >"$root/wrappers/systemctl" <<'SH'
#!/usr/bin/env bash
if [[ $1 == start && ${2:-} == h-ui.service && -f $HUI_TRC_FIXTURE/fail-next-start ]]; then
  rm "$HUI_TRC_FIXTURE/fail-next-start" || exit 2
  directory=/run/systemd/system/h-ui.service.d
  mkdir -p "$directory" || exit 2
  dropin=$directory/98-hui-fixture-port.conf
  trap 'rm -f "$dropin"; /usr/bin/systemctl daemon-reload' EXIT
  printf '[Service]\nExecStart=\nExecStart=/usr/local/h-ui/h-ui -p 18001\n' >"$dropin" || exit 2
  /usr/bin/systemctl daemon-reload || exit 2
  /usr/bin/systemctl "$@" || exit 2
  for attempt in {1..30}; do
    if curl -fsS --max-time 1 http://127.0.0.1:18001/ >/dev/null 2>&1; then
      # Record a candidate-only database change to prove rollback restores it.
      python3 -c 'import sqlite3; db=sqlite3.connect("/usr/local/h-ui/data/h_ui.db"); db.execute("UPDATE account SET quota=2000000000 WHERE username=?", ("smokeuser",)); db.commit()' || exit 2
      touch "$HUI_TRC_FIXTURE/injected"
      exit 0
    fi
    sleep 1
  done
  exit 2
fi
exec /usr/bin/systemctl "$@"
SH
  chmod 0755 "$root/wrappers/systemctl"
  touch "$root/fail-next-start"
  if HUI_TRC_FIXTURE="$root" PATH="$root/wrappers:$PATH" "$helper" "$artifacts" >"$root/failed-trc.log" 2>&1; then
    echo "expected failure and rollback from failed candidate health" >&2
    return 1
  fi
  if [[ ! -f $root/injected ]] || ! grep -Fq 'target panel health:' "$root/failed-trc.log" ||
      ! grep -Fq 'rollback successful: v2.3.1-trc.1' "$root/failed-trc.log"; then
    tail -n 25 "$root/failed-trc.log"
    return 1
  fi
  cmp "$live/h-ui" "$artifacts/old-panel"
  cmp "$live/bin/hysteria-linux-amd64" "$artifacts/old-kernel"
  cmp "$unitfile" "$root/old-unit"
  [[ $(systemctl is-enabled "$unit") == "$boot_state" ]]
  [[ $(trc_fixture_account) == "$fixture_account" ]]
  check_panel
  check_proxy "$artifacts/old-kernel"

  "$helper" "$artifacts"
  cmp "$live/h-ui" "$artifacts/h-ui-linux-amd64"
  cmp "$live/bin/hysteria-linux-amd64" "$artifacts/hysteria-linux-amd64"
  cmp "$unitfile" "$root/old-unit"
  [[ $(systemctl is-enabled "$unit") == "$boot_state" ]]
  [[ $(trc_fixture_account) == "$fixture_account" ]]
  check_panel
  check_proxy "$artifacts/hysteria-linux-amd64"
  echo "Verified pinned-asset rejection before stop, real health failure and rollback, and paired activation with authenticated HY2 traffic."
}

trc_fixture_account() {
  python3 - "$live/data/h_ui.db" <<'PYTHON'
import sqlite3
import sys

with sqlite3.connect(f"file:{sys.argv[1]}?mode=ro", uri=True) as db:
    assert db.execute("PRAGMA integrity_check").fetchone() == ("ok",)
    row = db.execute(
        "SELECT id, username, con_pass, quota, outbound FROM account WHERE username='smokeuser'"
    ).fetchone()
    assert row
    print(row)
PYTHON
}
