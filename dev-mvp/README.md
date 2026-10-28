# Local MVP stack

Runs the theliv API from a file config with no cluster, and a local trace and metrics stack for the agent.

    docker-compose -f dev-mvp/docker-compose.yaml up -d
    bash dev-mvp/verify.sh

`verify.sh` builds the API, serves the mock report, runs `agent/` against Ollama with traces to the collector, then checks Tempo for the trace and Prometheus for the theliv scrape. Grafana is at http://127.0.0.1:3000 with Tempo and Prometheus provisioned. The agent needs Ollama on 127.0.0.1:11434 with the model named in `agent/README.md`.
