#!/usr/bin/env bash
set -euo pipefail

# Uses standard installation paths. Run only in the disposable hosted CI VM.
[[ $EUID == 0 && ${GITHUB_ACTIONS:-} == true && ${RUNNER_ENVIRONMENT:-} == github-hosted ]]
[[ $(cat /proc/1/comm) == systemd ]]
binary=$(realpath "$1")
root=/usr/local/h-ui
unit=/etc/systemd/system/h-ui.service
xray_unit=/etc/systemd/system/h-ui-xray.service
hy2_unit=/etc/systemd/system/better-hy2-independent-fixture.service
link=/usr/local/bin/hui
work=$(mktemp -d)
for p in "$root" "$unit" "$xray_unit" "$hy2_unit" "$link" /var/lib/h-ui; do
  [[ ! -e $p && ! -L $p ]]
done
for name in h-ui.service h-ui-xray.service better-hy2-independent-fixture.service; do
  [[ $(systemctl show -p LoadState --value "$name") == not-found ]]
done

install -d "$root/bin" "$root/data" "$root/logs" "$root/export" /var/lib/h-ui/backups /var/lib/h-ui/xray
install -m 0755 "$binary" "$root/h-ui"
install -m 0644 h-ui.service "$unit"
ln -s "$root/h-ui" "$link"
# The deliberately non-executable kernel fixture prevents network downloads.
printf '%s\n' 'external kernel' > "$root/bin/hysteria-linux-amd64"
printf '%s\n' 'existing HY2 settings' > "$root/bin/hysteria2.yaml"
printf '%s\n' 'parked backup' > /var/lib/h-ui/backups/backup
printf '%s\n' 'shared Xray data' > /var/lib/h-ui/xray/keep
for path in "$xray_unit" "$hy2_unit"; do
  printf '[Service]\nExecStart=/bin/sleep infinity\n' > "$path"
done
systemctl daemon-reload
systemctl start h-ui-xray.service better-hy2-independent-fixture.service
systemctl enable --now h-ui.service
for attempt in {1..60}; do
  if curl -fsS http://127.0.0.1:8000/ -o "$work/index.html"; then break; fi
  sleep 0.25
done
grep -q 'id="app"' "$work/index.html"
test -s "$root/data/h_ui.db"

printf '\n' | "$link" uninstall | tee "$work/cancel"
grep -Fq 'Cancelled. No changes made.' "$work/cancel"
systemctl is-active --quiet h-ui.service
systemctl is-enabled --quiet h-ui.service
test -L "$link"

printf '%s\n' UNINSTALL | "$root/h-ui" uninstall | tee "$work/remove"
grep -Fq 'Better HY2 uninstall complete.' "$work/remove"
[[ $(systemctl show -p LoadState --value h-ui.service) == not-found ]]
[[ ! -e $root/h-ui && ! -e $unit && ! -L $link ]]
test -s "$root/data/h_ui.db"
test -s /var/lib/h-ui/backups/backup
test -s "$root/bin/hysteria2.yaml"

printf '%s\n' '#!/bin/sh' 'echo other-hui' > "$link"
chmod 0755 "$link"
"$binary" uninstall | grep -F 'No Better HY2 uninstall items remain.'
printf '\n' | "$binary" uninstall --purge | grep -F 'Cancelled. No changes made.'
test -s "$root/data/h_ui.db"
printf '%s\n' PURGE | "$binary" uninstall --purge | tee "$work/purge"
grep -Fq "Remove $root/data" "$work/purge"
grep -Fq 'Remove /var/lib/h-ui/backups' "$work/purge"
[[ ! -e $root/data && ! -e $root/logs && ! -e $root/export && ! -e /var/lib/h-ui/backups ]]
"$binary" uninstall --purge | grep -F 'No Better HY2 uninstall items remain.'
[[ $("$link") == other-hui ]]
test -s "$root/bin/hysteria-linux-amd64"
test -s "$root/bin/hysteria2.yaml"
test -s /var/lib/h-ui/xray/keep
systemctl is-active h-ui-xray.service better-hy2-independent-fixture.service
