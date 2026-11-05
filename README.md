# Better HY2

A lightweight, modern management panel for Hysteria2.

[中文](README_ZH.md) · [Releases](https://github.com/GlareGlimmering/better-hy2/releases) · [Installation and operations](docs/OPERATIONS.md)

## Features

- Hysteria2 server lifecycle and configuration management.
- DIRECT, native SOCKS5 and Xray-adapted VLESS/HTTP outbound nodes.
- Per-account outbound routing with explicit bindings and fail-closed validation.
- Traffic monitoring, quotas, expiry and device limits.
- Subscription templates and automatic public IPv4 node addresses.
- Responsive dashboard, account drawer and English/Chinese/Russian UI.
- System, HY2 kernel and request access logs with rotation and management.
- SHA256/version-verified updates, database backups and automatic rollback on startup/health-check failure.

## Requirements

Linux AMD64, systemd and root installation privileges. Ubuntu/Debian-style systemd installations are the supported deployment target. Other distributions and container installations are not certified by this release. The installer needs Bash, curl, coreutils and network access to GitHub and the pinned kernel release.

## Quick install

```bash
curl -fsSL https://raw.githubusercontent.com/GlareGlimmering/better-hy2/main/install.sh | sudo bash
```

The script resolves the latest stable release, downloads its exact assets and verifies the binary SHA256 and version. It installs the pinned custom HY2 kernel and initializes a random administrator. Credentials are printed once: record them securely. Xray is optional and is not downloaded on a fresh/DIRECT-only installation.

The panel listens on **http://127.0.0.1:8000**. Expose it through an HTTPS reverse proxy or Cloudflare Tunnel targeting that loopback endpoint; the installer does not open firewall ports or configure a reverse proxy. HY2 UDP listening is configured separately in the panel.

## Update

```bash
sudo hui update
```

The default channel installs stable releases only and refuses automatic downgrades. The legacy entry remains supported:

```bash
sudo /usr/local/h-ui/h-ui upgrade
```

## Migrate RC.23

RC.23 cannot parse the new stable tag. **Use the new installer, not the old binary's `upgrade v1.0.0`:**

```bash
curl -fsSL https://raw.githubusercontent.com/GlareGlimmering/better-hy2/main/install.sh | sudo bash -s -- --version v1.0.0
```

It verifies the new candidate and delegates to that candidate's compatible upgrader. Existing accounts, bindings, certificates and configuration paths remain unchanged. Back up first; service restart briefly interrupts panel and HY2 connections. Xray is not restarted by the panel upgrader. See [operations and recovery](docs/OPERATIONS.md).

## Documentation

[Installation, first-time setup, HY2 configuration, Xray outbounds, account routing, backup/recovery and troubleshooting](docs/OPERATIONS.md).

The internal Go module, binary filename, database and service names retain `h-ui` for compatibility. This project does not change the Hysteria2/Xray proxy protocols. The managed HY2 kernel is **v2.13.0-hui.4**; it is independently released at [GlareGlimmering/hysteria](https://github.com/GlareGlimmering/hysteria). Do not replace it with an incompatible upstream binary when managed user routing is enabled.

## Validation and limitations

The RC.23 baseline and existing proxy chains passed operator VPS acceptance before this release. Automated tests exercise configuration, account routing constraints, installer/upgrader, rollback, frontend and embedded assets. These checks do not constitute a production security audit or prove zero network leakage under all failures. UDP end-to-end support depends on the selected outbound and upstream; a TCP result does not validate UDP. The renamed stable installer and upgrade should still receive operator acceptance before broad rollout. No automated VPS deployment is performed.

## Build

Use the Go version from `go.mod`, Node 22 and pnpm from `frontend/package.json`:

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm run test:unit
pnpm run build:prod
cd ..
go test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -X h-ui/model/constant.Version=v1.0.0" -o h-ui-linux-amd64 .
```

Build the frontend first: Go embeds `frontend/dist`, including underscore-prefixed chunks. The release workflow verifies all embedded asset references.

## Credits and license

Derived from [H UI by jonssonyan](https://github.com/jonssonyan/h-ui). Thanks to [Hysteria](https://github.com/apernet/hysteria), [Xray-core](https://github.com/XTLS/Xray-core) and all upstream contributors. Original copyright and license notices remain applicable; see [third-party notices](THIRD_PARTY_NOTICES.md).

Better HY2 is distributed under [GPL-3.0](LICENSE). Source is supplied for the exact release tag. Independently downloaded proxy kernels retain their own licenses.
