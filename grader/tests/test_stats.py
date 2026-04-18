from grader.stats import compute_stats


def test_compute_stats_returns_pairwise_metrics() -> None:
    result = compute_stats(
        "exp-1",
        [
            {"variant_id": "a", "grades": [{"composite_score": 6.0, "test_pass_rate": 0.5, "judge_correctness": 5.0}]},
            {"variant_id": "b", "grades": [{"composite_score": 8.0, "test_pass_rate": 0.7, "judge_correctness": 7.0}]},
        ],
    )
    assert result
    assert result[0]["metric_name"] in {"composite_score", "test_pass_rate", "judge_correctness"}
