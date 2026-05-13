"""Pydantic schema tests for the failure taxonomy.

Pure-data tests — never construct a FailureClassifier (no network, no
SDK dependency). Verifies the type-level constraints we rely on at the
gRPC boundary.
"""
from __future__ import annotations

import pytest
from pydantic import ValidationError

from grader.failure_classifier.taxonomy import (
    EvidenceSpan,
    FailureClassification,
    FailureCode,
    FAILURE_DESCRIPTIONS,
)


def test_failure_code_has_all_13_values():
    expected = {
        "NONE", "HAL_API", "HAL_FILE", "DEP_MISS",
        "STOP_EARLY", "STOP_GIVEUP", "LOOP_INF", "WRONG_ABS",
        "MISREAD", "ENV_ERR", "SCOPE_DRIFT", "TIMEOUT", "SILENT_SKIP",
    }
    assert {c.value for c in FailureCode} == expected


def test_failure_descriptions_cover_every_code():
    for code in FailureCode:
        assert code in FAILURE_DESCRIPTIONS, f"missing description for {code.value}"
        assert FAILURE_DESCRIPTIONS[code], f"empty description for {code.value}"


def test_evidence_span_quote_capped_at_300():
    long_quote = "x" * 301
    with pytest.raises(ValidationError):
        EvidenceSpan(code=FailureCode.HAL_API, quote=long_quote, turn_index=0)


def test_evidence_span_requires_nonneg_turn_index():
    with pytest.raises(ValidationError):
        EvidenceSpan(code=FailureCode.HAL_API, quote="ok", turn_index=-1)


def test_classification_happy_path():
    c = FailureClassification(
        primary=FailureCode.HAL_API,
        secondary=[FailureCode.DEP_MISS],
        evidence=[
            EvidenceSpan(code=FailureCode.HAL_API, quote="AttributeError: no such method", turn_index=3),
        ],
        confidence=0.85,
        rationale="agent invoked nonexistent fastapi.bodyParser",
    )
    assert c.primary == FailureCode.HAL_API
    assert c.secondary == [FailureCode.DEP_MISS]
    assert c.confidence == 0.85


def test_classification_secondary_capped_at_3():
    with pytest.raises(ValidationError):
        FailureClassification(
            primary=FailureCode.HAL_API,
            secondary=[FailureCode.DEP_MISS, FailureCode.STOP_EARLY, FailureCode.MISREAD, FailureCode.SCOPE_DRIFT],
            confidence=0.5,
        )


def test_none_primary_rejects_secondary():
    with pytest.raises(ValidationError):
        FailureClassification(
            primary=FailureCode.NONE,
            secondary=[FailureCode.HAL_API],
            confidence=0.9,
        )


def test_none_in_secondary_rejected():
    with pytest.raises(ValidationError):
        FailureClassification(
            primary=FailureCode.HAL_API,
            secondary=[FailureCode.NONE],
            confidence=0.9,
        )


def test_confidence_clamped_via_validator():
    with pytest.raises(ValidationError):
        FailureClassification(primary=FailureCode.NONE, confidence=1.5)
    with pytest.raises(ValidationError):
        FailureClassification(primary=FailureCode.NONE, confidence=-0.1)


def test_rationale_capped_at_400():
    long_rationale = "y" * 401
    with pytest.raises(ValidationError):
        FailureClassification(
            primary=FailureCode.HAL_API,
            confidence=0.5,
            rationale=long_rationale,
        )


def test_none_classification_with_empty_secondary_passes():
    c = FailureClassification(primary=FailureCode.NONE, confidence=0.95)
    assert c.primary == FailureCode.NONE
    assert c.secondary == []
