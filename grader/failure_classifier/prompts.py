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


SYSTEM_PROMPT = f"""You are a failure-mode classifier for agentic coding runs.

Given a compact symptom packet and a tail of the agent's transcript, output a
structured FailureClassification identifying which categories from the
following taxonomy apply. Multi-label is allowed (primary + up to 3 secondary),
but NONE is mutually exclusive with all other codes — set it only when the
run completed cleanly without significant issues.

Each label must be backed by at least one EvidenceSpan: a verbatim quote from
the transcript and the 0-based turn index where it appears. If you cannot
find supporting evidence for a label, do not include it.

Taxonomy:
{_format_taxonomy()}

Output rules:
  - confidence in [0.0, 1.0] reflecting your certainty in the primary label.
  - rationale: at most 400 chars summarizing the verdict.
  - When primary=NONE, leave secondary empty and evidence empty.

Be precise. Symptoms like 'tests failed' alone do NOT imply STOP_EARLY unless
the agent also claimed completion.
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
