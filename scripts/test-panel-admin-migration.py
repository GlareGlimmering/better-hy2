"""Assertions for disposable systemd recovery using the actual RC.3 binary."""
import json
from pathlib import Path
import re
import sqlite3
import subprocess
import sys
import time
import urllib.error
import urllib.request

live, root = map(Path, sys.argv[1:3])
phase = sys.argv[3]
api = "http://127.0.0.1:8000/hui"
proxy_credential = "smokeuser.fixture-credential"

def credentials(path):
    output = path.read_text()
    return {
        key: re.search(rf"^Better HY2 Login {label}: (.+)$", output, re.M)[1]
        for key, label in (("username", "Username"), ("pass", "Password"))
    }

def call(path, data=None, token=None):
    headers = {"User-Agent": "Clash", "Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    request = urllib.request.Request(
        api + path, None if data is None else json.dumps(data).encode(), headers
    )
    # This fixture runs several independent login/change assertions from one
    # IP. Respect the production limiter between them, with a bounded retry.
    for attempt in range(3):
        try:
            with urllib.request.urlopen(request, timeout=5) as response:
                if path == "/" + proxy_credential:
                    return response.read()
                return json.load(response)
        except urllib.error.HTTPError as err:
            if err.code != 429 or not path.startswith("/auth/") or attempt == 2:
                raise
            delay = int(err.headers["Retry-After"])
            assert 1 <= delay <= 3
            err.close()
            time.sleep(delay)

def login(creds):
    return call("/auth/login", creds)

def proxy():
    return db.execute(
        "SELECT id, username, con_pass, outbound, quota, upload, download FROM account WHERE username='smokeuser'"
    ).fetchone()

def info(token):
    return call("/account/getAccountInfo", token=token)

legacy = credentials(root / "reset.log")
new_password = "New-fixture-password!2026"
with sqlite3.connect(live / "data" / "h_ui.db") as db:
    if phase == "legacy":
        (root / "old-token").write_text(login(legacy)["data"]["accessToken"])
        (live / "export" / "AccountExport-fixture.json").write_text("legacy export marker")
    elif phase == "snapshot":
        (root / "proxy.json").write_text(json.dumps(proxy()))
    elif phase == "migrated":
        assert list(proxy()) == json.loads((root / "proxy.json").read_text())
        assert (live / "export" / "AccountExport-fixture.json").read_text() == "legacy export marker"
        assert db.execute("SELECT count(*) FROM account WHERE role='admin'").fetchone() == (0,)
        assert db.execute("SELECT username, must_change FROM panel_admin").fetchone() == (legacy["username"], 1)
        assert info((root / "old-token").read_text())["type"] != "ok"
        gated = login(legacy)["data"]
        assert gated == {"passwordChangeRequired": True}
        assert login({"username": "smokeuser", "pass": proxy_credential})["type"] != "ok"
        try:
            auth = call("/hysteria2/auth", {"auth": legacy["username"] + "." + legacy["pass"], "addr": "127.0.0.1:23456", "tx": 0})
        except urllib.error.HTTPError as err:
            assert err.code == 404
        else:
            assert auth["ok"] is False
        assert proxy_credential.encode() in call("/" + proxy_credential)
        try:
            call("/auth/changePassword", {**legacy, "newPassword": new_password})
        except urllib.error.HTTPError as err:
            assert err.code == 404
        else:
            raise AssertionError("web password change is still exposed")
        subprocess.run([str(live / "h-ui"), "reset", "password", "--stdin"],
                       input=new_password + "\n", text=True, check=True, capture_output=True)
        token = login({**legacy, "pass": new_password})["data"]["accessToken"]
        assert info(token)["type"] == "ok"
        assert login(legacy)["type"] != "ok"
        assert login({**legacy, "pass": new_password})["data"]["accessToken"]
        for path in ("/account/importAccount", "/account/exportAccount"):
            try:
                call(path, {}, token)
            except urllib.error.HTTPError as err:
                assert err.code == 404
            else:
                raise AssertionError(f"removed route still exists: {path}")
        (root / "changed-token").write_text(token)
    elif phase == "reset":
        reset = credentials(root / "new-reset.log")
        assert reset["username"] == legacy["username"]
        assert info((root / "changed-token").read_text())["type"] != "ok"
        assert login(reset)["data"]["accessToken"]
        assert list(proxy()) == json.loads((root / "proxy.json").read_text())
    elif phase == "restored":
        assert login(legacy)["data"]["accessToken"]
        assert db.execute("SELECT count(*) FROM sqlite_master WHERE name='panel_admin'").fetchone() == (0,)
        assert (live / "export" / "AccountExport-fixture.json").read_text() == "legacy export marker"
    else:
        raise AssertionError(f"unknown phase: {phase}")
print(f"RC.3 panel administrator acceptance: {phase}")
