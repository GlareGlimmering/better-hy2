# Better HY2 v1.0.0

First stable Better HY2 release, based on the operator-accepted
`v0.0.25-custom.8-rc.23` source baseline.

- Better HY2 branding, SVG mark, English/Chinese documentation and Russian UI labels.
- Stable SemVer installation/update support with legacy custom-RC compatibility and automatic no-downgrade protection.
- `sudo hui update`, retaining the legacy binary, directories and services.
- Verified candidate-installer migration from RC.23; its old binary parser cannot accept `v1.0.0` directly.
- Fresh/DIRECT-only installations do not automatically download Xray.
- Retained unified outbound nodes, explicit account routing, fail-closed checks, traffic quotas, subscriptions, logs, dashboard and RC.23 ghost-node safeguards.
- The independently released HY2 kernel remains pinned to `v2.13.0-hui.4` with its existing SHA256. No proxy protocol, account, database or certificate migration.

Validation: complete Go suite; frontend unit/type/production checks; embedded asset graph; Linux AMD64 static build; installer shell tests; real empty-directory schema/admin CLI initialization with external service/network fixtures; stable/legacy version ordering; update, snapshot and rollback regressions; GitHub Actions required before publication.

Operator acceptance of RC.23 and existing proxy chains was confirmed before this release. No VPS was connected to or deployed by automation. The branded install/upgrade still needs operator rollout acceptance; local systemd/HTTP fixtures are not a real host installation. TCP tests do not prove UDP functionality. This release does not claim a comprehensive security audit or absolute zero leakage under every failure.
