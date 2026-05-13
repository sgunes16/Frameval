from __future__ import annotations

import os
from dataclasses import dataclass


@dataclass(slots=True)
class Settings:
    port: int = int(os.getenv("GRADER_PORT", "50051"))
    version: str = "0.1.0"
    enable_llm_judge: bool = os.getenv("FRAMEVAL_ENABLE_LLM_JUDGE", "false").lower() == "true"


def get_settings() -> Settings:
    return Settings()
