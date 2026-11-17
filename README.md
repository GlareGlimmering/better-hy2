# Better HY2

Better HY2 manages a Hysteria2 server, proxy accounts, and outbound nodes from a web panel. A single VPS can serve the panel and subscriptions through a Cloudflare Tunnel while HY2 clients connect directly over UDP.

[中文](README_ZH.md) · [Download v2.1.0](https://github.com/GlareGlimmering/better-hy2/releases/tag/v2.1.0) · [Operations guide](docs/OPERATIONS.md)

## What we added

- **Proxy accounts and subscriptions.** Set expiry, concurrent connection limits, GB quotas and shared per-account upload/download limits in Mbps. View traffic in the dashboard and customize subscription node names and addresses. The default node address uses the server's public IPv4.
- **Outbound nodes.** Assign each account to DIRECT, native SOCKS5, or VLESS and HTTP nodes backed by Xray. Strict routing rejects missing or failed routes. Test TCP and UDP separately.
- **Panel security.** Administrator credentials are separate from HY2 accounts. The panel uses salted Argon2id, login rate limits, trusted-proxy configuration and session revocation on password change. HY2 authentication uses a private loopback listener. Subscription links remain usable and their responses request no caching.
- **Operations.** Manage HY2, configure TLS, inspect system and access logs, and use CLI installation, checked updates, backups and uninstall. The panel ships with its tested kernel and a compatibility manifest.

## Install v2.1.0

Use a **Linux AMD64** host with **systemd**, Bash, curl, coreutils and root privileges. The installer checks the panel version and checksums and installs the paired kernel. On a fresh installation it prints a random panel login once. Xray is optional.

```bash
installer=$(mktemp) &&
curl -fsSL https://github.com/GlareGlimmering/better-hy2/releases/download/v2.1.0/install.sh -o "$installer" &&
sudo bash "$installer" --version v2.1.0
```

The standard panel listens on `http://127.0.0.1:8000`. The installer leaves the firewall and proxy configuration to you.

## Recommended setup: Cloudflare Tunnel + your own HY2 certificate

| Connection | Public endpoint | Origin |
| --- | --- | --- |
| Panel and subscriptions | `https://panel.example.com` | Cloudflare Tunnel → `http://127.0.0.1:8000` |
| HY2 client traffic | VPS public IP or a DNS-only hostname, UDP `443` | Hysteria2 listener |
| HY2 authentication callback | Local only | Dedicated loopback listener managed by Better HY2 |

1. Run `cloudflared` on the same VPS and publish an HTTP application for `panel.example.com` targeting `http://127.0.0.1:8000`. Cloudflare supplies the public HTTPS certificate. Configure the panel hostname to bypass cache if a rule would otherwise cache subscriptions. Omit full subscription URLs from proxy logs. [Cloudflare published applications](https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/routing-to-tunnel/) · [Proxy settings](docs/OPERATIONS.md#reverse-proxy-subscriptions-and-login-limits)
2. In **HY2 Management**, set the listen address to `:443` and enable **TLS**. Provide the HY2 certificate and key using the form. A self-signed private CA can issue the certificate. Its SAN should match the HY2 SNI, for example `hy2.example.com`. Open **UDP 443** on the VPS and its provider firewall. The public HTTP tunnel handles panel and subscription traffic, while HY2 clients use the independent UDP listener. [Certificate generation](docs/OPERATIONS.md#generate-a-private-hy2-certificate)
3. Create an ordinary proxy account, choose **DIRECT** for the first test, and start HY2. In strict routing mode every effective account needs an explicit binding. Copy the subscription URL or use the native HY2 client example below. A client using a private CA needs that CA and the matching SNI configured locally. Use a publicly trusted HY2 certificate if your client cannot import a private CA or verify a certificate pin. [Hysteria client TLS options](https://v2.hysteria.network/docs/advanced/Full-Client-Config/#tls)

```yaml
server: 198.51.100.10:443
auth: "paste-the-full-proxy-connection-credential"
tls:
  sni: hy2.example.com
  ca: ./hy2-ca.crt
  insecure: false
socks5:
  listen: 127.0.0.1:1080
```

Use your own public IP and install only the public CA certificate on clients. The panel administrator signs in to the web interface. Proxy users use the separate connection credential.

## Common commands

| Task | Command |
| --- | --- |
| Show the panel version | `hui version` |
| Show the installed kernel and capabilities | `/usr/local/h-ui/bin/hysteria-linux-amd64 version --json` |
| Recover administrator access | `sudo hui reset` |
| Update an installed v2 release within the stable v2 series | `sudo hui update` |
| Choose an exact v2 version | `sudo hui update v2.1.0` |
| Check the panel service | `systemctl status h-ui.service --no-pager` |
| Follow service logs | `sudo journalctl -u h-ui.service -f` |
| Follow HY2 request logs | `sudo tail -F /var/lib/hui/hy2_access.log` |
| Restart the panel and its managed child process | `sudo systemctl restart h-ui.service` |
| Uninstall and remove standard installation data | `sudo hui uninstall` |
| Uninstall with the legacy confirmation word | `sudo hui uninstall --purge` |

Use **HY2 Management** to start, stop or restart the HY2 child only. The uninstall command lists the paths and asks for confirmation before making changes.

## Upgrade to v2.1.0

- **Already on v2 with the matching kernel**, including v2.0.2-rc.4: Run `sudo hui update v2.1.0`. The updater checks the installed kernel's SHA256 before restarting the service. A mismatch stops the update before service downtime.
- **Official v1.0.0 with v2.13.0-hui.4:** Use the explicit [v1 maintenance-window migration](docs/OPERATIONS.md#explicit-migration-from-v100-to-a-v2-release). The legacy GitHub Latest pointer remains on v1 so its old updater selects a compatible release.
- **Moving to a lower major version:** The current updater and installer reject this before downloading the candidate or stopping the service. Recovery from a failed activation uses the matching executable and database snapshot. Old installers and manual file replacement run outside this guard.

v2.1.0 pairs with the exact `v2.13.0-hui.6-dev` build in the Release (Traffic Stats protocol 2 and bandwidth capability 1). The kernel version retains the suffix used by the tested RCs. Keep older panel binaries paired with their pre-upgrade databases when rolling back. [Backup and recovery](docs/OPERATIONS.md#update-backup-and-recovery)

## Build and credits

Use the Go version in `go.mod`, Node 22 and the pnpm version in `frontend/package.json`. Build the frontend first so the Go executable embeds it.

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm run test:unit
pnpm run build:prod
cd ..
go test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-X h-ui/model/constant.Version=v2.1.0" -o h-ui-linux-amd64 .
```

Based on [H UI](https://github.com/jonssonyan/h-ui), with [Hysteria](https://github.com/apernet/hysteria) and [Xray-core](https://github.com/XTLS/Xray-core). [GPL-3.0](LICENSE) · [Third-party notices](THIRD_PARTY_NOTICES.md) · [Acceptance records](docs/ACCEPTANCE_ZH.md)
