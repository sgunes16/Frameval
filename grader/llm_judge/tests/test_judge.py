from __future__ import annotations

import json
from types import SimpleNamespace
from unittest.mock import patch

import pytest

from grader.llm_judge.grader import DimensionVerdict, _DIMENSIONS, grade


class _AsyncStub:
    """Async stub that returns a different result per dim.

    Looks at the system prompt's first lines to identify which dim is
    being scored (rubrics start with 'You are a strict senior code
    reviewer scoring ONE dimension of an AI coding agent's output:
    **CORRECTNESS**.' etc.). Maps to the configured result.
    """

    def __init__(self, by_dim: dict[str, DimensionVerdict | Exception]):
        self.by_dim = by_dim
        self.calls: list[str] = []

    async def create(self, *, model, response_model, max_retries, max_tokens, messages):
        sys = messages[0]["content"]
        for dim in _DIMENSIONS:
            # Rubric headers use human-readable casing ("BEST PRACTICES"),
            # but dim keys are underscored ("best_practices"). Convert to match.
            tag = f"**{dim.upper().replace('_', ' ')}**"
            if tag in sys:
                self.calls.append(dim)
                result = self.by_dim[dim]
                if isinstance(result, Exception):
                    raise result
                return result
        raise AssertionError(f"no dim tag matched in system prompt: {sys[:120]}")


def _v(score: float, rationale: str = "evidence cited") -> DimensionVerdict:
    return DimensionVerdict(score=score, rationale=rationale)


def test_grade_happy_path_all_five():
    stub = _AsyncStub({
        "correctness": _v(8.0, "fix matches spec exactly"),
        "maintainability": _v(7.5, "names are clear"),
        "completeness": _v(9.0, "all 3 requirements covered"),
        "best_practices": _v(6.0, "uses sync lock in async code"),
        "error_handling": _v(5.5, "happy path only"),
    })
    with patch("grader.llm_judge.grader.build_client", return_value=stub):
        out = grade(
            code_grade={"test_pass_rate": 0.8, "lint_score": 7.0, "type_check_pass": True},
            process_grade={"premature_completion": False},
            task={"prompt": "Fix the race condition"},
            output_files=[{"path": "src/main.py", "content": b"x = 1"}],
            transcript_json=b"{}",
        )

    # Distinct scores — proves we are not anchoring on one number.
    assert out["correctness"] == 8.0
    assert out["maintainability"] == 7.5
    assert out["completeness"] == 9.0
    assert out["best_practices"] == 6.0
    assert out["error_handling"] == 5.5
    assert out["irr_alpha"] == 0.0
    # Five raw_responses, each tagged with its dim.
    assert len(out["raw_responses"]) == 5
    for entry, dim in zip(out["raw_responses"], _DIMENSIONS):
        assert entry.startswith(f"dim={dim};")
        # The non-sentinel payload is JSON-encoded DimensionVerdict.
        payload = entry[len(f"dim={dim};"):]
        parsed = json.loads(payload)
        assert "score" in parsed
        assert "rationale" in parsed
    # All five dims were actually called.
    assert sorted(stub.calls) == sorted(_DIMENSIONS)


def test_grade_one_dim_fails_others_succeed():
    stub = _AsyncStub({
        "correctness": RuntimeError("boom on correctness"),
        "maintainability": _v(7.0),
        "completeness": _v(8.0),
        "best_practices": _v(6.0),
        "error_handling": _v(5.0),
    })
    with patch("grader.llm_judge.grader.build_client", return_value=stub):
        out = grade(
            code_grade={}, process_grade={}, task={}, output_files=[], transcript_json=b"",
        )
    # The failing dim is zero with a sentinel raw_response.
    assert out["correctness"] == 0.0
    correctness_raw = next(r for r in out["raw_responses"] if r.startswith("dim=correctness;"))
    assert "judge_unavailable: boom on correctness" in correctness_raw
    # Other dims still scored normally.
    assert out["maintainability"] == 7.0
    assert out["completeness"] == 8.0
    assert out["best_practices"] == 6.0
    assert out["error_handling"] == 5.0


def test_grade_all_dims_fail_returns_sentinels():
    stub = _AsyncStub({
        dim: RuntimeError(f"boom-{dim}") for dim in _DIMENSIONS
    })
    with patch("grader.llm_judge.grader.build_client", return_value=stub):
        out = grade(
            code_grade={}, process_grade={}, task={}, output_files=[], transcript_json=b"",
        )
    for dim in _DIMENSIONS:
        assert out[dim] == 0.0
    # All five raw_responses are sentinels.
    assert all("judge_unavailable: boom-" in r for r in out["raw_responses"])


def test_grade_client_init_failure_returns_all_dims_failed():
    """When build_client raises, _all_dims_failed is returned without any
    per-dim LLM call attempt. All five raw_responses carry the same reason."""
    with patch("grader.llm_judge.grader.build_client", side_effect=RuntimeError("no key")):
        out = grade(
            code_grade={}, process_grade={}, task={}, output_files=[], transcript_json=b"",
        )
    for dim in _DIMENSIONS:
        assert out[dim] == 0.0
        matching = next(r for r in out["raw_responses"] if r.startswith(f"dim={dim};"))
        assert "judge_unavailable: no key" in matching


def test_grade_config_override_beats_env(monkeypatch):
    """Per-call override (e.g., from the engine's JudgeConfig proto) wins
    over env vars in load_config — verified via the captured cfg."""
    monkeypatch.setenv("FRAMEVAL_LLM_PROVIDER", "openrouter")
    stub = _AsyncStub({dim: _v(5.0) for dim in _DIMENSIONS})
    captured: dict[str, str] = {}

    def fake_build(cfg, *, async_client):
        captured["provider"] = cfg.provider
        captured["model"] = cfg.model
        captured["async_client"] = async_client
        return stub

    with patch("grader.llm_judge.grader.build_client", side_effect=fake_build):
        override = SimpleNamespace(provider="zai", model="glm-4.6", api_key="zk-test")
        grade(
            code_grade={}, process_grade={}, task={}, output_files=[], transcript_json=b"",
            config_override=override,
        )
    assert captured["provider"] == "zai"
    assert captured["model"] == "glm-4.6"
    assert captured["async_client"] is True
