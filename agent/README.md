# theliv-agent

Fetches report cards from theliv, asks an OpenAI-compatible model to explain them, and traces both calls.

    cd agent && uv sync && uv run theliv-agent

Env vars: `THELIV_URL` (http://127.0.0.1:8080), `MODEL_URL` (http://127.0.0.1:11434/v1), `MODEL` (qwen3.8:27b), `MODEL_API_KEY` (ollama), `OTEL_EXPORTER_OTLP_ENDPOINT` (unset prints spans to the console).

Pointing `MODEL_URL` at a gateway swaps the backend. The client code does not change.
