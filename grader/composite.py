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

    scores = (judge_grade or {}).get("scores") or {}
    rationales = (judge_grade or {}).get("rationales") or {}
    # A dimension whose LLM call failed comes back as 0.0 with a
    # "judge_unavailable: ..." rationale. That means "couldn't grade", not
    # "scored zero" — counting it would unfairly drag the composite down,
    # so drop it from the average.
    valid = {
        key: value
        for key, value in scores.items()
        if not str(rationales.get(key, "")).startswith("judge_unavailable")
    }
    judge_score = (sum(valid.values()) / len(valid)) if valid else None

    has_judge = judge_score is not None
    has_adherence = adherence_grade is not None
    # No usable judge signal (judge never ran, returned no scores, or every
    # dimension failed) and no adherence → grade on code + process only,
    # rather than penalizing the composite with a phantom zero judge term.
    if not has_judge and not has_adherence:
        return round((code_score * 0.6) + (process_score * 0.4), 4)

    judge_component = judge_score if has_judge else 0.0
    adherence_score = float((adherence_grade or {}).get("instruction_compliance", 0.0))
    return round((code_score * 0.3) + (judge_component * 0.3) + (process_score * 0.2) + (adherence_score * 0.2), 4)
