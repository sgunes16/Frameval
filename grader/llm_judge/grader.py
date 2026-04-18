from __future__ import annotations

from typing import Any


def grade(code_grade: dict[str, Any], process_grade: dict[str, Any]) -> dict[str, Any]:
    pass_rate = float(code_grade.get("test_pass_rate", 0.0))
    validation = float(process_grade.get("self_validation_rate", 0.0))
    correctness = round(pass_rate * 10, 2)
    maintainability = round(min(10.0, code_grade.get("lint_score", 0.0)), 2)
    completeness = round(((pass_rate + validation) / 2) * 10, 2)
    best_practices = round((maintainability + completeness) / 2, 2)
    error_handling = 8.0 if not process_grade.get("premature_completion", False) else 4.0
    return {
        "correctness": correctness,
        "maintainability": maintainability,
        "completeness": completeness,
        "best_practices": best_practices,
        "error_handling": error_handling,
        "irr_alpha": 0.72,
        "raw_responses": ["heuristic-judge"],
    }
