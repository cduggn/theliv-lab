import httpx

from theliv_agent.llm import render_findings
from theliv_agent.model import Finding, parse_report
from theliv_agent.settings import Settings
from theliv_agent.theliv import fetch_report

MOCK_REPORT = [
    {
        "name": "missing-image",
        "rootCause": {
            "name": "ContainerWaitingAsImagePullBackOff",
            "description": "Container demo of Pod missing-image-7d9f6c5b4-x2k9p in namespace theliv-demo is waiting due to ImagePullBackOff, reason is ImagePullBackOff",
            "solutions": [
                "1. Unable to pull image busybox:theliv-intentionally-missing for the container demo. The root cause could be one of the following.",
                "2. Either the image repository name is incorrect or does NOT exist.",
                "3. Either the image tag theliv-intentionally-missing is invalid or does NOT exist."
            ],
            "commands": [
                "kubectl describe po missing-image-7d9f6c5b4-x2k9p -n theliv-demo",
                "kubectl get events --field-selector involvedObject.name=missing-image-7d9f6c5b4-x2k9p -n theliv-demo"
            ],
            "tags": {
                "container": "demo",
                "image": "busybox:theliv-intentionally-missing",
                "namespace": "theliv-demo",
                "pod": "missing-image-7d9f6c5b4-x2k9p",
                "resourcetype": "container"
            },
            "causelevel": 1
        },
        "resources": [
            {
                "name": "missing-image-7d9f6c5b4-x2k9p",
                "type": "Pod",
                "labels": {
                    "app": "missing-image"
                },
                "issue": {
                    "name": "ContainerWaitingAsImagePullBackOff",
                    "description": "Container demo of Pod missing-image-7d9f6c5b4-x2k9p in namespace theliv-demo is waiting due to ImagePullBackOff, reason is ImagePullBackOff",
                    "solutions": [
                        "1. Unable to pull image busybox:theliv-intentionally-missing for the container demo. The root cause could be one of the following.",
                        "2. Either the image repository name is incorrect or does NOT exist.",
                        "3. Either the image tag theliv-intentionally-missing is invalid or does NOT exist."
                    ],
                    "commands": [
                        "kubectl describe po missing-image-7d9f6c5b4-x2k9p -n theliv-demo",
                        "kubectl get events --field-selector involvedObject.name=missing-image-7d9f6c5b4-x2k9p -n theliv-demo"
                    ],
                    "tags": {
                        "container": "demo",
                        "image": "busybox:theliv-intentionally-missing",
                        "namespace": "theliv-demo",
                        "pod": "missing-image-7d9f6c5b4-x2k9p",
                        "resourcetype": "container"
                    },
                    "causelevel": 1
                }
            }
        ],
        "topResourceType": "Deployment",
        "level": 2,
        "id": "1"
    }
]

POD = "missing-image-7d9f6c5b4-x2k9p"
SOLUTION_1 = (
    "1. Unable to pull image busybox:theliv-intentionally-missing for the container demo. "
    "The root cause could be one of the following."
)

EXPECTED = Finding(
    card="missing-image",
    issue="ContainerWaitingAsImagePullBackOff",
    description=(
        "Container demo of Pod missing-image-7d9f6c5b4-x2k9p in namespace theliv-demo "
        "is waiting due to ImagePullBackOff, reason is ImagePullBackOff"
    ),
    resource=POD,
    solutions=(
        SOLUTION_1,
        "2. Either the image repository name is incorrect or does NOT exist.",
        "3. Either the image tag theliv-intentionally-missing is invalid or does NOT exist.",
    ),
    commands=(
        "kubectl describe po missing-image-7d9f6c5b4-x2k9p -n theliv-demo",
        "kubectl get events --field-selector involvedObject.name=missing-image-7d9f6c5b4-x2k9p -n theliv-demo",
    ),
)


def test_parse_report_maps_go_mock_to_finding():
    assert parse_report(MOCK_REPORT) == [EXPECTED]


def test_render_findings_contains_pod_and_first_solution():
    text = render_findings([EXPECTED])
    assert POD in text
    assert f"solution: {SOLUTION_1}" in text


def test_fetch_report_parses_response_from_theliv_path():
    seen = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request.url.path)
        return httpx.Response(200, json=MOCK_REPORT)

    settings = Settings.from_env()
    with httpx.Client(transport=httpx.MockTransport(handler)) as client:
        assert fetch_report(client, settings) == [EXPECTED]
    assert seen == ["/theliv-api/v1/agent/mock/report"]
