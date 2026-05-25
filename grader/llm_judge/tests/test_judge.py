from __future__ import annotations

import json
from types import SimpleNamespace
from unittest.mock import patch

import pytest

from grader.llm_judge.grader import DimensionVerdict, grade


class _AsyncStub:
    def __init__(self, by_key: dict):
        self.by_key = by_key

    async def create(self, *, model, response_model, max_retries, max_tokens, messages):
        sys = messages[0]["content"]
        for key, result in self.by_key.items():
            tag = f"**{key.upper().replace('_', ' ')}**"
            if tag in sys:
                if isinstance(result, Exception):
                    raise result
                return result
        raise AssertionError(f"no key matched in system prompt: {sys[:80]}")


def _v(score: float, rationale: str = "ok") -> DimensionVerdict:
    return DimensionVerdict(score=score, rationale=rationale)


def _proto_rubrics(*keys_and_prompts: tuple[str, str]):
    return SimpleNamespace(
        provider="", model="", api_key="",
        rubrics=[SimpleNamespace(key=k, prompt=p) for k, p in keys_and_prompts],
    )


def test_grade_uses_rubrics_from_proto():
    stub = _AsyncStub({"foo": _v(8.0, "rfoo"), "bar": _v(7.0, "rbar")})
    cfg = _proto_rubrics(("foo", "scoring **FOO**"), ("bar", "scoring **BAR**"))
    with patch("grader.llm_judge.grader.build_client", return_value=stub):
        out = grade({}, {}, task={}, output_files=[], transcript_json=b"", config_override=cfg)
    assert out["scores"] == {"foo": 8.0, "bar": 7.0}
    assert out["rationales"] == {"foo": "rfoo", "bar": "rbar"}
    assert len(out["raw_responses"]) == 2


def test_grade_falls_back_to_builtin_rubrics_when_none_supplied():
    stub = _AsyncStub({
        "correctness": _v(8.0), "maintainability": _v(7.0), "completeness": _v(9.0),
        "best_practices": _v(6.0), "error_handling": _v(5.0),
    })
    with patch("grader.llm_judge.grader.build_client", return_value=stub):
        out = grade({}, {}, task={}, output_files=[], transcript_json=b"", config_override=None)
    assert len(out["scores"]) == 5
    assert "correctness" in out["scores"]


def test_grade_one_dim_fails_others_succeed():
    stub = _AsyncStub({
        "correctness": RuntimeError("boom"),
        "maintainability": _v(7.0), "completeness": _v(8.0),
        "best_practices": _v(6.0), "error_handling": _v(5.0),
    })
    with patch("grader.llm_judge.grader.build_client", return_value=stub):
        out = grade({}, {}, task={}, output_files=[], transcript_json=b"", config_override=None)
    assert out["scores"]["correctness"] == 0.0
    assert "judge_unavailable: boom" in out["rationales"]["correctness"]
    assert out["scores"]["maintainability"] == 7.0


def test_grade_client_init_failure_returns_all_failed():
    cfg = _proto_rubrics(("alpha", "**ALPHA**"), ("beta", "**BETA**"))
    with patch("grader.llm_judge.grader.build_client", side_effect=RuntimeError("no key")):
        out = grade({}, {}, task={}, output_files=[], transcript_json=b"", config_override=cfg)
    assert out["scores"] == {"alpha": 0.0, "beta": 0.0}
    assert all("judge_unavailable: no key" in r for r in out["rationales"].values())


def test_grade_config_override_beats_env(monkeypatch):
    """Per-call override (e.g., from the engine's JudgeConfig proto) wins
    over env vars in load_config — verified via the captured cfg."""
    monkeypatch.setenv("FRAMEVAL_LLM_PROVIDER", "openrouter")
    stub = _AsyncStub({
        "correctness": _v(5.0), "maintainability": _v(5.0), "completeness": _v(5.0),
        "best_practices": _v(5.0), "error_handling": _v(5.0),
    })
    captured: dict[str, str] = {}

    def fake_build(cfg, *, async_client):
        captured["provider"] = cfg.provider
        captured["model"] = cfg.model
        captured["async_client"] = async_client
        return stub

    with patch("grader.llm_judge.grader.build_client", side_effect=fake_build):
        override = SimpleNamespace(provider="zai", model="glm-4.6", api_key="zk-test", rubrics=[])
        grade(
            code_grade={}, process_grade={}, task={}, output_files=[], transcript_json=b"",
            config_override=override,
        )
    assert captured["provider"] == "zai"
    assert captured["model"] == "glm-4.6"
    assert captured["async_client"] is True
