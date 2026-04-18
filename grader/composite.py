from __future__ import annotations

from typing import Any


def compute_process_score(process_grade: dict[str, Any]) -> float:
    self_validation = float(process_grade.get("self_validation_rate", 0.0))
    token_efficiency = float(process_grade.get("token_efficiency", 0.0))
    context_utilization = float(process_grade.get("context_utilization", 0.0))
    return round(((self_validation * 0.4) + (token_efficiency * 0.3) + (context_utilization * 0.3)) * 10, 4)


def compute_composite(
    code_grade: dict[str, Any],
    process_grade: dict[str, Any],
    judge_grade: dict[str, Any] | None = None,
    adherence_grade: dict[str, Any] | None = None,
) -> float:
    code_score = float(code_grade.get("test_pass_rate", 0.0)) * 10
    process_score = compute_process_score(process_grade)

    if judge_grade is None and adherence_grade is None:
        return round((code_score * 0.6) + (process_score * 0.4), 4)

    judge_score = float((judge_grade or {}).get("correctness", 0.0))
    adherence_score = float((adherence_grade or {}).get("instruction_compliance", 0.0))
    return round((code_score * 0.3) + (judge_score * 0.3) + (process_score * 0.2) + (adherence_score * 0.2), 4)
