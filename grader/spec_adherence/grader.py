from __future__ import annotations

import logging
from typing import Any

from pydantic import BaseModel, Field

from grader.llm_client import build_client_with_cleanup, load_config

logger = logging.getLogger(__name__)

_SENTINEL: dict[str, Any] = {
    "instruction_compliance": 0.0,
    "convention_adherence": 0.0,
    "constraint_violations": 0,
    "per_instruction": [],
}

_SYSTEM_PROMPT = """\
You are a rigorous spec-adherence reviewer evaluating an AI coding agent's implementation.

Given:
1. A task prompt (with instructions, constraints, and conventions)
2. The filesystem diff produced by the agent

Judge how well the agent followed the task's explicit instructions and codebase conventions.

## Output format

Return a JSON object with:
- instruction_compliance: float in [0.0, 1.0] — fraction of the task's explicit
  instructions that were correctly followed (1.0 = all followed, 0.0 = none followed).
- convention_adherence: float in [0.0, 1.0] — how well the implementation matches
  the codebase style and conventions implied or stated in the task (naming, structure,
  idioms, file placement, etc.).
- constraint_violations: int >= 0 — count of explicit constraints or requirements that
  were violated or ignored.
- per_instruction: list of objects, one per identifiable instruction or constraint
  extracted from the task prompt. Each object has:
    - instruction: str — the instruction or constraint (quote or paraphrase from prompt)
    - status: str — one of "complied", "violated", "partial", "not_applicable"
    - reasoning: str — one sentence explaining why

## Calibration

- instruction_compliance=1.0 only when every stated requirement is satisfied.
- instruction_compliance=0.0 when the implementation ignores the task entirely.
- Be specific: reference filenames, function names, or diff hunks in per_instruction reasoning.
- constraint_violations must be consistent with per_instruction (count the "violated" items).
- If the task prompt has no explicit instructions or constraints, return instruction_compliance=1.0,
  convention_adherence=1.0, constraint_violations=0, per_instruction=[].
"""


class PerInstruction(BaseModel):
    instruction: str
    status: str = Field(pattern=r"^(complied|violated|partial|not_applicable)$")
    reasoning: str = Field(max_length=500)


class SpecAdherenceVerdict(BaseModel):
    instruction_compliance: float = Field(ge=0.0, le=1.0)
    convention_adherence: float = Field(ge=0.0, le=1.0)
    constraint_violations: int = Field(ge=0)
    per_instruction: list[PerInstruction] = Field(default_factory=list)


def grade(
    task_prompt: str,
    diff: str,
    judge_config: Any,
    output_files: list[dict[str, Any]] | None = None,
) -> dict[str, Any]:
    """Grade spec adherence via one instructor structured-output call.

    Reuses the same LLM client infrastructure as llm_judge. Never raises —
    any failure returns the sentinel dict (all-zeros / empty list) so the
    caller can always populate the proto fields safely.

    Returns:
        {
            "instruction_compliance": float 0..1,
            "convention_adherence": float 0..1,
            "constraint_violations": int,
            "per_instruction": [{"instruction": str, "status": str, "reasoning": str}, ...],
        }
    """
    try:
        cfg = load_config(judge_config)
    except Exception as exc:
        logger.warning("spec_adherence: config load failed: %s", exc)
        return _SENTINEL.copy()

    try:
        client, _aclose = build_client_with_cleanup(cfg, async_client=False)
    except Exception as exc:
        logger.warning("spec_adherence: client init failed: %s", exc)
        return _SENTINEL.copy()

    user_prompt = _build_user_prompt(task_prompt, diff, output_files)

    try:
        verdict: SpecAdherenceVerdict = client.create(
            model=cfg.model,
            response_model=SpecAdherenceVerdict,
            max_retries=2,
            max_tokens=8192,
            messages=[
                {"role": "system", "content": _SYSTEM_PROMPT},
                {"role": "user", "content": user_prompt},
            ],
        )
        return {
            "instruction_compliance": verdict.instruction_compliance,
            "convention_adherence": verdict.convention_adherence,
            "constraint_violations": verdict.constraint_violations,
            "per_instruction": [
                {
                    "instruction": item.instruction,
                    "status": item.status,
                    "reasoning": item.reasoning,
                }
                for item in verdict.per_instruction
            ],
        }
    except Exception as exc:
        logger.warning("spec_adherence: LLM call failed: %s", exc)
        return _SENTINEL.copy()


def _build_user_prompt(
    task_prompt: str,
    diff: str,
    output_files: list[dict[str, Any]] | None,
) -> str:
    files_block = ""
    if output_files:
        snippets = []
        for f in output_files[:5]:
            path = f.get("path", "<unknown>")
            content = f.get("content", b"")
            if isinstance(content, bytes):
                try:
                    content = content.decode("utf-8", errors="replace")
                except Exception:
                    content = ""
            snippets.append(f"=== {path} ===\n{str(content)[:2000]}")
        files_block = "\n\n".join(snippets)

    diff_section = (diff or "(no diff provided)")[:6000]

    parts = [
        "# Task prompt\n\n" + (task_prompt or "(no prompt)"),
        "# Filesystem diff\n\n" + diff_section,
    ]
    if files_block:
        parts.append("# Output files (truncated)\n\n" + files_block)
    return "\n\n".join(parts)
