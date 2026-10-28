import sys

import httpx

from theliv_agent import telemetry
from theliv_agent.llm import explain
from theliv_agent.settings import Settings
from theliv_agent.theliv import fetch_report


def main() -> int:
    settings = Settings.from_env()
    tracer = telemetry.setup(settings)
    try:
        with tracer.start_as_current_span("theliv_agent.run"), httpx.Client() as client:
            with tracer.start_as_current_span("theliv.fetch_report") as span:
                findings = fetch_report(client, settings)
                span.set_attribute("theliv.findings", len(findings))
            print(f"findings: {len(findings)}")
            for f in findings:
                print(f"- {f.issue} on {f.resource} ({f.card})")
            with tracer.start_as_current_span("llm.explain"):
                answer = explain(client, settings, findings)
            print()
            print(answer)
        return 0
    except httpx.HTTPError as e:
        print(f"error: {e}", file=sys.stderr)
        return 1
    finally:
        telemetry.shutdown()


if __name__ == "__main__":
    sys.exit(main())
