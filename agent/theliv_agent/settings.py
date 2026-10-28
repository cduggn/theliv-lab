import os
from dataclasses import dataclass


@dataclass(frozen=True)
class Settings:
    theliv_url: str
    model_url: str
    model: str
    model_api_key: str
    otlp_endpoint: str | None

    @classmethod
    def from_env(cls) -> "Settings":
        env = os.environ
        return cls(
            theliv_url=env.get("THELIV_URL", "http://127.0.0.1:8080").rstrip("/"),
            model_url=env.get("MODEL_URL", "http://127.0.0.1:11434/v1").rstrip("/"),
            model=env.get("MODEL", "qwen3.8:27b"),
            model_api_key=env.get("MODEL_API_KEY", "ollama"),
            otlp_endpoint=env.get("OTEL_EXPORTER_OTLP_ENDPOINT") or None,
        )
