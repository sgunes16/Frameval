from grader.composite import compute_composite

def test_no_judge_no_adherence_uses_60_40():
    out = compute_composite({"test_pass_rate": 1.0}, {})
    # code=10 * 0.6 + process_score * 0.4 (process_score=0 here)
    assert out == 6.0

def test_judge_with_one_dim_averages_correctly():
    out = compute_composite(
        {"test_pass_rate": 1.0},
        {"self_validation_rate": 0.5, "token_efficiency": 0.4, "context_utilization": 0.5},
        judge_grade={"scores": {"correctness": 8.0}},
    )
    # mean({correctness: 8}) = 8; 10*0.3 + 8*0.3 + process_score*0.2 + 0*0.2
    assert out > 0

def test_judge_with_five_dims_averages_them():
    judge = {"scores": {"correctness": 8.0, "maintainability": 7.0, "completeness": 9.0, "best_practices": 6.0, "error_handling": 5.0}}
    out = compute_composite({"test_pass_rate": 1.0}, {}, judge_grade=judge)
    # mean = 7.0; 10*0.3 + 7*0.3 + 0 + 0 = 5.1
    assert abs(out - 5.1) < 0.01

def test_judge_with_n_dims_averages_them():
    judge = {"scores": {f"dim{i}": 10.0 for i in range(20)}}
    out = compute_composite({"test_pass_rate": 0.0}, {}, judge_grade=judge)
    # 0 + 10 * 0.3 + 0 + 0 = 3.0
    assert abs(out - 3.0) < 0.01

def test_judge_empty_scores_treated_as_zero():
    out = compute_composite({"test_pass_rate": 0.0}, {}, judge_grade={"scores": {}})
    assert out == 0.0

def test_judge_unavailable_dim_excluded_from_average():
    # One dimension's LLM call failed (judge_unavailable sentinel + 0.0 score).
    # It must NOT count as a real zero — exclude it so a parse/timeout failure
    # doesn't drag the composite down. Mirrors speckit/tdd-first in exp 47d31451.
    judge = {
        "scores": {"best_practices": 8.5, "completeness": 9.0, "correctness": 7.0,
                   "error_handling": 0.0, "maintainability": 9.2},
        "rationales": {"best_practices": "ok", "completeness": "ok", "correctness": "ok",
                       "error_handling": "judge_unavailable: EOF while parsing",
                       "maintainability": "ok"},
    }
    out = compute_composite({"test_pass_rate": 0.0}, {}, judge_grade=judge)
    # valid mean = (8.5+9+7+9.2)/4 = 8.425; composite = 0*0.3 + 8.425*0.3 = 2.5275
    assert abs(out - 2.5275) < 0.001

def test_all_judge_dims_unavailable_is_zero():
    judge = {
        "scores": {"a": 0.0, "b": 0.0},
        "rationales": {"a": "judge_unavailable: x", "b": "judge_unavailable: y"},
    }
    out = compute_composite({"test_pass_rate": 0.0}, {}, judge_grade=judge)
    assert out == 0.0
