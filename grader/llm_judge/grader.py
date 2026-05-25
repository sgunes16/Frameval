from __future__ import annotations

import logging
from typing import Any

from pydantic import BaseModel, Field

from grader.llm_client import build_client, load_config
from grader.llm_judge.prompts import SYSTEM_PROMPT, render_user_prompt

logger = logging.getLogger(__name__)


class JudgeResult(BaseModel):
    correctness: float = Field(ge=0.0, le=10.0)
    maintainability: float = Field(ge=0.0, le=10.0)
    completeness: float = Field(ge=0.0, le=10.0)
    best_practices: float = Field(ge=0.0, le=10.0)
    error_handling: float = Field(ge=0.0, le=10.0)
    rationale: str = Field(max_length=600)


def grade(
    code_grade: dict[str, Any],
    process_grade: dict[str, Any],
    task: dict[str, Any] | None = None,
    output_files: list[dict[str, Any]] | None = None,
    transcript_json: bytes | None = None,
    config_override: Any = None,
) -> dict[str, Any]:
    """Score one run on five dimensions via a single LLM call.

    Returns the dict shape grader_pb2.JudgeGradeResult expects. On any
    hard failure (network, validation, no key) returns a sentinel result
    with all dims at 0.0 and a `judge_unavailable: <reason>` raw_responses
    entry, so the orchestrator never crashes mid-pipeline.

    config_override is the request-time JudgeConfig proto (or any object
    with .provider / .model / .api_key string attrs). Falsy fields fall
    through to env-var defaults inside load_config.
    """
    try:
        cfg = load_config(config_override)
        print(f"[judge_debug] resolved cfg provider={cfg.provider} base_url={cfg.base_url} model={cfg.model} has_key={bool(cfg.api_key)}", flush=True)
        client = build_client(cfg)
        print(f"[judge_debug] client built", flush=True)
    except Exception as exc:
        print(f"[judge_debug] client init exception: {exc!r}", flush=True)
        logger.warning("judge client init failed: %s", exc)
        return _failed_judge_result(str(exc))

    prompt = render_user_prompt(
        code_grade=code_grade,
        process_grade=process_grade,
        task=task or {},
        output_files=output_files or [],
        transcript_json=transcript_json or b"",
    )
    print(f"[judge_debug] calling client.create model={cfg.model} prompt_len={len(prompt)}", flush=True)
    try:
        verdict: JudgeResult = client.create(
            model=cfg.model,
            response_model=JudgeResult,
            max_retries=2,
            max_tokens=1024,
            messages=[
                {"role": "system", "content": SYSTEM_PROMPT},
                {"role": "user", "content": prompt},
            ],
        )
        print(f"[judge_debug] client.create returned ok correctness={verdict.correctness}", flush=True)
    except Exception as exc:
        print(f"[judge_debug] client.create exception: {exc!r}", flush=True)
        logger.warning("judge call failed: %s", exc)
        return _failed_judge_result(str(exc))

    return {
        "correctness": verdict.correctness,
        "maintainability": verdict.maintainability,
        "completeness": verdict.completeness,
        "best_practices": verdict.best_practices,
        "error_handling": verdict.error_handling,
        "irr_alpha": 0.0,  # single-model run — no IRR by definition
        "raw_responses": [verdict.model_dump_json()],
    }


def _failed_judge_result(reason: str) -> dict[str, Any]:
    return {
        "correctness": 0.0,
        "maintainability": 0.0,
        "completeness": 0.0,
        "best_practices": 0.0,
        "error_handling": 0.0,
        "irr_alpha": 0.0,
        "raw_responses": [f"judge_unavailable: {reason[:300]}"],
    }
