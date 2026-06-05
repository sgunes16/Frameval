"""Prompt templates for the failure classifier.

Kept separate from the LLM-calling code so the prompt text can be inspected
in tests and audited as part of the calibration study (Story #25).

Design:
  - System prompt enumerates the taxonomy with short descriptions so the
    classifier sees the canonical definitions on every call.
  - User prompt embeds a JSON-encoded symptom packet (already capped to
    ~1-4 KB by the Go-side symptom extractor) plus the task description
    plus a trailing transcript tail. The tail is a last-N-turns slice
    intentionally — full transcripts blow context and obscure signal.
"""
from __future__ import annotations

import json
from typing import Mapping

from grader.failure_classifier.taxonomy import FAILURE_DESCRIPTIONS, FailureCode


def _format_taxonomy() -> str:
    """Render the FailureCode taxonomy as a bullet list for the system prompt."""
    lines: list[str] = []
    for code in FailureCode:
        lines.append(f"  * {code.value}: {FAILURE_DESCRIPTIONS[code]}")
    return "\n".join(lines)


SYSTEM_PROMPT = f"""You are a failure-mode classifier for agentic coding runs. Your job is to
analyze the symptoms and transcript, then output a structured FailureClassification.

## Taxonomy

{_format_taxonomy()}

## Analysis Process

1. Read the symptoms packet — note test results, error messages, and declared completion.
2. Read the transcript tail — look for evidence of specific failure modes.
3. Match evidence to taxonomy codes. Each label needs at least one verbatim quote.
4. If no failure evidence exists, set primary=NONE with empty secondary and evidence.

## Output Rules

- primary: the single most significant failure code (or NONE if clean run)
- secondary: up to 3 additional contributing codes (never includes NONE)
- evidence: list of EvidenceSpan objects, each with code, quote (≤300 chars), turn_index
- confidence: float in [0.0, 1.0] — your certainty in the primary label
- rationale: at most 400 chars summarizing why you chose this classification

## Key Distinctions

- STOP_EARLY requires BOTH: (a) tests/build still failing AND (b) agent declared completion
- SILENT_SKIP requires: agent encountered error but continued without addressing it
- SCOPE_DRIFT requires: agent modified files outside the task's expected scope
- MISREAD requires: agent solved the wrong problem or broke an existing contract
- HAL_API requires: agent used a function/method that doesn't exist in the library

If symptoms show "tests failed" but the agent never claimed completion, do NOT label STOP_EARLY.
Look for the agent's actual words in the transcript to determine declared_completion.
"""


def render_user_prompt(
    *,
    symptoms: Mapping[str, object],
    task_description: str,
    transcript_tail: str,
) -> str:
    """Render the per-run user prompt.

    symptoms        — already-compact JSON-serializable dict from the Go
                      symptom extractor; the classifier sees it verbatim.
    task_description— the original task.yaml prompt, so the classifier knows
                      what was being attempted.
    transcript_tail — last ~10-20 turns of the transcript, formatted as
                      ``[<turn_index>][<role>] <content>`` per line.
    """
    return (
        "## Task being attempted\n"
        f"{task_description.strip()}\n\n"
        "## Symptoms packet (JSON)\n"
        f"```json\n{json.dumps(symptoms, indent=2, sort_keys=True)}\n```\n\n"
        "## Transcript tail\n"
        f"{transcript_tail.strip() if transcript_tail else '(empty)'}\n"
    )
