from __future__ import annotations

from typing import Any


def grade(transcript_json: str, task_prompt: str) -> dict[str, Any]:
    instructions = [line.strip() for line in task_prompt.splitlines() if line.strip()][:5]
    lower_transcript = transcript_json.lower()
    results: list[dict[str, str]] = []
    followed = 0
    for instruction in instructions:
        status = "followed" if any(token in lower_transcript for token in instruction.lower().split()[:2]) else "na"
        if status == "followed":
            followed += 1
        results.append({"instruction": instruction, "status": status, "reasoning": "heuristic transcript match"})
    total = max(len(instructions), 1)
    return {
        "instruction_compliance": round((followed / total) * 10, 2),
        "constraint_violations": 0,
        "convention_adherence": 8.0,
        "per_instruction": results,
    }
