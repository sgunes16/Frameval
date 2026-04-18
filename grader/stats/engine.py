from __future__ import annotations

from itertools import combinations
from statistics import mean, median, pstdev
from typing import Any

try:
    from scipy.stats import mannwhitneyu  # type: ignore
except Exception:  # pragma: no cover
    mannwhitneyu = None


def _metric_values(grades: list[dict[str, Any]], metric_name: str) -> list[float]:
    values: list[float] = []
    for grade in grades:
        if metric_name in grade:
            values.append(float(grade[metric_name]))
    return values


def compute_stats(experiment_id: str, variant_grades: list[dict[str, Any]]) -> list[dict[str, Any]]:
    stats: list[dict[str, Any]] = []
    metrics = ["composite_score", "test_pass_rate", "token_efficiency", "context_utilization"]
    for left, right in combinations(variant_grades, 2):
        for metric in metrics:
            values_a = _metric_values(left["grades"], metric)
            values_b = _metric_values(right["grades"], metric)
            if not values_a or not values_b:
                continue
            u_stat = 0.0
            p_value = 1.0
            if mannwhitneyu is not None:
                test = mannwhitneyu(values_a, values_b, alternative="two-sided")
                u_stat = float(test.statistic)
                p_value = float(test.pvalue)
            stat = {
                "experiment_id": experiment_id,
                "variant_a_id": left["variant_id"],
                "variant_b_id": right["variant_id"],
                "metric_name": metric,
                "mean_a": mean(values_a),
                "mean_b": mean(values_b),
                "median_a": median(values_a),
                "median_b": median(values_b),
                "std_a": pstdev(values_a) if len(values_a) > 1 else 0.0,
                "std_b": pstdev(values_b) if len(values_b) > 1 else 0.0,
                "mann_whitney_u": u_stat,
                "p_value": p_value,
                "cohens_d": mean(values_a) - mean(values_b),
                "ci_lower": min(min(values_a), min(values_b)),
                "ci_upper": max(max(values_a), max(values_b)),
                "is_significant": p_value < 0.05,
                "observed_power": 0.5 if p_value >= 0.05 else 0.8,
            }
            stats.append(stat)
    return stats
