"""Write the same paired artifact manifest for release and recovery CI."""
import hashlib
import json
from pathlib import Path
import sys

directory, panel_version, panel_sha, kernel_version, kernel_sha = sys.argv[1:]
directory = Path(directory)


def artifact(filename, version, commit):
    with (directory / filename).open("rb") as binary:
        digest = hashlib.file_digest(binary, "sha256").hexdigest()
    return {"version": version, "commit": commit, "sha256": digest}


pair = {
    "panel": artifact("h-ui-linux-amd64", panel_version, panel_sha),
    "kernel": artifact("hysteria-linux-amd64", kernel_version, kernel_sha),
    "os": "linux",
    "arch": "amd64",
    "accountConsistency": 2,
    "accountBandwidth": 1,
}
(directory / "HY2-PAIR.json").write_text(json.dumps(pair, indent=2) + "\n")
