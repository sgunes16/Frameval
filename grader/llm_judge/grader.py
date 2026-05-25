from __future__ import annotations

import asyncio
import logging
from typing import Any

from pydantic import BaseModel, Field

from grader.llm_client import build_client, load_config
from grader.llm_judge.prompts import DIMENSION_RUBRICS, render_user_prompt

logger = logging.getLogger(__name__)


class DimensionVerdict(BaseModel):
    """One per-dimension LLM call's structured output."""
    score: float = Field(ge=0.0, le=10.0)
    rationale: str = Field(max_length=600)


_DIMENSIONS: tuple[str, ...] = (
    "correctness",
    "maintainability",
    "completeness",
    "best_practices",
    "error_handling",
)


def grade(
    code_grade: dict[str, Any],
    process_grade: dict[str, Any],
    task: dict[str, Any] | None = None,
    output_files: list[dict[str, Any]] | None = None,
    transcript_json: bytes | None = None,
    config_override: Any = None,
) -> dict[str, Any]:
    """Score one run on five dimensions via FIVE concurrent LLM calls.

    Public API and return shape are unchanged from the prior single-call
    implementation. raw_responses now contains five entries, each tagged
    'dim=<name>;<payload>' where payload is either a JSON-encoded
    DimensionVerdict or a 'judge_unavailable: <reason>' sentinel.
    """
    try:
        cfg = load_config(config_override)
    except Exception as exc:
        logger.warning("judge config load failed: %s", exc)
        return _all_dims_failed(str(exc))
    return asyncio.run(
        _grade_async(cfg, code_grade, process_grade, task, output_files, transcript_json)
    )


async def _grade_async(
    cfg,
    code_grade: dict[str, Any],
    process_grade: dict[str, Any],
    task: dict[str, Any] | None,
    output_files: list[dict[str, Any]] | None,
    transcript_json: bytes | None,
) -> dict[str, Any]:
    try:
        client = build_client(cfg, async_client=True)
    except Exception as exc:
        logger.warning("judge async client init failed: %s", exc)
        return _all_dims_failed(str(exc))

    user_prompt = render_user_prompt(
        code_grade=code_grade,
        process_grade=process_grade,
        task=task or {},
        output_files=output_files or [],
        transcript_json=transcript_json or b"",
    )

    tasks = [_score_one_dim(client, cfg.model, dim, user_prompt) for dim in _DIMENSIONS]
    # return_exceptions=False because _score_one_dim already catches
    # everything internally and never raises out.
    results = await asyncio.gather(*tasks, return_exceptions=False)

    scores = {dim: results[i][0] for i, dim in enumerate(_DIMENSIONS)}
    raw_responses = [results[i][1] for i in range(len(_DIMENSIONS))]
    return {
        **scores,
        "irr_alpha": 0.0,
        "raw_responses": raw_responses,
    }


async def _score_one_dim(
    client, model: str, dim: str, user_prompt: str,
) -> tuple[float, str]:
    """Run one dim's LLM call. Returns (score, raw_response_or_sentinel).
    Never raises — failures become 0.0 + a judge_unavailable sentinel so
    one failing dim doesn't take down the whole grade.
    """
    try:
        verdict: DimensionVerdict = await client.create(
            model=model,
            response_model=DimensionVerdict,
            max_retries=2,
            max_tokens=512,
            messages=[
                {"role": "system", "content": DIMENSION_RUBRICS[dim]},
                {"role": "user", "content": user_prompt},
            ],
        )
        return verdict.score, _tag_response(dim, verdict.model_dump_json())
    except Exception as exc:
        logger.warning("judge dim=%s call failed: %s", dim, exc)
        return 0.0, _tag_response(dim, f"judge_unavailable: {str(exc)[:300]}")


def _tag_response(dim: str, payload: str) -> str:
    """Prefix each raw_response with its dim so the UI can group them.

    Format: 'dim=<name>;<payload>'. Frontend's extractRationalesByDim
    parses this prefix to map rationales to their dimension.
    """
    return f"dim={dim};{payload}"


def _all_dims_failed(reason: str) -> dict[str, Any]:
    """Sentinel when the whole pipeline fails (config / client init) —
    every dim is zero and every raw_response carries the same reason.
    """
    short = reason[:300]
    return {
        **{dim: 0.0 for dim in _DIMENSIONS},
        "irr_alpha": 0.0,
        "raw_responses": [f"dim={dim};judge_unavailable: {short}" for dim in _DIMENSIONS],
    }
