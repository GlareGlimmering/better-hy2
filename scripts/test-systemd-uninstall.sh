#!/usr/bin/env bash
set -euo pipefail

# Uses standard installation paths. Run only in the disposable hosted CI VM.
[[ $EUID == 0 && ${GITHUB_ACTIONS:-} == true && ${RUNNER_ENVIRONMENT:-} == github-hosted ]]
[[ $(cat /proc/1/comm) == systemd ]]
binary=$(realpath "$1")
root=/usr/local/h-ui
state=/var/lib/h-ui
access=/var/lib/hui
xray="$state/xray"
unit=/etc/systemd/system/h-ui.service
xray_unit=/etc/systemd/system/h-ui-xray.service
independent=/etc/systemd/system/better-hy2-independent-fixture.service
link=/usr/local/bin/hui
work=$(mktemp -d)
for p in "$root" "$unit" "$xray_unit" "$independent" "$link" "$state" "$access"; do
  [[ ! -e $p && ! -L $p ]]
done
! getent passwd h-ui-xray
! getent group h-ui-xray

install -d "$root/bin" "$root/data" "$root/logs" "$root/export" "$root/my_acme_dir" "$state/backups" "$xray/certs" "$xray/versions/fixture" "$access"
install -m 0755 "$binary" "$root/h-ui"
install -m 0644 h-ui.service "$unit"
ln -s "$root/h-ui" "$link"
printf 'kernel fixture\n' > "$root/bin/hysteria-linux-amd64"
printf 'HY2 configuration\n' > "$root/bin/hysteria2.yaml"
printf 'certificate storage\n' > "$root/my_acme_dir/fixture"
printf 'backup\n' > "$state/backups/backup"
printf 'proxy config\n' > "$xray/config.json"
printf 'access log\n' > "$access/hy2_access.log"
printf 'rotated log\n' > "$access/hy2_access-2026-01-01T00-00-00.000.log.gz"

useradd --system --user-group --home-dir "$xray" --shell /usr/sbin/nologin h-ui-xray
ln -s "$xray/versions/fixture" "$xray/current"
printf '%s\n' '#!/bin/sh' 'exec /bin/sleep infinity' > "$xray/current/xray"
chmod 0755 "$xray/current/xray"
cat > "$xray_unit" <<'UNIT'
[Unit]
Description=Better HY2 managed Xray outbound
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=h-ui-xray
Group=h-ui-xray
ExecStart=/var/lib/h-ui/xray/current/xray run -c /var/lib/h-ui/xray/config.json
Restart=on-failure
RestartSec=3s
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=strict
ReadOnlyPaths=/var/lib/h-ui/xray
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
CapabilityBoundingSet=
AmbientCapabilities=

[Install]
WantedBy=multi-user.target
UNIT
printf '[Service]\nExecStart=/bin/sleep infinity\n' > "$independent"
systemctl daemon-reload
systemctl start better-hy2-independent-fixture.service
systemctl enable --now h-ui-xray.service h-ui.service
for attempt in {1..60}; do
  if curl -fsS http://127.0.0.1:8000/ -o "$work/index.html"; then break; fi
  sleep 0.25
done
grep -q 'id="app"' "$work/index.html"
test -s "$root/data/h_ui.db"

printf '\n' | "$link" uninstall | tee "$work/cancel"
grep -Fq 'Cancelled. No changes made.' "$work/cancel"
systemctl is-active --quiet h-ui.service
systemctl is-active --quiet h-ui-xray.service
test -L "$link"
test -s "$access/hy2_access.log"

printf '%s\n' UNINSTALL | "$link" uninstall | tee "$work/remove"
grep -Fq 'Better HY2 uninstall complete.' "$work/remove"
for name in h-ui.service h-ui-xray.service; do
  [[ $(systemctl show -p LoadState --value "$name") == not-found ]]
done
for path in "$root" "$state" "$access" "$link" "$unit" "$xray_unit"; do
  [[ ! -e $path && ! -L $path ]]
done
! getent passwd h-ui-xray
! getent group h-ui-xray
if command -v nft >/dev/null && nft list tables > "$work/nft-tables" 2>/dev/null; then
  ! grep -Fxq 'table inet hui_porthopping' "$work/nft-tables"
fi
systemctl is-active --quiet better-hy2-independent-fixture.service

printf '%s\n' '#!/bin/sh' 'echo other-hui' > "$link"
chmod 0755 "$link"
"$binary" uninstall | grep -F 'No Better HY2 uninstall items remain.'
[[ $("$link") == other-hui ]]

install -d "$xray"
printf 'external Xray data\n' > "$xray/keep"
printf '[Service]\nExecStart=/bin/sleep infinity\n' > "$xray_unit"
systemctl daemon-reload
systemctl start h-ui-xray.service
install -d "$state/backups"
printf 'fixture panel data\n' > "$state/backups/backup"
printf 'PURGE\n' | "$binary" uninstall --purge | tee "$work/purge"
grep -Fq 'Preserve h-ui-xray.service' "$work/purge"
test -s "$xray/keep"
[[ ! -e $state/backups ]]
systemctl is-active --quiet h-ui-xray.service
[[ $("$link") == other-hui ]]
