from __future__ import annotations

import json
from typing import Any


SYSTEM_PROMPT = """You are a strict code reviewer scoring an AI coding agent's output.

You will be given (a) the task the agent was asked to complete, (b) the
output files the agent produced, (c) summary code metrics (test pass rate,
lint result), and (d) a tail of the conversation transcript.

Score the agent's output on five dimensions, each on a 0.0-10.0 scale:

- correctness: Does the output actually solve the task as specified?
  10 = fully solves, 0 = entirely wrong / off-topic.
- maintainability: Is the code well-structured, named, and free of dead
  code or duplication? 10 = production-quality, 0 = unmaintainable.
- completeness: Did the agent finish all required parts of the task?
  10 = nothing missing, 0 = abandoned mid-task.
- best_practices: Does the code follow language / framework conventions?
  10 = idiomatic, 0 = anti-patterns throughout.
- error_handling: Does the code handle obvious failure modes? 10 =
  robust, 0 = no handling of any error path.

Also produce a short rationale (<=600 chars) explaining the lowest score.

Be calibrated and skeptical. Most real-world agent outputs are 4-7, not 9+.
Reserve 9-10 for code you would ship to production without changes.
Reserve 0-2 for output that does not address the task at all."""


def render_user_prompt(
    *,
    code_grade: dict[str, Any],
    process_grade: dict[str, Any],
    task: dict[str, Any],
    output_files: list[dict[str, Any]],
    transcript_json: bytes,
) -> str:
    files_block = "\n\n".join(
        f"=== {f.get('path', '<unnamed>')} ===\n{_decode_content(f.get('content'))[:4000]}"
        for f in output_files[:10]  # cap files to keep prompts bounded
    )
    transcript_tail = _decode_content(transcript_json)[-3000:]
    metrics = {
        "test_pass_rate": code_grade.get("test_pass_rate"),
        "lint_score": code_grade.get("lint_score"),
        "type_check_pass": code_grade.get("type_check_pass"),
        "premature_completion": process_grade.get("premature_completion"),
    }
    return (
        f"# Task\n\n{task.get('prompt', '<no prompt>')}\n\n"
        f"# Code metrics\n\n{json.dumps(metrics, indent=2)}\n\n"
        f"# Output files (truncated)\n\n{files_block or '(no files)'}\n\n"
        f"# Transcript tail\n\n{transcript_tail or '(empty)'}\n"
    )


def _decode_content(blob: Any) -> str:
    if blob is None:
        return ""
    if isinstance(blob, bytes):
        try:
            return blob.decode("utf-8", errors="replace")
        except Exception:
            return ""
    return str(blob)
