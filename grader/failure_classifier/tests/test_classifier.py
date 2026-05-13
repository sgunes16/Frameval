"""FailureClassifier integration tests with a fake client.

Verifies the wrapper without touching the real Anthropic API. The
classifier's contract: never crash on LLM failure, return the
unclassified() sentinel instead.
"""
from __future__ import annotations

from typing import Any

import pytest

from grader.failure_classifier.grader import (
    DEFAULT_MODEL,
    FailureClassifier,
    classify,
    set_default_classifier,
    unclassified,
)
from grader.failure_classifier.taxonomy import (
    EvidenceSpan,
    FailureClassification,
    FailureCode,
)


class _RecordingFake:
    """Captures the create() call args and returns a canned response."""

    def __init__(self, response: FailureClassification | Exception):
        self.response = response
        self.last_call: dict[str, Any] | None = None

    def create(self, **kwargs: Any) -> FailureClassification:
        self.last_call = kwargs
        if isinstance(self.response, Exception):
            raise self.response
        return self.response


def _hap_api_response() -> FailureClassification:
    return FailureClassification(
        primary=FailureCode.HAL_API,
        secondary=[FailureCode.DEP_MISS],
        evidence=[
            EvidenceSpan(code=FailureCode.HAL_API, quote="AttributeError", turn_index=3),
        ],
        confidence=0.82,
        rationale="agent imported a nonexistent attribute",
    )


def test_classifier_forwards_messages_and_model():
    expected = _hap_api_response()
    fake = _RecordingFake(expected)
    cls = FailureClassifier(model="custom-model", client=fake)

    got = cls.classify(
        symptoms={"tests_failed": 3},
        task_description="build a CLI",
        transcript_tail="[0][assistant] tool: foo",
    )
    assert got == expected
    assert fake.last_call is not None
    assert fake.last_call["model"] == "custom-model"
    assert fake.last_call["max_retries"] >= 1
    assert fake.last_call["response_model"] is FailureClassification
    msgs = fake.last_call["messages"]
    assert msgs[0]["role"] == "system"
    assert msgs[1]["role"] == "user"
    # The taxonomy must appear in the system prompt so the model sees definitions.
    assert "HAL_API" in msgs[0]["content"]
    # The symptom packet must appear in the user prompt.
    assert "tests_failed" in msgs[1]["content"]
    # The transcript tail must appear in the user prompt.
    assert "tool: foo" in msgs[1]["content"]


def test_classifier_returns_unclassified_on_exception():
    fake = _RecordingFake(RuntimeError("API blew up"))
    cls = FailureClassifier(model=DEFAULT_MODEL, client=fake)

    got = cls.classify(symptoms={}, task_description="x", transcript_tail="")
    assert got.primary == FailureCode.NONE
    assert got.confidence == 0.0
    assert "API blew up" in got.rationale


def test_classifier_no_api_key_returns_unclassified(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.delenv("ANTHROPIC_API_KEY", raising=False)
    cls = FailureClassifier()  # no client kwarg → tries to lazy-init real one

    got = cls.classify(symptoms={}, task_description="x", transcript_tail="")
    assert got.primary == FailureCode.NONE
    assert got.confidence == 0.0
    assert "ANTHROPIC_API_KEY" in got.rationale


def test_unclassified_sentinel():
    sentinel = unclassified("some reason")
    assert sentinel.primary == FailureCode.NONE
    assert sentinel.secondary == []
    assert sentinel.confidence == 0.0
    assert "some reason" in sentinel.rationale


def test_module_level_classify_uses_default():
    fake = _RecordingFake(_hap_api_response())
    set_default_classifier(FailureClassifier(model="t", client=fake))

    got = classify(symptoms={"x": 1}, task_description="t", transcript_tail="")
    assert got.primary == FailureCode.HAL_API
    assert fake.last_call is not None


def test_unclassified_long_reason_respects_400_cap():
    # A pathologically long reason must still produce a valid
    # FailureClassification (rationale ≤ 400).
    long_reason = "x" * 1000
    sentinel = unclassified(long_reason)
    assert len(sentinel.rationale) <= 400
    assert sentinel.rationale.startswith("classifier_unavailable: ")
    assert sentinel.primary == FailureCode.NONE
