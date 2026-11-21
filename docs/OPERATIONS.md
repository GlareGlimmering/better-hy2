# Better HY2 installation and operations

## Installation and first-time setup

Use the [README installer](../README.md). The README pins a stable tag. The installer verifies SHA256 and executable version, installs HY2, initializes schema/admin without starting the server, then installs systemd and checks HTML plus the entry JavaScript. A partial installation without a valid binary/unit resumes fresh initialization. An existing valid installation uses the candidate's upgrader. No credentials or databases are shipped in releases.

The service remains `h-ui.service`, executable `/usr/local/h-ui/h-ui`, data root `HUI_DATA=/usr/local/h-ui/`, SQLite `/usr/local/h-ui/data/h_ui.db`, runtime data `/var/lib/h-ui`. The new `/usr/local/bin/hui` is a symlink to that binary. An unrelated existing `hui` command is never overwritten.

Panel endpoint: `http://127.0.0.1:8000`. Provide HTTPS through Caddy/Nginx or a Cloudflare Tunnel; it must forward to the loopback address. For private initial access, an SSH local forward is another option. Do not expose raw HTTP publicly. Retain reverse-proxy trust and access restrictions appropriate to your environment. Changing panel paths or certificates is not required by the rename.

Recover administrator access from any working directory:

```bash
sudo /usr/local/h-ui/h-ui reset
```

This resets only administrator credentials, not proxy accounts. `HUI_DATA` can explicitly select another data root: never use it casually on production.

## Panel administrators in v1.1.0-rc.4 and later

To customize local administrator credentials, use `sudo hui reset --interactive` for both fields, `sudo hui reset username` to change only the username, or `sudo hui reset password` to change only the password. Password prompts are hidden. Scripts can read one line from standard input with `hui reset --username <name> --password-stdin` or `hui reset password --stdin`. Plain `hui reset` still generates a password.

Panel passwords and HY2 connection credentials are separate. Existing active administrator usernames migrate automatically. If the old panel password was also used as a proxy credential, sign in with it once and complete the required password change to obtain a new session. If anyone else knows the old password, use local recovery with `sudo hui reset` before exposing the login page. Reset keeps the existing administrator username when available, outputs a new panel-only password, and revokes that administrator's sessions. Sign out and use **Change password** on the login page to set a personal password.

Move any clients still using old administrator credentials to an independent proxy account before upgrading. Ordinary proxy accounts and their subscriptions carry forward. Account import and export are removed. Historical export files are retained and may contain connection credentials. Review and handle them privately.

For rollback to RC.3, restore its pre-upgrade SQLite snapshot as well as the matching binary and configuration. The RC.3 executable cannot authenticate panel administrators against the migrated database.

### Reverse proxy, subscriptions, and login limits

Keep the existing public HTTPS origin and path prefix for the panel and subscriptions. The managed HY2 callback binds to a separate loopback socket and its address is regenerated before HY2 starts. Remove any reverse-proxy rule dedicated to the old `POST <prefix>/hui/hysteria2/auth` endpoint. Forward only to the panel port.

Only immediate peers explicitly listed in the panel process's `HUI_TRUSTED_PROXIES` environment variable can supply a client IP via `X-Forwarded-For`. List exact IPs or CIDRs separated by commas. With the variable unset, clients behind a proxy share its login budget. Login and password changes share a limit per IP of five attempts in a burst, replenished at one per second. List only proxies that sanitize or correctly append to the header and keep the origin private. For a single local Nginx instance with no upstream CDN, set `Environment=HUI_TRUSTED_PROXIES=127.0.0.1/32` in the panel unit's service drop-in and use this origin configuration inside the panel's TLS virtual host:

```nginx
location / {
    proxy_pass http://127.0.0.1:8000;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $remote_addr;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_cache off;
    access_log off;
}
```

Reload systemd and restart the panel after changing its environment. For Caddy, keep forwarding to the panel via `reverse_proxy` and omit subscription paths from the site's access logs. For Cloudflare, set a **Bypass cache** rule on the panel hostname or subscription path (including its prefix) if an existing cache rule can match it. Apply equivalent exclusions to intermediary caches and clear previously cached subscriptions if applicable. The response sends `Cache-Control: no-store`. Disable raw URL logging on subscription routes or log a fixed route label, method, and status. Review access, error/debug, CDN and tracing logs because the request path itself contains the proxy credential.

[Nginx proxy controls](https://nginx.org/en/docs/http/ngx_http_proxy_module.html) · [Nginx access logging](https://nginx.org/en/docs/http/ngx_http_log_module.html) · [Caddy reverse proxy](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy) · [Cloudflare cache rules](https://developers.cloudflare.com/cache/how-to/cache-rules/settings/)

## HY2 configuration

Use HY2 Management to configure your own listen address, certificate/ACME and networking. Keep the exact `v2.13.0-hui.5-dev` kernel from the v2 release and verify it against the release SHA256SUMS. HTTP authentication and `userOutbounds` are generated by the panel; do not replace them manually. IPv4-only ACME propagation resolver handling remains in the custom kernel. TLS verification must remain enabled where configured.

Subscription server defaults to detected public IPv4. A NAT-only host without a public interface address needs the supported explicit override `HUI_NODE_ADDRESS`. The subscription HTTP URL and TLS SNI are separate fields.

## Generate a private HY2 certificate

Run this on a trusted machine with OpenSSL. Replace `hy2.example.com` with your HY2 certificate name before running it. Keep a pre-existing PKI in a separate directory.

```bash
mkdir -m 0700 hy2-pki &&
(
  cd hy2-pki &&
  umask 077 &&
  openssl req -x509 -newkey rsa:3072 -nodes -sha256 -days 3650 \
    -subj '/CN=Better HY2 private CA' \
    -addext 'basicConstraints=critical,CA:TRUE' \
    -addext 'keyUsage=critical,keyCertSign,cRLSign' \
    -keyout hy2-ca.key -out hy2-ca.crt &&
  openssl req -new -newkey rsa:2048 -nodes \
    -subj '/CN=hy2.example.com' -keyout hy2-server.key -out hy2-server.csr &&
  printf '%s\n' 'basicConstraints=critical,CA:FALSE' \
    'keyUsage=critical,digitalSignature,keyEncipherment' \
    'extendedKeyUsage=serverAuth' 'subjectAltName=DNS:hy2.example.com' > hy2-server.ext &&
  openssl x509 -req -in hy2-server.csr -CA hy2-ca.crt -CAkey hy2-ca.key \
    -CAcreateserial -out hy2-server.crt -days 365 -sha256 -extfile hy2-server.ext &&
  openssl verify -CAfile hy2-ca.crt -verify_hostname hy2.example.com hy2-server.crt
)
```

Use `hy2-server.crt` and `hy2-server.key` in the HY2 TLS configuration. Keep the service key readable only to its service account, and keep the CA private key offline. Give clients `hy2-ca.crt` and set the SNI to `hy2.example.com`. The client can connect to the public IP by using that SNI and the private CA. Renew the service certificate before expiry. A new certificate signed by the same CA keeps the existing client trust configuration valid. [Hysteria client TLS options](https://v2.hysteria.network/docs/advanced/Full-Client-Config/#tls)

## Xray nodes and account routing

DIRECT is built in; native SOCKS5 does not require Xray. Add VLESS or HTTP nodes in Outbound Nodes, install the pinned Xray kernel when needed, supply credentials and required CA, validate and apply. Each adapted node has its own loopback SOCKS5 endpoint. Do not expose these endpoints publicly. HTTP CONNECT is not UDP support.

Select an enabled node in the account drawer and save. In strict mode every effective proxy user needs an explicit binding, including DIRECT. An invalid/unavailable binding is not silently replaced. Referenced nodes cannot be removed/disabled. Test TCP and UDP independently; a recent test is not a live health guarantee. Historical unidentified HY2 rules remain in advanced settings; RC.23 selector safeguards are retained.

## Update, backup and recovery

Use the target release’s version-pinned installer when the local updater is older. It delegates to the verified target updater. The new updater verifies release assets before stopping the service. A matching kernel hash permits a panel-only upgrade. A changed hash on a running standard v2 installation triggers verification of the installed source pair and a coordinated maintenance upgrade. On v2, `sudo hui update` selects the highest available stable v2 release. The legacy `upgrade` command remains. v1 installations with their original kernel require a separate v2 installation or an independently verified migration.

Upgrade snapshots under `/var/lib/h-ui/backups/upgrade-*` contain a SQLite-consistent database copy, previous binary, systemd unit, HY2 YAML, and relevant Xray config/CA files, with ownership metadata. Five newest snapshots are retained. Startup/HTTP/frontend failure restores the snapshot and prior running state; an unsuccessful rollback is reported explicitly. Existing configuration, bindings and certificate paths are retained.

Before upgrading, additionally keep a private off-host copy of `/usr/local/h-ui`, `/var/lib/h-ui` and the service unit, preserving permissions; stop services for a consistent raw file copy, or use SQLite-aware backup. Do not publish the backup: it contains credentials. For manual recovery, stop `h-ui.service`, preserve the failed state separately, inspect the snapshot `manifest.json`, restore each mapped file to its original path with recorded permissions/ownership, reload systemd, then restore the previous service state. Database rollback discards changes made after the snapshot. The symlink continues to point at the restored binary; for an old version lacking `update`, use `hui upgrade`.

## Explicit migration from v1.0.0 to a v2 release

This is for the official v1.0.0 panel and official v2.13.0-hui.4 kernel on Linux AMD64. It requires the active, standard root-run h-ui.service on port 8000, standard data directories and no systemd overrides. Keep an independent backup before starting.

```bash
installer=$(mktemp) &&
curl -fsSL https://github.com/GlareGlimmering/better-hy2/releases/download/v2.0.1/install.sh -o "$installer" &&
sudo bash "$installer" --version v2.0.1 --migrate-v1
```

The installer stages and checks both binaries and prepares a recovery snapshot while the old service runs. The outage starts when h-ui.service and its HY2 child stop. During the outage it refreshes the SQLite backup, activates the pair, and starts the panel, which migrates the database and managed HY2 configuration. A failed start or health check restores the v1 executables, SQLite and configuration and restarts the original unit. Previously committed usage is preserved. Usage still in the old kernel's in-memory counters at shutdown may be lost. If power loss or a forced kill interrupts the operation, use the printed snapshot and the manual recovery steps above before retrying.

Sign in with the old administrator username. If the password was shared with the proxy credential, complete the required password change. If login recovery is needed, run `sudo hui reset` locally. Users still using the former administrator credential as a proxy credential should switch to an ordinary account before migration. After migration, test one new real HY2 connection and refresh its existing subscription.

## Uninstall

Run `sudo hui uninstall` (or `sudo /usr/local/h-ui/h-ui uninstall`). Review the removal plan and type `UNINSTALL`. Press Enter to cancel. The command stops and removes the standard panel service and its managed HY2 child, deletes the matching hui symlink and the standard installation tree, and removes upgrade backups and HY2 access logs including rotated files. The older `--purge` spelling is equivalent and asks for `PURGE`.

The standard Better HY2 Xray unit, runtime files and dedicated service account are also removed when their ownership checks pass. Independent or modified services and their associated files are preserved. External certificate paths, shared system journals, external backups, and Cloudflare services are outside the uninstall scope. Filesystem removal does not overwrite disk blocks. Save a private backup elsewhere before confirming if you need the data later. If the command reports an incomplete uninstall, correct the error and rerun it to review remaining items.

New account deletions use SQLite secure deletion. Existing backup snapshots, historical logs, and earlier free-space or journal copies require separate retention decisions.

## Troubleshooting

```bash
systemctl status h-ui --no-pager
journalctl -u h-ui -n 100 --no-pager
curl -fsS http://127.0.0.1:8000/
sudo /usr/local/h-ui/h-ui version
```

Check the JavaScript entry in returned HTML if the UI is blank. A healthy panel does not prove HY2 or an outbound works. Read `/usr/local/h-ui/logs/h-ui.log`, `hysteria2.log`, and `/var/lib/hui/hy2_access.log`; access events require actual proxy requests. Review authentication reason codes without sharing secrets. For ACME, inspect the real fatal error rather than disabling certificate validation. A refused node or missing binding must remain fail-closed.

## Release verification

Download `h-ui-linux-amd64`, `hysteria-linux-amd64`, `HY2-PAIR.json`, `install.sh` and `SHA256SUMS` from the same exact v2 release; run `sha256sum --check SHA256SUMS` in that directory. Verify `./h-ui-linux-amd64 version` matches the tag. No private server is contacted by CI. Local fixture tests cannot replace real systemd/reverse-proxy/UDP operator acceptance.
