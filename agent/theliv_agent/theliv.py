import httpx

from theliv_agent.model import Finding, parse_report
from theliv_agent.settings import Settings


def fetch_report(client: httpx.Client, settings: Settings) -> list[Finding]:
    resp = client.get(f"{settings.theliv_url}/theliv-api/v1/agent/mock/report")
    resp.raise_for_status()
    return parse_report(resp.json())
