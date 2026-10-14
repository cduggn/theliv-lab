#!/usr/bin/env python3
"""Loopback-only static UI server and authenticated proxy for the local demo."""
import http.client
import mimetypes
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import unquote, urlsplit

ROOT = Path(__file__).resolve().parents[1]
DIST = ROOT / "website/dist/theliv"
PORT = 4200
HOSTS = {f"localhost:{PORT}", f"127.0.0.1:{PORT}"}
HOP_HEADERS = {"connection", "keep-alive", "proxy-authenticate", "proxy-authorization",
               "te", "trailer", "transfer-encoding", "upgrade", "content-length"}


def static_path(url):
    path = unquote(urlsplit(url).path)
    if not path.startswith("/theliv/"):
        return None
    target = (DIST / path[len("/theliv/"):]).resolve()
    if not target.is_relative_to(DIST.resolve()):
        return None
    if target.is_file():
        return target
    if not target.suffix:
        return DIST / "index.html"
    return None


class Handler(BaseHTTPRequestHandler):
    def handle_request(self):
        if self.headers.get("Host") not in HOSTS:
            self.send_error(403, "Local host required")
            return
        origin = self.headers.get("Origin")
        if origin and origin not in {f"http://{host}" for host in HOSTS}:
            self.send_error(403, "Local origin required")
            return
        if self.headers.get("Sec-Fetch-Site") == "cross-site":
            self.send_error(403, "Cross-site requests are disabled")
            return
        if urlsplit(self.path).path.startswith("/theliv-api/v1/"):
            self.proxy()
            return
        if self.command not in {"GET", "HEAD"}:
            self.send_error(405)
            return
        if self.path in {"/", "/theliv"}:
            self.send_response(302)
            self.send_header("Location", "/theliv/")
            self.end_headers()
            return
        target = static_path(self.path)
        if target is None or not target.is_file():
            self.send_error(404)
            return
        content = target.read_bytes()
        self.send_response(200)
        self.send_header("Content-Type", mimetypes.guess_type(str(target))[0] or "application/octet-stream")
        self.send_header("Content-Length", str(len(content)))
        self.send_header("X-Content-Type-Options", "nosniff")
        self.end_headers()
        if self.command != "HEAD":
            self.wfile.write(content)

    def proxy(self):
        try:
            size = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            self.send_error(400)
            return
        if not 0 <= size <= 1024 * 1024 or self.headers.get("Transfer-Encoding"):
            self.send_error(413)
            return
        headers = {key: value for key, value in self.headers.items()
                   if key.lower() not in HOP_HEADERS | {"host", "accesskey", "cookie", "authorization"}}
        headers["ACCESSKEY"] = (ROOT / ".local/accesskey").read_text().strip()
        connection = http.client.HTTPConnection("127.0.0.1", 8080, timeout=45)
        try:
            connection.request(self.command, self.path, body=self.rfile.read(size), headers=headers)
            response = connection.getresponse()
            body = response.read()
            self.send_response(response.status)
            for key, value in response.getheaders():
                if key.lower() not in HOP_HEADERS:
                    self.send_header(key, value)
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            if self.command != "HEAD":
                self.wfile.write(body)
        except (OSError, http.client.HTTPException):
            self.send_error(502, "Local Theliv API unavailable")
        finally:
            connection.close()

    do_GET = handle_request
    do_HEAD = handle_request
    do_POST = handle_request
    do_PUT = handle_request
    do_DELETE = handle_request
    do_OPTIONS = handle_request


if __name__ == "__main__":
    if not (DIST / "index.html").is_file():
        raise SystemExit("UI has not been built. Run python3 dev/local.py up.")
    print(f"Theliv: http://127.0.0.1:{PORT}/theliv/", flush=True)
    ThreadingHTTPServer(("127.0.0.1", PORT), Handler).serve_forever()
