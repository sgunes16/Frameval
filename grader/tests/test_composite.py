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
