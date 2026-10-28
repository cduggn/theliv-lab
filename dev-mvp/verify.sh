#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
mkdir -p .local

base=http://127.0.0.1:8080/theliv-api/v1
server_pid=""
trap '[ -n "$server_pid" ] && kill "$server_pid" 2>/dev/null || true' EXIT

go build -o .local/theliv .
.local/theliv -config dev-mvp/theliv.yaml >.local/theliv.log 2>&1 &
server_pid=$!

for _ in $(seq 1 30); do
  curl -sf "$base/health" >/dev/null && break
  sleep 1
done
curl -sf "$base/health" >/dev/null
echo "PASS theliv server healthy"

curl -sf "$base/agent/mock/report" | python3 -c '
import json, sys
got = json.load(sys.stdin)[0]["rootCause"]["name"]
assert got == "ContainerWaitingAsImagePullBackOff", got
'
echo "PASS mock report root cause is ContainerWaitingAsImagePullBackOff"

stack_up=1
(exec 3<>/dev/tcp/127.0.0.1/4318) 2>/dev/null || stack_up=0
if [ "$stack_up" = 1 ]; then
  export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318
else
  unset OTEL_EXPORTER_OTLP_ENDPOINT
fi

(cd agent && uv run theliv-agent) >.local/agent-out.txt
python3 - <<'PY'
lines = [l for l in open(".local/agent-out.txt").read().splitlines() if l.strip()]
summary = [l for l in lines if l.startswith(("findings:", "- "))]
answer = [l for l in lines if l not in summary]
assert summary, "no findings summary"
assert answer, "no model answer after the findings summary"
PY
echo "PASS agent printed findings summary and model answer"

if [ "$stack_up" = 0 ]; then
  echo "SKIP tempo and prometheus checks: run 'docker-compose -f dev-mvp/docker-compose.yaml up -d' and rerun"
  exit 0
fi

sleep 10
curl -sf 'http://127.0.0.1:3200/api/search?tags=service.name%3Dtheliv-agent' | python3 -c '
import json, sys
n = len(json.load(sys.stdin).get("traces", []))
assert n >= 1, "no theliv-agent traces in tempo"
'
echo "PASS tempo has a theliv-agent trace"

curl -sf --get 'http://127.0.0.1:9090/api/v1/query' --data-urlencode 'query=up{job="theliv"}' | python3 -c '
import json, sys
res = json.load(sys.stdin)["data"]["result"]
assert res and res[0]["value"][1] == "1", res
'
echo "PASS prometheus scrapes theliv (up=1)"
