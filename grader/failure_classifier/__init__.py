"""Failure classifier — the only LLM stage of AgentDx.

Public entry points re-exported here for convenience.
"""
from grader.failure_classifier.grader import (
    FailureClassifier,
    classify,
)
from grader.failure_classifier.taxonomy import (
    FailureCode,
    EvidenceSpan,
    FailureClassification,
)

__all__ = [
    "FailureClassifier",
    "classify",
    "FailureCode",
    "EvidenceSpan",
    "FailureClassification",
]
