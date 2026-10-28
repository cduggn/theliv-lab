import httpx
from opentelemetry import trace

from theliv_agent.model import Finding
from theliv_agent.settings import Settings

SYSTEM_PROMPT = (
    "You are a Kubernetes SRE. Explain the finding to a developer in five "
    "sentences and name the single most likely fix."
)


def render_findings(findings: list[Finding]) -> str:
    blocks = []
    for f in findings:
        lines = [
            f"issue: {f.issue}",
            f"resource: {f.resource}",
            f"description: {f.description}",
            *(f"solution: {s}" for s in f.solutions),
            *(f"command: {c}" for c in f.commands),
        ]
        blocks.append("\n".join(lines))
    return "\n\n".join(blocks)


def explain(client: httpx.Client, settings: Settings, findings: list[Finding]) -> str:
    resp = client.post(
        f"{settings.model_url}/chat/completions",
        headers={"Authorization": f"Bearer {settings.model_api_key}"},
        json={
            "model": settings.model,
            "messages": [
                {"role": "system", "content": SYSTEM_PROMPT},
                {"role": "user", "content": render_findings(findings)},
            ],
            "stream": False,
        },
        timeout=300,
    )
    resp.raise_for_status()
    body = resp.json()
    span = trace.get_current_span()
    span.set_attribute("llm.model", settings.model)
    usage = body.get("usage")
    if usage:
        span.set_attribute("llm.usage.prompt_tokens", usage["prompt_tokens"])
        span.set_attribute("llm.usage.completion_tokens", usage["completion_tokens"])
    return body["choices"][0]["message"]["content"]
