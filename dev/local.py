#!/usr/bin/env python3
"""Build, start, verify and stop the isolated Theliv demo on macOS."""
import argparse
import json
import os
import secrets
import shutil
import signal
import socket
import subprocess
import sys
import time
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

ROOT = Path(__file__).resolve().parents[1]
STATE = ROOT / ".local"
PROFILE = "theliv"
CONTEXT = "colima-theliv"
CLUSTER = "rks-local"  # Upstream's etcd loader requires an [era]ks cluster name.
ETCD_IMAGE = "gcr.io/etcd-development/etcd:v3.6.10"
ETCD_CONTAINER = "theliv-etcd"
LABEL = "dev.theliv.workspace"
CHART_VERSION = "29.35.0"
NODE_IMAGE = "node:14.21.3-bullseye"
LOCAL_ROLE_FILE = ROOT / "dev/local-role.json"


def run(args, *, capture=False, input=None, check=True, env=None):
    result = subprocess.run([str(arg) for arg in args], cwd=ROOT, text=True,
                            input=input, capture_output=capture, env=env)
    if check and result.returncode:
        if capture and result.stderr:
            print(result.stderr.strip(), file=sys.stderr)
        # Do not echo commands: etcd key paths can contain access keys.
        raise RuntimeError(f"{args[0]} failed (exit {result.returncode})")
    return result.stdout.strip() if capture else result


def docker(*args, **kwargs):
    return run(["docker", "--context", CONTEXT, *args], **kwargs)


def kube(*args, **kwargs):
    return run(["kubectl", "--context", CONTEXT, *args], **kwargs)


def state_dir():
    STATE.mkdir(exist_ok=True, mode=0o700)
    STATE.chmod(0o700)
    (STATE / "logs").mkdir(exist_ok=True)


def doctor():
    missing = [cmd for cmd in ("colima", "docker", "go", "kubectl", "helm", "openssl", "brew")
               if not shutil.which(cmd)]
    print("Missing tools: " + (", ".join(missing) or "none"))
    network = True
    for host in ("github.com", "proxy.golang.org", "registry.npmjs.org"):
        try:
            socket.getaddrinfo(host, 443)
        except OSError:
            network = False
            print(f"DNS/network unavailable: {host}")
    if shutil.which("colima"):
        run(["colima", "list"])
    if shutil.which("docker"):
        result = subprocess.run(["docker", "--context", CONTEXT, "info"],
                                text=True, capture_output=True)
        print("Theliv Docker context: " + ("available" if result.returncode == 0 else "not running/accessible"))
    print("Target: Colima profile theliv, Kubernetes/Docker context colima-theliv")
    print("Resources: 4 CPUs, 8 GiB RAM, 40 GiB disk. UI 127.0.0.1:4200; API :8080; etcd :2379.")
    return not missing and network


def install_tools():
    for tool in ("colima", "docker", "go", "openssl"):
        if not shutil.which(tool):
            raise RuntimeError(f"Required tool missing: {tool}")
    missing = [tool for tool in ("kubectl", "helm") if not shutil.which(tool)]
    if missing:
        if not shutil.which("brew"):
            raise RuntimeError("Install kubectl and Helm before running up.")
        print("Installing missing tools: " + ", ".join(missing), flush=True)
        run(["brew", "install", *missing])


def start_cluster():
    profiles = run(["colima", "list", "--json"], capture=True)
    existing = [json.loads(line) for line in profiles.splitlines() if line.strip()]
    marker = STATE / "profile-owner"
    if any(item["name"] == PROFILE for item in existing) and not marker.exists():
        raise RuntimeError("A Colima profile named theliv already exists without this workspace's ownership marker.")
    if marker.exists() and marker.read_text().strip() != str(ROOT):
        raise RuntimeError("The Colima profile belongs to another workspace.")
    # Retain ownership even if a VM download/start fails partway through.
    marker.write_text(str(ROOT) + "\n")
    run(["colima", "start", "--profile", PROFILE, "--kubernetes", "--cpu", "4",
         "--memory", "8", "--disk", "40", "--activate=false", "--ssh-config=false",
         "--mount", str(ROOT) + ":w"])
    kube("wait", "--for=condition=Ready", "node", "--all", "--timeout=180s")
    docker("info", capture=True)


def generate_certificates():
    certs = STATE / "certs"
    certs.mkdir(exist_ok=True, mode=0o700)
    expected = [certs / f"{name}.{ext}" for name in ("ca", "server", "client") for ext in ("crt", "key")]
    if all(path.is_file() for path in expected):
        for name in ("ca", "server", "client"):
            run(["openssl", "x509", "-in", certs / f"{name}.crt", "-checkend", "86400", "-noout"], capture=True)
        return
    if any(path.exists() for path in expected):
        raise RuntimeError("Partial certificate set in .local/certs; inspect before replacing it.")
    config = ROOT / "dev/openssl.cnf"
    run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "365",
         "-subj", "/CN=theliv-local-ca", "-keyout", certs / "ca.key", "-out", certs / "ca.crt",
         "-config", config, "-extensions", "ca"], capture=True)
    for name in ("server", "client"):
        run(["openssl", "req", "-new", "-newkey", "rsa:2048", "-nodes", "-subj", f"/CN=theliv-{name}",
             "-keyout", certs / f"{name}.key", "-out", certs / f"{name}.csr", "-config", config], capture=True)
        run(["openssl", "x509", "-req", "-in", certs / f"{name}.csr", "-CA", certs / "ca.crt",
             "-CAkey", certs / "ca.key", "-CAcreateserial", "-days", "365",
             "-extfile", config, "-extensions", name, "-out", certs / f"{name}.crt"], capture=True)
    for key in certs.glob("*.key"):
        key.chmod(0o600)


def etcdctl(*args, **kwargs):
    return docker("exec", "-i", ETCD_CONTAINER, "etcdctl", "--endpoints=https://127.0.0.1:2379",
                  "--cacert=/certs/ca.crt", "--cert=/certs/client.crt", "--key=/certs/client.key",
                  *args, **kwargs)


def etcd_owned():
    result = subprocess.run(["docker", "--context", CONTEXT, "inspect", ETCD_CONTAINER], text=True, capture_output=True)
    if result.returncode:
        return False
    container = json.loads(result.stdout)[0]
    if (container["Config"].get("Labels") or {}).get(LABEL) != str(ROOT):
        raise RuntimeError("Existing theliv-etcd container belongs to another workspace.")
    return True


def start_etcd():
    if etcd_owned():
        docker("start", ETCD_CONTAINER, capture=True)
    else:
        volume = "theliv-etcd-data"
        result = subprocess.run(["docker", "--context", CONTEXT, "volume", "inspect", volume], text=True, capture_output=True)
        if result.returncode == 0 and (json.loads(result.stdout)[0].get("Labels") or {}).get(LABEL) != str(ROOT):
            raise RuntimeError("Existing etcd volume belongs to another workspace.")
        docker("volume", "create", "--label", f"{LABEL}={ROOT}", volume, capture=True)
        docker("run", "--detach", "--name", ETCD_CONTAINER, "--label", f"{LABEL}={ROOT}",
               "--publish", "127.0.0.1:2379:2379", "--volume", str(STATE / "certs") + ":/certs:ro",
               "--volume", volume + ":/data", ETCD_IMAGE, "etcd", "--name=default", "--data-dir=/data",
               "--listen-client-urls=https://0.0.0.0:2379", "--advertise-client-urls=https://127.0.0.1:2379",
               "--listen-peer-urls=http://127.0.0.1:2380", "--initial-advertise-peer-urls=http://127.0.0.1:2380",
               "--initial-cluster=default=http://127.0.0.1:2380", "--cert-file=/certs/server.crt",
               "--key-file=/certs/server.key", "--trusted-ca-file=/certs/ca.crt", "--client-cert-auth=true", capture=True)
    for _ in range(30):
        try:
            etcdctl("endpoint", "health", capture=True)
            return
        except RuntimeError:
            time.sleep(1)
    raise RuntimeError("etcd did not become healthy.")


def configure_cluster():
    kube("apply", "-f", ROOT / "dev/cluster.yaml")
    run(["helm", "upgrade", "--install", "theliv-monitoring",
         "oci://ghcr.io/prometheus-community/charts/prometheus", "--version", CHART_VERSION,
         "--kube-context", CONTEXT, "--namespace", "theliv-monitoring", "--create-namespace",
         "--values", ROOT / "dev/prometheus-values.json", "--wait", "--timeout", "10m"])
    config = json.loads(kube("config", "view", "--minify", "--flatten", "--raw", "-o", "json", capture=True))
    token = kube("-n", "theliv-system", "create", "token", "theliv-reader", "--duration=24h", capture=True)
    # Export only a read-only service account credential, never the admin key.
    read_config = {"apiVersion": "v1", "kind": "Config", "clusters": config["clusters"],
                   "users": [{"name": "theliv-reader", "user": {"token": token}}],
                   "contexts": [{"name": CLUSTER, "context": {"cluster": config["clusters"][0]["name"], "user": "theliv-reader"}}],
                   "current-context": CLUSTER}
    keyfile = STATE / "accesskey"
    if not keyfile.exists():
        keyfile.write_text(secrets.token_hex(32) + "\n")
        keyfile.chmod(0o600)
    values = {
        "/theliv/config": {"port": 8080, "loglevel": 0, "qps": 20, "burst": 40,
                           "logDriver": "k8s", "eventDriver": "k8s", "teamName": "Local demo", "developedByTeam": "Theliv"},
        "/theliv/config/authconf": {"whitelistpath": ["/theliv-api/v1/userinfo", "/theliv-api/v1/configinfo"]},
        "/theliv/config/oidc": {},
        "/theliv/config/prometheus": {"scheme": "http", "namespace": "theliv-monitoring", "name": "theliv-prometheus", "port": "80"},
        "/theliv/config/levelconf": {"managednamespaces": ["kube-system", "theliv-system", "theliv-monitoring"]},
        f"/theliv/clusters/rks/{CLUSTER}/basic": {"name": CLUSTER, "provider": "k3s"},
        "/theliv/accesskeys/" + keyfile.read_text().strip(): {"UID": "local-demo", "Displayname": "Local developer"},
    }
    for key, value in values.items():
        etcdctl("put", key, input=json.dumps(value), capture=True)
    etcdctl("put", f"/theliv/clusters/rks/{CLUSTER}/kubeconf", input=json.dumps(read_config), capture=True)
    etcdctl("put", "/theliv/uids/local-demo", input="local-demo", capture=True)
    update_local_role()


def update_local_role():
    paths = json.loads(LOCAL_ROLE_FILE.read_text())
    if not isinstance(paths, list) or not paths or any(
            not isinstance(path, str) or not path.startswith("/") or "," in path for path in paths):
        raise RuntimeError("Local role paths must be a nonempty list of absolute API-relative paths.")
    etcdctl("put", "/theliv/roles/local-demo", input=",".join(paths), capture=True)


def repair_auth():
    # RBAC data is read on each request, so this needs no rebuild or restart.
    docker("info", capture=True)
    if not etcd_owned():
        raise RuntimeError("The local etcd container does not exist. Run up first.")
    etcdctl("endpoint", "health", capture=True)
    update_local_role()
    print("Updated local role permissions. Verifying the running demo...", flush=True)
    verify()


def build():
    env = dict(os.environ, GOCACHE=str(STATE / "go-build"), GOMODCACHE=str(STATE / "go-mod"))
    run(["go", "test", "./..."], env=env)
    run(["go", "build", "-o", STATE / "theliv", "."], env=env)
    docker("run", "--rm", "--volume", str(ROOT) + ":/workspace", "--workdir", "/workspace/website",
           NODE_IMAGE, "sh", "/workspace/dev/build-ui.sh")


def stop_process(name):
    pidfile = STATE / f"{name}.pid"
    if not pidfile.exists():
        return
    pid = int(pidfile.read_text())
    command = run(["ps", "-p", str(pid), "-o", "command="], capture=True, check=False)
    expected = str(STATE / "theliv") if name == "api" else str(ROOT / "dev/gateway.py")
    if command and expected not in command:
        raise RuntimeError(f"Refusing to stop reused PID for {name}.")
    if command:
        os.kill(pid, signal.SIGTERM)
        for _ in range(50):
            try:
                os.kill(pid, 0)
            except ProcessLookupError:
                break
            time.sleep(0.1)
        else:
            raise RuntimeError(f"{name} did not exit after SIGTERM.")
    pidfile.unlink(missing_ok=True)


def launch(name, command):
    with (STATE / "logs" / f"{name}.log").open("a") as log:
        process = subprocess.Popen([str(arg) for arg in command], cwd=ROOT, stdin=subprocess.DEVNULL,
                                   stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
    (STATE / f"{name}.pid").write_text(str(process.pid))


def ensure_port_free(port):
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", port))


def fetch(path, *, gateway=False, authenticated=True):
    port = 4200 if gateway else 8080
    headers = {}
    if authenticated and not gateway:
        headers["ACCESSKEY"] = (STATE / "accesskey").read_text().strip()
    with urlopen(Request(f"http://127.0.0.1:{port}{path}", headers=headers), timeout=45) as response:
        return response.read()


def diagnosis_found(report):
    if isinstance(report, dict):
        tags = report.get("tags") or {}
        if (report.get("name") == "ContainerWaitingAsImagePullBackOff" and report.get("solutions")
                and isinstance(tags, dict) and tags.get("namespace") == "theliv-demo"
                and tags.get("pod", "").startswith("missing-image-")):
            return True
        return any(diagnosis_found(value) for value in report.values())
    return isinstance(report, list) and any(diagnosis_found(value) for value in report)


def verify():
    for _ in range(30):
        try:
            fetch("/theliv-api/v1/health", authenticated=False)
            break
        except (OSError, URLError):
            time.sleep(1)
    else:
        raise RuntimeError("API health failed; see .local/logs/api.log")
    try:
        fetch("/theliv-api/v1/clusters/", authenticated=False)
    except HTTPError as error:
        if error.code != 401:
            raise RuntimeError(f"Expected 401 without an access key, got {error.code}") from error
    else:
        raise RuntimeError("API unexpectedly accepted a request without authentication.")
    clusters = json.loads(fetch("/theliv-api/v1/clusters/"))
    if CLUSTER not in clusters:
        raise RuntimeError("The local cluster is missing from Theliv.")
    namespaces = json.loads(fetch(f"/theliv-api/v1/clusters/{CLUSTER}/namespaces/"))
    if "theliv-demo" not in namespaces:
        raise RuntimeError("The demo namespace is missing.")
    fetch("/theliv/", gateway=True)
    fetch("/theliv-api/v1/userinfo/", gateway=True)
    print("API health, authentication, cluster access, and browser proxy passed.", flush=True)
    print("Waiting up to 5 minutes for the missing-image diagnosis...", flush=True)
    deadline = time.monotonic() + 300
    while time.monotonic() < deadline:
        report = json.loads(fetch(f"/theliv-api/v1/detector/{CLUSTER}/theliv-demo/detect"))
        if diagnosis_found(report):
            (STATE / "diagnosis.json").write_text(json.dumps(report, indent=2) + "\n")
            print("PASS: Theliv returned an image-pull diagnosis with solutions.")
            return
        time.sleep(10)
    raise RuntimeError("No image-pull diagnosis; inspect theliv-monitoring pods and .local/logs/api.log.")


def up():
    state_dir()
    install_tools()
    start_cluster()
    generate_certificates()
    start_etcd()
    configure_cluster()
    stop_process("gateway")
    stop_process("api")
    ensure_port_free(8080)
    ensure_port_free(4200)
    build()
    certs = STATE / "certs"
    launch("api", [STATE / "theliv", "-listen-address", "127.0.0.1", "-ca", certs / "ca.crt",
                   "-cert", certs / "client.crt", "-key", certs / "client.key", "-endpoints", "https://127.0.0.1:2379"])
    launch("gateway", [sys.executable, ROOT / "dev/gateway.py"])
    verify()
    print("Open http://127.0.0.1:4200/theliv/ and select rks-local / theliv-demo.")


def stop():
    stop_process("gateway")
    stop_process("api")
    if etcd_owned():
        docker("stop", ETCD_CONTAINER, capture=True)
    marker = STATE / "profile-owner"
    if marker.exists() and marker.read_text().strip() == str(ROOT):
        run(["colima", "stop", "--profile", PROFILE])
    print("Local demo stopped; configuration and data retained.")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=["doctor", "up", "verify", "repair-auth", "stop"])
    args = parser.parse_args()
    os.umask(0o077)
    try:
        result = globals()[args.command.replace("-", "_")]()
        if result is False:
            raise SystemExit(1)
    except HTTPError as error:
        print(f"Theliv: HTTP {error.code} {error.reason} from {error.url}", file=sys.stderr)
        raise SystemExit(1)
    except (RuntimeError, OSError, ValueError) as error:
        print(f"Theliv: {error}", file=sys.stderr)
        raise SystemExit(1)
