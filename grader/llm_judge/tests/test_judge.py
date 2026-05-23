from __future__ import annotations

from types import SimpleNamespace
from unittest.mock import patch

from grader.llm_judge.grader import JudgeResult, grade


def _fake_client_returning(result: JudgeResult):
    class _Stub:
        def create(self, **kwargs):
            return result
    return _Stub()


def test_grade_happy_path():
    fake = _fake_client_returning(JudgeResult(
        correctness=8.0, maintainability=7.5, completeness=9.0,
        best_practices=7.0, error_handling=6.5, rationale="solid",
    ))
    with patch("grader.llm_judge.grader.build_client", return_value=fake):
        out = grade(
            code_grade={"test_pass_rate": 0.8, "lint_score": 7.0, "type_check_pass": True},
            process_grade={"self_validation_rate": 0.6, "premature_completion": False},
            task={"prompt": "Add a CLI flag"},
            output_files=[{"path": "src/main.py", "content": b"def main(): ..."}],
            transcript_json=b"{}",
        )
    assert out["correctness"] == 8.0
    assert out["error_handling"] == 6.5
    assert out["irr_alpha"] == 0.0
    assert out["raw_responses"][0].startswith("{")


def test_grade_client_init_failure_returns_sentinel():
    with patch("grader.llm_judge.grader.build_client", side_effect=RuntimeError("no key")):
        out = grade(
            code_grade={}, process_grade={}, task={}, output_files=[], transcript_json=b"",
        )
    assert out["correctness"] == 0.0
    assert out["raw_responses"][0].startswith("judge_unavailable: no key")


def test_grade_call_failure_returns_sentinel():
    class _ExplodingClient:
        def create(self, **kwargs):
            raise RuntimeError("boom")
    with patch("grader.llm_judge.grader.build_client", return_value=_ExplodingClient()):
        out = grade(
            code_grade={}, process_grade={}, task={}, output_files=[], transcript_json=b"",
        )
    assert out["correctness"] == 0.0
    assert out["raw_responses"][0].startswith("judge_unavailable: boom")


def test_grade_config_override_beats_env(monkeypatch):
    monkeypatch.setenv("FRAMEVAL_LLM_PROVIDER", "openrouter")
    fake = _fake_client_returning(JudgeResult(
        correctness=5.0, maintainability=5.0, completeness=5.0,
        best_practices=5.0, error_handling=5.0, rationale="ok",
    ))
    captured = {}

    def fake_build(cfg):
        captured["provider"] = cfg.provider
        captured["model"] = cfg.model
        return fake

    with patch("grader.llm_judge.grader.build_client", side_effect=fake_build):
        override = SimpleNamespace(provider="zai", model="glm-4.6", api_key="zk-test")
        grade(
            code_grade={}, process_grade={}, task={}, output_files=[], transcript_json=b"",
            config_override=override,
        )
    assert captured["provider"] == "zai"
    assert captured["model"] == "glm-4.6"
