from __future__ import annotations

from types import SimpleNamespace
from unittest.mock import MagicMock, patch

import pytest

from grader.spec_adherence.grader import SpecAdherenceVerdict, PerInstruction, grade


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _make_judge_cfg(provider: str = "openrouter", model: str = "test-model", api_key: str = "sk-test"):
    return SimpleNamespace(provider=provider, model=model, api_key=api_key)


def _canned_verdict() -> SpecAdherenceVerdict:
    return SpecAdherenceVerdict(
        instruction_compliance=0.85,
        convention_adherence=0.90,
        constraint_violations=1,
        per_instruction=[
            PerInstruction(
                instruction="Use snake_case for all function names",
                status="complied",
                reasoning="All functions in the diff use snake_case naming.",
            ),
            PerInstruction(
                instruction="Do not modify existing tests",
                status="violated",
                reasoning="The diff deletes test_foo.py which was an existing test.",
            ),
        ],
    )


# ---------------------------------------------------------------------------
# Happy path: LLM returns a valid structured verdict
# ---------------------------------------------------------------------------

def test_grade_maps_verdict_to_dict(monkeypatch):
    """grade() should map the instructor structured output to the expected dict shape."""
    verdict = _canned_verdict()

    fake_client = MagicMock()
    fake_client.create.return_value = verdict

    # Patch build_client_with_cleanup so no real network call happens
    monkeypatch.setattr(
        "grader.spec_adherence.grader.build_client_with_cleanup",
        lambda cfg, async_client=False: (fake_client, MagicMock()),
    )

    result = grade(
        task_prompt="Use snake_case for all function names. Do not modify existing tests.",
        diff="--- a/src/foo.py\n+++ b/src/foo.py\n-def fooBar(): pass\n+def foo_bar(): pass",
        judge_config=_make_judge_cfg(),
    )

    assert result["instruction_compliance"] == pytest.approx(0.85)
    assert result["convention_adherence"] == pytest.approx(0.90)
    assert result["constraint_violations"] == 1
    assert len(result["per_instruction"]) == 2

    # First item: complied
    item = result["per_instruction"][0]
    assert item["instruction"] == "Use snake_case for all function names"
    assert item["status"] == "complied"
    assert "snake_case" in item["reasoning"]

    # Second item: violated
    item = result["per_instruction"][1]
    assert item["instruction"] == "Do not modify existing tests"
    assert item["status"] == "violated"


def test_grade_calls_client_with_response_model(monkeypatch):
    """grade() must call client.create with response_model=SpecAdherenceVerdict."""
    verdict = _canned_verdict()

    fake_client = MagicMock()
    fake_client.create.return_value = verdict

    monkeypatch.setattr(
        "grader.spec_adherence.grader.build_client_with_cleanup",
        lambda cfg, async_client=False: (fake_client, MagicMock()),
    )

    grade(
        task_prompt="Implement feature X.",
        diff="+ some code",
        judge_config=_make_judge_cfg(),
    )

    call_kwargs = fake_client.create.call_args.kwargs
    assert call_kwargs["response_model"] is SpecAdherenceVerdict
    assert call_kwargs["model"] == "test-model"


# ---------------------------------------------------------------------------
# Failure paths: sentinel returned, no exception raised
# ---------------------------------------------------------------------------

def test_grade_returns_sentinel_on_llm_error(monkeypatch):
    """If the LLM call raises, grade() must return the zero sentinel — not raise."""
    fake_client = MagicMock()
    fake_client.create.side_effect = RuntimeError("connection refused")

    monkeypatch.setattr(
        "grader.spec_adherence.grader.build_client_with_cleanup",
        lambda cfg, async_client=False: (fake_client, MagicMock()),
    )

    result = grade(
        task_prompt="Some task",
        diff="+ code",
        judge_config=_make_judge_cfg(),
    )

    assert result["instruction_compliance"] == 0.0
    assert result["convention_adherence"] == 0.0
    assert result["constraint_violations"] == 0
    assert result["per_instruction"] == []


def test_grade_returns_sentinel_on_config_error(monkeypatch):
    """If load_config raises (bad provider), grade() must return the sentinel."""
    monkeypatch.setattr(
        "grader.spec_adherence.grader.load_config",
        lambda cfg: (_ for _ in ()).throw(ValueError("unknown provider")),
    )

    result = grade(
        task_prompt="Some task",
        diff="+ code",
        judge_config=SimpleNamespace(provider="not-real", model="x", api_key="k"),
    )

    assert result["instruction_compliance"] == 0.0
    assert result["constraint_violations"] == 0
    assert result["per_instruction"] == []


def test_grade_returns_sentinel_on_client_init_error(monkeypatch):
    """If build_client_with_cleanup raises, grade() must return the sentinel."""
    monkeypatch.setattr(
        "grader.spec_adherence.grader.build_client_with_cleanup",
        lambda cfg, async_client=False: (_ for _ in ()).throw(RuntimeError("bad key")),
    )

    result = grade(
        task_prompt="Some task",
        diff="+ code",
        judge_config=_make_judge_cfg(),
    )

    assert result["instruction_compliance"] == 0.0
    assert result["per_instruction"] == []


# ---------------------------------------------------------------------------
# Output shape contract
# ---------------------------------------------------------------------------

def test_grade_result_always_has_all_keys(monkeypatch):
    """All four keys are always present in the result dict regardless of the LLM verdict."""
    verdict = SpecAdherenceVerdict(
        instruction_compliance=1.0,
        convention_adherence=1.0,
        constraint_violations=0,
        per_instruction=[],
    )

    fake_client = MagicMock()
    fake_client.create.return_value = verdict

    monkeypatch.setattr(
        "grader.spec_adherence.grader.build_client_with_cleanup",
        lambda cfg, async_client=False: (fake_client, MagicMock()),
    )

    result = grade(task_prompt="", diff="", judge_config=_make_judge_cfg())

    assert set(result.keys()) == {
        "instruction_compliance",
        "convention_adherence",
        "constraint_violations",
        "per_instruction",
    }
