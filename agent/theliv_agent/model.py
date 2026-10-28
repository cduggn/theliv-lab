from dataclasses import dataclass


@dataclass(frozen=True)
class Finding:
    card: str
    issue: str
    description: str
    resource: str
    solutions: tuple[str, ...]
    commands: tuple[str, ...]


def parse_report(payload: list[dict]) -> list[Finding]:
    return [
        Finding(
            card=card["name"],
            issue=card["rootCause"]["name"],
            description=card["rootCause"]["description"],
            resource=card["resources"][0]["name"],
            solutions=tuple(card["rootCause"].get("solutions", ())),
            commands=tuple(card["rootCause"].get("commands", ())),
        )
        for card in payload
    ]
