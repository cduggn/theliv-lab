# Local Theliv demo

This setup creates a dedicated Colima profile named `theliv` with 4 CPUs,
8 GiB RAM, 40 GiB disk, and k3s. Every Docker, kubectl, and Helm operation
targets `colima-theliv`. The existing default Colima profile is not used.

## Status

The local stack has previously returned an image-pull diagnosis for the demo
workload. The refreshed UI passes its development build, TypeScript checks, and
19 focused tests; live browser review is still required.

`SOURCE_REVISION` records the upstream baseline. The lab repository preserves
that history and separates the UI commit from the local setup commit.

## Start

Requires macOS, Python 3.9+, Go 1.25+, Colima, Docker CLI, OpenSSL, and network
access to GitHub, Go/npm registries, and container registries. The `up` command
installs missing kubectl and Helm using Homebrew. Colima must be permitted to
create and run its VM. The host needs enough free memory for the 8 GiB VM.

From the repository root:

```sh
python3 dev/local.py doctor
python3 dev/local.py up
```

The `up` command:

1. Starts the dedicated Colima/k3s profile without changing the active context.
2. Creates local certificates and starts a separate etcd container with client
   certificate authentication and a persistent Docker volume.
3. Installs Prometheus chart 29.35.0 and kube-state-metrics, and loads the 33
   alert expressions from upstream `alerting_rules.md`. Alerts use a 30-second
   delay for this demo; Prometheus keeps six hours of data in ephemeral storage.
4. Creates a Kubernetes account with read access to diagnostic resources and
   service-proxy access, then registers it as `rks-local` in Theliv. Upstream's
   cluster-name parser requires an `eks`, `aks`, or `rks` substring.
5. Seeds a random Theliv access key and a role limited to the local cluster's
   diagnostics and UI metadata.
6. Runs Go tests and builds the API. Builds the UI inside a Node 14.21.3
   container with npm 8.19.4. The build creates `website/package-lock.json`
   on its first successful install; subsequent builds use `npm ci`.
7. Starts the API and local browser gateway, then checks health, authentication,
   cluster access, UI delivery, and a real image-pull diagnosis with solutions.

After verification passes, open **http://127.0.0.1:4200/theliv/** and select
**rks-local → theliv-demo**. The `missing-image` deployment intentionally
references a nonexistent image tag. It stays broken so that the demo remains
available.

## Review the investigation workspace

The default page now opens the investigation workspace. Select a namespace to
review searchable findings alongside Overview, Evidence, and Resources. The
workspace uses the existing diagnostic API; no agent or change-execution backend
is required. Finding links include the cluster, namespace, and finding ID, so a
reload fetches their data again. A link is a current finding, not a saved history.

If the gateway is already running, refresh the browser after rebuilding the UI;
it serves `website/dist/theliv` directly. There is no additional port-forward.
Use the existing `up` command for the container-based rebuild/start workflow.

For a review, check the demo finding, open its related events, expand a resource,
copy a suggested command, and reload a copied finding link. Search for an absent
resource to inspect the filtered empty state. The theme switch and layout also
support small screens. Diagnostic failures and event failures have separate
retry controls; earlier results are labeled when a refresh fails.

Focused frontend state and adapter tests run without a browser:

```sh
npm --prefix website run test:investigations
```

## Local services and credentials

| Service | Address |
| --- | --- |
| Browser UI and API proxy | `http://127.0.0.1:4200` |
| Authenticated Go API | `http://127.0.0.1:8080` |
| etcd, requiring a client certificate | `https://127.0.0.1:2379` |

The gateway injects the local access key on the server side. It binds only to
loopback, restricts Host/Origin, and does not put credentials in the frontend
bundle. Secrets, logs, build caches, process IDs, and the final diagnosis are
under `.local/`, which is excluded from Git. API logs are in
`.local/logs/api.log`; gateway logs are in `.local/logs/gateway.log`.

Kubernetes tokens expire after 24 hours. Run `up` again to refresh the token and
restart the application. Certificates last one year; the setup stops before
using expired or incomplete certificates so they can be inspected first.

## Verify and stop

```sh
python3 dev/local.py verify
python3 dev/local.py stop
```

If an earlier `up` completed the build but failed with HTTP 403 on
`/theliv-api/v1/clusters/`, update the stored permissions and rerun verification:

```sh
python3 dev/local.py repair-auth
```

This updates only the existing local role. It preserves credentials and does
not rebuild the frontend or restart services. The application reads role
permissions from etcd on each request.

`stop` terminates this workspace's API and gateway, stops its etcd container,
and stops its Colima VM. It retains the VM, etcd volume, and local credentials.
Rerun `up` to restart. It refuses to take over a pre-existing `theliv` profile
or etcd container/volume without the matching ownership marker.

## Local compatibility changes

- Prometheus configuration accepts `scheme: http`; the default remains HTTPS.
  The connection to the Kubernetes API still uses its HTTPS endpoint and CA.
- The API accepts `-listen-address` so the local launcher binds to loopback.
- An empty etcd OIDC configuration disables OIDC, allowing access-key-only auth.
- A small Python gateway serves the built UI and proxies authenticated API calls,
  so this flow does not depend on the missing upstream Angular proxy file.

Node 14 is used only as an isolated builder because the project's Angular 12
version lists Node 12/14 in its historical [compatibility table](https://angular.dev/reference/versions).
This local demo does not upgrade the legacy frontend framework. The local build
has passed. Run the verification command against your own running stack.

Offline setup checks:

```sh
python3 -m unittest discover -s dev -p 'test_*.py' -v
GOCACHE="$PWD/.local/go-build" go test pkg/prometheus/address.go pkg/prometheus/address_test.go
```
