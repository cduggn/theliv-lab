import io
import json
import os
import subprocess
import tempfile
import unittest
from email.message import Message
from pathlib import Path
from unittest.mock import Mock, patch

import gateway
import local


class GatewayTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.dist = self.root / "dist"
        self.dist.mkdir()
        (self.dist / "index.html").write_text("<app-root></app-root>")
        (self.dist / "main.js").write_text("console.log('demo')")
        (self.root / ".local").mkdir()
        (self.root / ".local/accesskey").write_text("test-server-key\n")
        for name, value in (("ROOT", self.root), ("DIST", self.dist)):
            patcher = patch.object(gateway, name, value)
            patcher.start()
            self.addCleanup(patcher.stop)

    def handler(self, path="/theliv/", headers=None, method="GET"):
        handler = object.__new__(gateway.Handler)
        handler.path = path
        handler.command = method
        handler.headers = Message()
        for key, value in {"Host": "127.0.0.1:4200", **(headers or {})}.items():
            handler.headers[key] = value
        handler.rfile = io.BytesIO()
        handler.wfile = io.BytesIO()
        for name in ("send_response", "send_header", "end_headers", "send_error"):
            setattr(handler, name, Mock())
        return handler

    def test_ui_routes_and_assets(self):
        for path, expected in (("/theliv/", "index.html"),
                               ("/theliv/main.js?version=1", "main.js"),
                               ("/theliv/kube-platform", "index.html")):
            with self.subTest(path=path):
                handler = self.handler(path)
                handler.handle_request()
                handler.send_response.assert_called_once_with(200)
                self.assertEqual(handler.wfile.getvalue(), (self.dist / expected).read_bytes())

    def test_missing_assets_do_not_return_html(self):
        handler = self.handler("/theliv/missing.js")
        handler.handle_request()
        handler.send_error.assert_called_once_with(404)

    def test_traversal_and_symlinks_cannot_expose_credentials(self):
        (self.dist / "secret").symlink_to(self.root / ".local/accesskey")
        for path in ("/theliv/../.local/accesskey", "/theliv/%2e%2e/.local/accesskey", "/theliv/secret"):
            with self.subTest(path=path):
                self.assertIsNone(gateway.static_path(path))

    def test_external_requests_cannot_use_local_credentials(self):
        for headers in ({"Host": "example.com:4200"},
                        {"Origin": "https://example.com"},
                        {"Sec-Fetch-Site": "cross-site"}):
            with self.subTest(headers=headers):
                handler = self.handler("/theliv-api/v1/clusters/", headers)
                with patch.object(handler, "proxy") as proxy:
                    handler.handle_request()
                    proxy.assert_not_called()
                self.assertEqual(handler.send_error.call_args.args[0], 403)

    def test_proxy_injects_server_key_without_forwarding_browser_credentials(self):
        handler = self.handler("/theliv-api/v1/clusters/", {"ACCESSKEY": "untrusted-key",
                               "Cookie": "sso=untrusted", "Authorization": "Bearer untrusted"})
        connection = Mock()
        response = connection.getresponse.return_value
        response.status = 200
        response.read.return_value = b'["rks-local"]'
        response.getheaders.return_value = [("Content-Type", "application/json"), ("Transfer-Encoding", "chunked")]
        with patch.object(gateway.http.client, "HTTPConnection", return_value=connection):
            handler.handle_request()
        headers = connection.request.call_args.kwargs["headers"]
        self.assertEqual(headers["ACCESSKEY"], "test-server-key")
        self.assertNotIn("Cookie", headers)
        self.assertNotIn("Authorization", headers)
        self.assertEqual(handler.wfile.getvalue(), b'["rks-local"]')
        self.assertNotIn(b"test-server-key", handler.wfile.getvalue())
        connection.close.assert_called_once()

    def test_invalid_request_lengths_are_rejected_before_reading(self):
        for size, status in (("invalid", 400), ("-1", 413), (str(2**21), 413)):
            with self.subTest(size=size):
                handler = self.handler("/theliv-api/v1/clusters/", {"Content-Length": size})
                handler.handle_request()
                handler.send_error.assert_called_once_with(status)


class LocalSetupTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.state = Path(self.directory.name)
        patcher = patch.object(local, "STATE", self.state)
        patcher.start()
        self.addCleanup(patcher.stop)

    def test_generated_certificates_validate_for_their_tls_roles_and_are_reused(self):
        local.generate_certificates()
        certs = self.state / "certs"
        original = (certs / "ca.crt").read_bytes()
        for name, purpose in (("server", "sslserver"), ("client", "sslclient")):
            result = subprocess.run(["openssl", "verify", "-purpose", purpose,
                                     "-CAfile", str(certs / "ca.crt"), str(certs / f"{name}.crt")],
                                    text=True, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual((certs / f"{name}.key").stat().st_mode & 0o777, 0o600)
        text = subprocess.check_output(["openssl", "x509", "-in", str(certs / "server.crt"), "-noout", "-text"], text=True)
        self.assertIn("DNS:localhost", text)
        self.assertIn("IP Address:127.0.0.1", text)
        local.generate_certificates()
        self.assertEqual((certs / "ca.crt").read_bytes(), original)

    def test_partial_certificates_are_not_overwritten(self):
        certs = self.state / "certs"
        certs.mkdir()
        (certs / "ca.key").write_text("existing-key")
        with self.assertRaisesRegex(RuntimeError, "Partial certificate"):
            local.generate_certificates()
        self.assertEqual((certs / "ca.key").read_text(), "existing-key")

    def test_existing_unowned_colima_profile_is_untouched(self):
        with patch.object(local, "run", return_value='{"name":"theliv"}') as run:
            with self.assertRaisesRegex(RuntimeError, "ownership marker"):
                local.start_cluster()
        self.assertEqual(run.call_count, 1)
        self.assertFalse((self.state / "profile-owner").exists())

    def test_unowned_etcd_is_untouched(self):
        result = subprocess.CompletedProcess([], 0, json.dumps([{"Config": {"Labels": None}}]), "")
        with patch.object(local.subprocess, "run", return_value=result):
            with self.assertRaisesRegex(RuntimeError, "another workspace"):
                local.etcd_owned()

    def test_reused_pid_is_not_signalled(self):
        (self.state / "api.pid").write_text("1234")
        with patch.object(local, "run", return_value="some-other-app"), patch.object(local.os, "kill") as kill:
            with self.assertRaisesRegex(RuntimeError, "reused PID"):
                local.stop_process("api")
            kill.assert_not_called()

    def test_cluster_registration_uses_service_account_not_admin_key(self):
        admin_config = {"clusters": [{"name": "colima", "cluster": {"server": "https://127.0.0.1:6443", "certificate-authority-data": "ca"}}],
                        "users": [{"name": "admin", "user": {"client-key-data": "secret-admin-key"}}]}
        def kube(*args, **kwargs):
            if args[0] == "config":
                return json.dumps(admin_config)
            if "token" in args:
                return "read-only-service-token"
        with patch.object(local, "kube", side_effect=kube), patch.object(local, "run"), patch.object(local, "etcdctl") as etcd:
            local.configure_cluster()
        stored = {call.args[1]: call.kwargs["input"] for call in etcd.call_args_list}
        config = json.loads(stored["/theliv/clusters/rks/rks-local/kubeconf"])
        self.assertEqual(config["users"], [{"name": "theliv-reader", "user": {"token": "read-only-service-token"}}])
        self.assertNotIn("secret-admin-key", json.dumps(stored))
        accesskey = self.state / "accesskey"
        self.assertEqual(len(accesskey.read_text().strip()), 64)
        self.assertEqual(accesskey.stat().st_mode & 0o777, 0o600)

    def test_diagnosis_requires_demo_pod_and_solutions(self):
        issue = {"name": "ContainerWaitingAsImagePullBackOff", "solutions": ["Fix the image tag"],
                 "tags": {"namespace": "theliv-demo", "pod": "missing-image-abc-123"}}
        self.assertTrue(local.diagnosis_found([{"resources": [{"issue": issue}]}]))
        self.assertFalse(local.diagnosis_found({**issue, "solutions": []}))
        self.assertFalse(local.diagnosis_found({**issue, "tags": {"namespace": "production", "pod": "missing-image-abc-123"}}))
        self.assertFalse(local.diagnosis_found({**issue, "tags": {"namespace": "theliv-demo", "pod": "other-broken-pod"}}))

    def test_role_seed_matches_the_policy_tested_by_the_application(self):
        with patch.object(local, "etcdctl") as etcd:
            local.update_local_role()
        paths = json.loads(local.LOCAL_ROLE_FILE.read_text())
        etcd.assert_called_once_with("put", "/theliv/roles/local-demo", input=",".join(paths), capture=True)
        self.assertTrue(all(path.startswith("/") for path in paths))

    def test_auth_repair_only_updates_the_existing_role_and_verifies(self):
        with patch.object(local, "docker"), patch.object(local, "etcd_owned", return_value=True), \
                patch.object(local, "etcdctl") as etcd, patch.object(local, "verify") as verify, \
                patch.object(local, "build") as build, patch.object(local, "start_cluster") as cluster, \
                patch.object(local, "launch") as launch:
            local.repair_auth()
        writes = [call for call in etcd.call_args_list if call.args[0] == "put"]
        self.assertEqual(len(writes), 1)
        self.assertEqual(writes[0].args[1], "/theliv/roles/local-demo")
        verify.assert_called_once()
        build.assert_not_called()
        cluster.assert_not_called()
        launch.assert_not_called()


if __name__ == "__main__":
    unittest.main()
