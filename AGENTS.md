# H-UI Custom Repository Instructions

## Working directory

- Treat `D:\User\documents\Office\Codex\h-ui` as the only active H-UI repository.
- Preserve `main` and the stable release tags as rollback points unless the user explicitly requests changing them.
- Develop each iteration on an appropriate non-`main` development or feature branch.

## Required delivery workflow

For every completed iteration in this repository:

1. Inspect the branch, `git status`, relevant diffs, and configured remotes.
2. Run the tests, builds, static checks, and `git diff --check` appropriate to the changes.
3. Review tracked and untracked files for secrets, credentials, production data, build artifacts, caches, and unrelated changes.
4. Create a clear Git commit containing only the intended iteration changes.
5. Push the corresponding ordinary development branch to `origin` without requesting additional permission.
6. Verify that the remote branch resolves to the new local commit and report the commit SHA and branch URL.
7. Create the next unused annotated RC prerelease only after a complete feature reaches an independently testable acceptance point and CI passes. Do not create a Release for every ordinary commit.

Git commit and ordinary development-branch push are pre-authorized after successful verification. Incrementing RC prereleases are pre-authorized only for complete, independently acceptable features. Release notes must include the version, changes, test results, and known limitations. H-UI and the customized Hysteria2 kernel use separate version lines. Tags and Releases must never be replaced. Stop and report the problem instead of publishing when tests or builds fail, a merge conflict exists, authentication is unavailable, or GitHub rejects an operation.

## Development-stage verification

- Prefer focused tests for changed Go packages and directly related logic.
- For frontend changes, run the necessary type check or production build.
- For Go backend changes, run relevant tests and a Linux AMD64 build.
- Let the GitHub Actions release run perform one complete test, frontend build, embed verification, checksum, and static Linux AMD64 build.
- Do not repeat already-passing checks for unaffected code or perform a full security audit by default during rapid development.
- Still inspect the commit file list for private keys, tokens, databases, production data, and build artifacts.
- Avoid unrelated refactors, additional test frameworks, and dependency upgrades.

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
