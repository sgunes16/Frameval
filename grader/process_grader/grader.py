from __future__ import annotations

from typing import Any


def grade(transcript_json: str) -> dict[str, Any]:
    lines = [line for line in transcript_json.splitlines() if line.strip()]
    total_tokens = sum(len(line.split()) for line in lines)
    backtracks = sum(1 for line in lines if "different approach" in line.lower() or "revert" in line.lower())
    validations = sum(1 for line in lines if "test" in line.lower() or "lint" in line.lower())
    idle_turns = sum(1 for line in lines if len(line.split()) < 3)
    return {
        "turn_count": len(lines),
        "total_tokens": total_tokens,
        "cost_usd": round(total_tokens / 100000, 4),
        "token_efficiency": round(min(1.0, 1000 / max(total_tokens, 1)), 4),
        "backtrack_count": backtracks,
        "self_validation_rate": round(validations / max(len(lines), 1), 4),
        "premature_completion": validations == 0 and len(lines) > 0,
        "idle_turns": idle_turns,
        "error_recovery_count": backtracks,
        "tool_call_accuracy": 0.75 if len(lines) else 0.0,
        "context_utilization": 0.8 if total_tokens > 20 else 0.4,
    }
