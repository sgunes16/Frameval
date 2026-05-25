from __future__ import annotations

import os
from dataclasses import dataclass, field


@dataclass(slots=True)
class Settings:
    port: int = field(default_factory=lambda: int(os.environ.get("GRADER_PORT", "50051")))
    version: str = "0.1.0"
    enable_llm_judge: bool = field(
        default_factory=lambda: os.environ.get("FRAMEVAL_ENABLE_LLM_JUDGE", "true").lower() == "true"
    )


def get_settings() -> Settings:
    return Settings()
