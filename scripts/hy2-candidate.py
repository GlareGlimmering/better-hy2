#!/usr/bin/env python3
"""Prepare and test a Better HY2 kernel candidate in a separate Linux directory."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
PIN = ROOT / "model/constant/hy2-pair.json"
UPSTREAM = "https://github.com/HyNetworks/hysteria.git"
CUSTOM = "https://github.com/GlareGlimmering/hysteria.git"
VERSION = re.compile(r"v2\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)")
COMMIT = re.compile(r"[0-9a-f]{40}")
CAPABILITIES = {"accountConsistency": 2, "accountBandwidth": 1, "acmeDNSWorker": 1}


def capture(args, directory, **kwargs):
    return subprocess.run(args, cwd=directory, text=True, check=True, stdout=subprocess.PIPE, **kwargs).stdout.strip()


def git(directory, *args, **kwargs):
    return capture(["git", *args], directory, **kwargs)


def write_json(path, data):
    path.write_text(json.dumps(data, indent=2) + "\n", encoding="utf-8")


def require_ancestor(source, older, newer, message):
    if subprocess.run(["git", "merge-base", "--is-ancestor", older, newer], cwd=source).returncode != 0:
        raise ValueError(message)


def prepare(args):
    approved = json.loads(PIN.read_text(encoding="utf-8"))
    target = args.tag or approved["upstreamTag"]
    version = args.version or (approved["version"] if target == approved["upstreamTag"] else f"{target}-hui.1-dev")
    if not VERSION.fullmatch(target) or not re.fullmatch(re.escape(target) + r"-hui\.[1-9][0-9]*(?:-dev)?", version):
        raise ValueError("expected an official v2.x.y tag and a matching v2.x.y-hui.N[-dev] candidate version")
    base = approved["commit"]
    if not all(COMMIT.fullmatch(sha) for sha in [base, approved["upstreamCommit"], args.candidate or base]):
        raise ValueError("expected full commit SHAs for the approved pin and any reviewed candidate")
    work = args.work.resolve()
    work.mkdir(parents=True)
    source, report = work / "source", work / "report"
    source.mkdir()
    report.mkdir()
    git(source, "init", "-q")
    git(source, "fetch", "--no-tags", "-q", UPSTREAM, f"refs/tags/app/{target}:refs/tags/app/{target}", approved["upstreamCommit"])
    upstream = git(source, "rev-parse", f"refs/tags/app/{target}^{{commit}}")
    if target == approved["upstreamTag"] and upstream != approved["upstreamCommit"]:
        raise ValueError("the official tag no longer matches the approved upstream commit")
    fork = str(args.fork_dir.resolve()) if args.fork_dir else CUSTOM
    git(source, "fetch", "--no-tags", "-q", fork, base, args.candidate or base)
    require_ancestor(source, approved["upstreamCommit"], upstream, "target tag must descend from the approved upstream base")
    require_ancestor(source, approved["upstreamCommit"], base, "approved fork does not contain its pinned upstream commit")
    info = {"upstreamTag": target, "upstreamCommit": upstream, "approvedForkCommit": base, "version": version}

    if args.candidate:
        require_ancestor(source, upstream, args.candidate, "reviewed candidate does not contain the target upstream tag")
        require_ancestor(source, base, args.candidate, "reviewed candidate does not contain the approved Better HY2 fork")
        commit = args.candidate
    elif subprocess.run(["git", "merge-base", "--is-ancestor", upstream, base], cwd=source).returncode == 0:
        commit = base
    else:
        merge = subprocess.run(["git", "merge-tree", "--write-tree", "--name-only", base, upstream],
                               cwd=source, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        (report / "merge.txt").write_text(merge.stdout, encoding="utf-8")
        if merge.returncode == 1:
            write_json(report / "source.json", {**info, "status": "conflict"})
            print(merge.stdout, file=sys.stderr, end="")
            print("Merge the target into the fork, review conflicts, then rerun with --candidate <fork SHA>.", file=sys.stderr)
            return 2
        merge.check_returncode()
        tree = merge.stdout.splitlines()[0]
        timestamp = str(max(int(git(source, "show", "-s", "--format=%ct", sha)) for sha in (base, upstream)))
        env = dict(os.environ, GIT_AUTHOR_NAME="Better HY2", GIT_COMMITTER_NAME="Better HY2",
                   GIT_AUTHOR_EMAIL="kernel-candidate@better-hy2.invalid", GIT_COMMITTER_EMAIL="kernel-candidate@better-hy2.invalid",
                   GIT_AUTHOR_DATE=f"@{timestamp} +0000", GIT_COMMITTER_DATE=f"@{timestamp} +0000")
        commit = git(source, "commit-tree", tree, "-p", base, "-p", upstream,
                     "-m", f"Integrate Hysteria {target} into Better HY2", env=env)

    git(source, "checkout", "--detach", "-q", commit)
    with (report / "candidate.patch").open("wb") as patch:
        subprocess.run(["git", "diff", "--binary", "--no-ext-diff", upstream, commit, "--"], cwd=source, stdout=patch, check=True)
    write_json(report / "source.json", {**info, "commit": commit, "status": "prepared"})
    print(f"{version}: {commit} (upstream {upstream})")
    return 0


def verify(args):
    work = args.work.resolve()
    source, report = work / "source", work / "report"
    info = json.loads((report / "source.json").read_text(encoding="utf-8"))
    if (info["status"] != "prepared" or git(source, "rev-parse", "HEAD") != info["commit"]
            or git(source, "status", "--porcelain")):
        raise ValueError("candidate must be a clean checkout of the prepared commit")
    staging = work / "staging"
    staging.mkdir()
    binary = staging / "hysteria-linux-amd64"
    version, commit = info["version"], info["commit"]
    compiler = capture(["go", "env", "GOVERSION"], source)
    venv = work / "python"
    subprocess.run(["uv", "sync", "--locked"], cwd=source,
                   env=dict(os.environ, UV_PROJECT_ENVIRONMENT=str(venv), UV_COMPILE_BYTECODE="1"), check=True)
    env = dict(os.environ, GOOS="linux", GOARCH="amd64", CGO_ENABLED="0",
               PATH=str(venv / "bin") + os.pathsep + os.environ["PATH"])
    module = "github.com/apernet/hysteria/app/v2/cmd"
    subprocess.run(["go", "build", "-buildvcs=false", "-trimpath",
                    f"-ldflags=-X {module}.appVersion={version} -X {module}.appCommit={commit}",
                    "-o", binary, "./app"], cwd=source, env=env, check=True)
    reported = json.loads(capture([binary, "version", "--json"], source))
    expected = {"version": version, "commit": commit, "os": "linux", "arch": "amd64", **CAPABILITIES}
    if any(type(reported.get(key)) is not type(value) or reported[key] != value for key, value in expected.items()):
        raise ValueError("candidate binary is missing required Better HY2 capabilities or build identity")
    packages = ["./app/...", "./core/...", "./extras/..."]
    subprocess.run(["go", "test", "-count=1", "-timeout=5m", *packages], cwd=source, env=env, check=True)
    subprocess.run(["go", "vet", *packages], cwd=source, env=env, check=True)
    subprocess.run(["go", "test", "-race", "-count=1", "-timeout=5m",
                    "./core/server", "./core/internal/integration_tests", "./extras/trafficlogger"],
                   cwd=source, env=dict(env, CGO_ENABLED="1"), check=True)
    with binary.open("rb") as stream:
        digest = hashlib.file_digest(stream, "sha256").hexdigest()
    (staging / "SHA256SUMS").write_text(f"{digest}  {binary.name}\n", encoding="utf-8")
    write_json(staging / "candidate.json", {**info, **expected, "status": "kernel-tests-passed", "goVersion": compiler, "sha256": digest})
    staging.rename(work / "verified")
    print(f"Kernel tests passed: {digest}  {binary.name}")
    return 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    command = commands.add_parser("prepare")
    command.add_argument("work", type=Path, help="a new work directory")
    command.add_argument("--tag", help="official tag without app/ (defaults to the approved base)")
    command.add_argument("--candidate", help="full SHA of a reviewed merge pushed to the Better HY2 kernel fork")
    command.add_argument("--fork-dir", type=Path, help="use a local kernel Git worktree to test a commit before pushing")
    command.add_argument("--version", help="candidate v2.x.y-hui.N[-dev] version")
    verify_command = commands.add_parser("verify")
    verify_command.add_argument("work", type=Path, help="work directory created by prepare")
    args = parser.parse_args()
    try:
        return (prepare if args.command == "prepare" else verify)(args)
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        print(f"kernel candidate: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
