# Sourced by the disposable systemd test after the candidate starts.
exercise_udp_relay() {
  python3 - "$root" "$panel_origin$panel_path" <<'PY'
import json
from pathlib import Path
import re
import sys
import time
import urllib.request

root, origin = sys.argv[1:]
output = (Path(root) / "reset.log").read_text()
creds = {
    key: re.search(rf"^Better HY2 Login {label}: (.+)$", output, re.M)[1]
    for key, label in (("username", "Username"), ("pass", "Password"))
}

def call(path, data=None, token=None):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    req = urllib.request.Request(origin + "/hui" + path, None if data is None else json.dumps(data).encode(), headers)
    with urllib.request.urlopen(req, timeout=5) as response:
        reply = json.load(response)
    assert reply["code"] == 20000, f"{path} failed with application code {reply['code']}"
    return reply["data"]

token = call("/auth/login", creds)["accessToken"]
path = "/config/fixedUDPRelays"
assert call(path, token=token) == {"supported": True, "relays": []}
relay = {"name": "FIXTURE_RELAY", "addr": "127.0.0.1:18501"}
job = call(path, relay, token)
for _ in range(120):
    result = call("/config/applyHysteria2Config?id=" + job["id"], token=token)
    if result["state"] != "pending":
        break
    time.sleep(0.5)
assert result["state"] == "succeeded", result
assert call(path, token=token) == {"supported": True, "relays": [relay]}
print("Added and listed a fixed UDP relay through the authenticated panel API.")
PY
  systemctl stop "$unit"
  python3 - "$live/data/h_ui.db" <<'PY'
import sqlite3
import sys

with sqlite3.connect(sys.argv[1]) as db:
    db.execute("INSERT INTO account(username, con_pass, quota, expire_time, outbound, device_no) VALUES (?, ?, ?, ?, ?, ?)",
               ("relay-fixture", "relay-fixture.udp-test", 1000000000, 253370736000000, "FIXTURE_RELAY", 2))
PY
  systemctl start "$unit"
  check_panel
  check_proxy "$artifacts/hysteria-linux-amd64"
  python3 - "$artifacts/hysteria-linux-amd64" "$root" <<'PY'
import json
from pathlib import Path
import socket
import subprocess
import sys
import threading

kernel, root = sys.argv[1:]
with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as good, \
     socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as forbidden, \
     socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as probe:
    good.bind(("127.0.0.1", 18501))
    forbidden.bind(("127.0.0.1", 18502))
    client_config = Path(root) / "udp-client.json"
    client_config.write_text(json.dumps({
        "server": "127.0.0.1:18443",
        "auth": "relay-fixture.udp-test",
        "tls": {"insecure": True},
        "udpForwarding": [
            {"listen": "127.0.0.1:18503", "remote": "127.0.0.1:18501"},
            {"listen": "127.0.0.1:18504", "remote": "127.0.0.1:18502"},
        ],
    }))
    good.settimeout(12)
    def echo():
        try:
            data, origin = good.recvfrom(1024)
            good.sendto(data, origin)
        except OSError:
            pass

    thread = threading.Thread(target=echo, daemon=True)
    thread.start()
    with (Path(root) / "udp-client.log").open("w") as log:
        client = subprocess.Popen([kernel, "client", "-c", str(client_config)], stdout=log, stderr=log)
        try:
            probe.settimeout(0.4)
            for _ in range(20):
                assert client.poll() is None, (Path(root) / "udp-client.log").read_text()
                probe.sendto(b"fixed-relay-smoke", ("127.0.0.1", 18503))
                try:
                    reply = probe.recv(1024)
                except TimeoutError:
                    continue
                assert reply == b"fixed-relay-smoke"
                break
            else:
                raise AssertionError((Path(root) / "udp-client.log").read_text())
            forbidden.settimeout(1.5)
            probe.sendto(b"forbidden-target", ("127.0.0.1", 18504))
            try:
                forbidden.recv(1024)
            except TimeoutError:
                pass
            else:
                raise AssertionError("wrong endpoint received traffic")
        finally:
            client.terminate()
            client.wait(timeout=10)
print("Panel-managed UDP relay passed real HY2 traffic and denied the wrong port.")
PY
}
