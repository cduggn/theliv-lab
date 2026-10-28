from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import (
    BatchSpanProcessor,
    ConsoleSpanExporter,
    SimpleSpanProcessor,
)

from theliv_agent.settings import Settings

_provider: TracerProvider | None = None


def setup(settings: Settings) -> trace.Tracer:
    global _provider
    _provider = TracerProvider(resource=Resource.create({"service.name": "theliv-agent"}))
    if settings.otlp_endpoint:
        endpoint = settings.otlp_endpoint.rstrip("/") + "/v1/traces"
        _provider.add_span_processor(BatchSpanProcessor(OTLPSpanExporter(endpoint=endpoint)))
    else:
        _provider.add_span_processor(SimpleSpanProcessor(ConsoleSpanExporter()))
    HTTPXClientInstrumentor().instrument(tracer_provider=_provider)
    return _provider.get_tracer("theliv_agent")


def shutdown() -> None:
    if _provider is not None:
        _provider.shutdown()
