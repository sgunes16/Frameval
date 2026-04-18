from grader.code_grader import grade


def test_grade_runs_shell_tests() -> None:
    result = grade(
        {"test_cases": [{"name": "echo", "command": "python3 -c \"print(1)\""}]},
        [{"path": "main.py", "content": "print(1)"}],
    )
    assert result["test_pass_count"] == 1
    assert result["file_state_valid"] is True
