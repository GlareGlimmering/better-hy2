# Better HY2 Repository Instructions

## Working directory

- Treat `D:\User\documents\Office\Codex\h-ui` as the only active H-UI repository.
- Public repository: `GlareGlimmering/better-hy2`. Product: Better HY2. Preserve the internal `h-ui` module, binary/data paths and systemd names for compatibility.
- Preserve `main` and the stable release tags as rollback points unless the user explicitly requests changing them.
- Develop each iteration on an appropriate non-`main` development or feature branch.

## Public wording

- Describe the project as research into Linux multi-hop data transfer and which initial network fingerprint characteristics survive each hop.
- In new public documentation, commit messages, and release notes, use neutral research terminology and describe the actual changes accurately. Omit references to proxying, firewall circumvention, Xray, and HY2 where writing those public summaries.

## Implementation and acceptance

- The implementation Codex writes code, fixes bugs, and runs the relevant automated checks. The operator and review assistant review changes, propose features, and conduct acceptance testing. The operator relays instructions and results between sessions.
- Use [the acceptance record](docs/ACCEPTANCE_ZH.md) when resuming this work. Record the exact versions tested, reproducible steps, observed results, and open cases. Keep implementation expectations, CI evidence, and manual observations distinct.
- Before handing off a feature or fix for acceptance, identify its commit and any paired kernel revision, outline the shortest real-client test, and report checks run and known limitations. Scrub passwords, tokens, subscription URLs, and production details from shared notes.

## Required delivery workflow

1. When the change meets the request, reuse checks already passed for the same commit and run focused checks for affected changes. Before committing, quickly review the diff and file list for unrelated changes, secrets, and generated artifacts, and run `git diff --check`.
2. Commit and push the change to the development branch directly. Verify the remote branch SHA.
3. When a complete batch reaches an independent acceptance point, create and push the next unused annotated RC tag and follow the existing release CI through publication. Let that CI run its configured build and checks once. Run additional installation, upgrade, or rollback checks only when the changes touch those paths or CI reports a failure. Investigate failures in the affected area.
4. Confirm the GitHub prerelease and required download assets are available. Reuse CI's artifact checksums. Routine releases require no fresh download or checksum pass after publication. Report the release and CI links, version-pinned installation or upgrade commands, and the paired kernel version when applicable.

Release notes include the changes, checks, and known limitations. Panel and kernel versions are managed separately. Stable releases require an explicit request. Preserve existing tags and releases. Resolve failed checks or push errors before proceeding.

## WSL2 test workflow

- Use the Ubuntu WSL2 checkout at `/mnt/d/User/documents/Office/Codex/h-ui` for Go tests and Linux build verification. It is the same working tree as `D:\User\documents\Office\Codex\h-ui`, not a second repository.
- Keep the shared repository's local Git setting `core.autocrlf=true`. This matches the Windows checkout while `.gitattributes` keeps shell scripts at LF, and prevents WSL Git from reporting line-ending-only changes across the repository. Never bulk-normalize or rewrite files merely to clear WSL status.
- Run Go checks from WSL, for example:

  ```bash
  wsl.exe -d Ubuntu -- bash -lc 'cd /mnt/d/User/documents/Office/Codex/h-ui && go test ./...'
  ```

- For test-initialization or import-graph investigations, clear only the Go test result cache when needed:

  ```bash
  wsl.exe -d Ubuntu -- bash -lc 'cd /mnt/d/User/documents/Office/Codex/h-ui && go clean -testcache && go test ./...'
  ```

- Prefer focused WSL package tests during development, then run `go test ./...` before a complete backend iteration. Use the GitHub Actions run, which reads the minimum Go version from `go.mod`, as the final Linux compatibility check.
- Do not run native Windows `go test` for this repository by default. The pinned `modernc.org/sqlite` generated sources are known not to compile correctly in the native Windows environment used by Codex. Use WSL instead; do not upgrade or replace the SQLite driver merely to make native Windows tests pass.

## Permission boundaries

Without a new explicit user request, do not:

- merge or push changes directly to `main`;
- create, delete, or move stable version tags, or publish a stable Release;
- force-push or otherwise rewrite remote history;
- deploy to production or connect to/update the production VPS;
- modify the running H-UI, Hysteria2, or Cloudflare Tunnel services.

Never bypass a failed push, branch protection, test failure, or conflict with `--force` or destructive Git operations.
