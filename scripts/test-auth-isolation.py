"""Exercise the same public paths through a real HTTP reverse proxy."""
from http.client import HTTPConnection
from http.server import BaseHTTPRequestHandler, HTTPServer
import json
import sys
from threading import Thread
from urllib.parse import urlsplit

panel, phase = sys.argv[1:]
assert phase in {"legacy", "isolated"}
origin = urlsplit(panel)
prefix = origin.path + "/hui"
credential = "smokeuser.fixture-credential"

class ReverseProxy(BaseHTTPRequestHandler):
    def do_GET(self):
        self.forward()

    def do_POST(self):
        self.forward()

    def log_message(self, *_args):
        # The subscription URL carries the fixture credential.
        pass

    def forward(self):
        upstream = HTTPConnection(origin.hostname, origin.port, timeout=5)
        try:
            body = self.rfile.read(int(self.headers.get("Content-Length", "0")))
            upstream.request(self.command, self.path, body, {
                "User-Agent": "Clash",
                "Content-Type": "application/json",
                "X-Forwarded-For": self.client_address[0],
            })
            response = upstream.getresponse()
            data = response.read()
            self.send_response(response.status)
            for name in ("Cache-Control", "Content-Disposition"):
                if value := response.getheader(name):
                    self.send_header(name, value)
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)
        finally:
            upstream.close()

with HTTPServer(("127.0.0.1", 0), ReverseProxy) as proxy:
    thread = Thread(target=proxy.serve_forever)
    thread.start()
    try:
        conn = HTTPConnection(*proxy.server_address, timeout=5)
        conn.request("POST", prefix + "/hysteria2/auth",
                     json.dumps({"auth": credential, "addr": "127.0.0.1:30000", "tx": 0}))
        response = conn.getresponse()
        body = response.read()
        if phase == "legacy":
            assert response.status == 200 and json.loads(body)["ok"] is True
        else:
            assert response.status == 404
        conn.request("GET", prefix + "/" + credential)
        response = conn.getresponse()
        assert response.status == 200
        assert response.getheader("Content-Disposition")
        assert credential.encode() in response.read()
        if phase == "isolated":
            assert response.getheader("Cache-Control") == "no-store"
        conn.close()
    finally:
        proxy.shutdown()
        thread.join()
print(f"RC.4 reverse-proxy compatibility: {phase}")
