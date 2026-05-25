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


def grade(
    code_grade: dict[str, Any],
    process_grade: dict[str, Any],
    task: dict[str, Any] | None = None,
    output_files: list[dict[str, Any]] | None = None,
    transcript_json: bytes | None = None,
    config_override: Any = None,
) -> dict[str, Any]:
    """Score one run on N dimensions via N concurrent LLM calls.

    Rubrics are read from config_override.rubrics (a list of proto
    DimensionRubric messages). When absent or empty, falls back to the
    hardcoded DIMENSION_RUBRICS defaults.

    Return shape:
        {
            "scores":        {key: float, ...},
            "rationales":    {key: str, ...},
            "irr_alpha":     0.0,
            "raw_responses": [str, ...],
        }
    """
    try:
        cfg = load_config(config_override)
    except Exception as exc:
        logger.warning("judge config load failed: %s", exc)
        rubrics_for_error = _extract_rubrics(config_override) or _builtin_rubrics()
        return _all_dims_failed(str(exc), rubrics_for_error)
    rubrics_from_request = _extract_rubrics(config_override)
    effective = rubrics_from_request or _builtin_rubrics()
    return asyncio.run(
        _grade_async(cfg, code_grade, process_grade, task, output_files, transcript_json, effective)
    )


def _extract_rubrics(config_override: Any) -> list[tuple[str, str]] | None:
    """Pull (key, prompt) pairs out of a JudgeConfig proto's rubrics field.
    Returns None if config_override is missing or has no rubrics."""
    if config_override is None:
        return None
    rubrics_attr = getattr(config_override, "rubrics", None)
    if not rubrics_attr:
        return None
    return [(r.key, r.prompt) for r in rubrics_attr if r.key and r.prompt]


def _builtin_rubrics() -> list[tuple[str, str]]:
    return list(DIMENSION_RUBRICS.items())


async def _grade_async(
    cfg,
    code_grade: dict[str, Any],
    process_grade: dict[str, Any],
    task: dict[str, Any] | None,
    output_files: list[dict[str, Any]] | None,
    transcript_json: bytes | None,
    rubrics: list[tuple[str, str]],
) -> dict[str, Any]:
    try:
        client = build_client(cfg, async_client=True)
    except Exception as exc:
        logger.warning("judge async client init failed: %s", exc)
        return _all_dims_failed(str(exc), rubrics)

    user_prompt = render_user_prompt(
        code_grade=code_grade,
        process_grade=process_grade,
        task=task or {},
        output_files=output_files or [],
        transcript_json=transcript_json or b"",
    )

    tasks = [_score_one_dim(client, cfg.model, key, prompt, user_prompt) for key, prompt in rubrics]
    # return_exceptions=False because _score_one_dim already catches
    # everything internally and never raises out.
    results = await asyncio.gather(*tasks, return_exceptions=False)

    scores = {rubrics[i][0]: results[i][0] for i in range(len(rubrics))}
    rationales = {rubrics[i][0]: results[i][1] for i in range(len(rubrics))}
    raw_responses = [results[i][2] for i in range(len(rubrics))]
    return {
        "scores": scores,
        "rationales": rationales,
        "irr_alpha": 0.0,
        "raw_responses": raw_responses,
    }


async def _score_one_dim(
    client, model: str, key: str, prompt: str, user_prompt: str,
) -> tuple[float, str, str]:
    """Run one dim's LLM call. Returns (score, rationale, raw_response_string).
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
                {"role": "system", "content": prompt},
                {"role": "user", "content": user_prompt},
            ],
        )
        return verdict.score, verdict.rationale, _tag_response(key, verdict.model_dump_json())
    except Exception as exc:
        logger.warning("judge dim=%s call failed: %s", key, exc)
        sentinel = f"judge_unavailable: {str(exc)[:300]}"
        return 0.0, sentinel, _tag_response(key, sentinel)


def _tag_response(key: str, payload: str) -> str:
    """Prefix each raw_response with its dim so the UI can group them.

    Format: 'dim=<name>;<payload>'. Frontend's extractRationalesByDim
    parses this prefix to map rationales to their dimension.
    """
    return f"dim={key};{payload}"


def _all_dims_failed(reason: str, rubrics: list[tuple[str, str]]) -> dict[str, Any]:
    """Sentinel when the whole pipeline fails (config / client init) —
    every dim is zero and every raw_response carries the same reason.
    """
    short = reason[:300]
    return {
        "scores": {key: 0.0 for key, _ in rubrics},
        "rationales": {key: f"judge_unavailable: {short}" for key, _ in rubrics},
        "irr_alpha": 0.0,
        "raw_responses": [f"dim={key};judge_unavailable: {short}" for key, _ in rubrics],
    }
