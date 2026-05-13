"""Pydantic types + enum for the AgentDx failure taxonomy.

The 12 failure categories + a NONE sentinel are the canonical multi-label
classification schema downstream consumers see. Definitions and intended
evidence patterns live in the AgentDx design spec, Appendix A.

This module is kept import-free of anthropic / instructor so it can be
imported into the gRPC proto layer or test fixtures without triggering the
LLM dependency tree.
"""
from __future__ import annotations

from enum import Enum
from typing import List

from pydantic import BaseModel, Field, model_validator


class FailureCode(str, Enum):
    """Canonical AgentDx failure-mode taxonomy.

    12 failure modes plus NONE for clean runs. String-valued so it
    serializes to JSON / Protobuf as a stable identifier.
    """

    NONE = "NONE"
    HAL_API = "HAL_API"
    HAL_FILE = "HAL_FILE"
    DEP_MISS = "DEP_MISS"
    STOP_EARLY = "STOP_EARLY"
    STOP_GIVEUP = "STOP_GIVEUP"
    LOOP_INF = "LOOP_INF"
    WRONG_ABS = "WRONG_ABS"
    MISREAD = "MISREAD"
    ENV_ERR = "ENV_ERR"
    SCOPE_DRIFT = "SCOPE_DRIFT"
    TIMEOUT = "TIMEOUT"
    SILENT_SKIP = "SILENT_SKIP"


# Short, classifier-prompt-friendly descriptions for each label. Used both
# in the LLM prompt rendering AND surfaced to the frontend as tooltip text.
FAILURE_DESCRIPTIONS: dict[FailureCode, str] = {
    FailureCode.NONE: "Run completed without significant issues; tests pass and there is no failure evidence.",
    FailureCode.HAL_API: "Hallucinated API — used a function/method/parameter that does not exist in the library.",
    FailureCode.HAL_FILE: "Phantom file — referenced a file that was never created or expected file in wrong location.",
    FailureCode.DEP_MISS: "Missing dependency — used a package without installing it or declaring it in requirements.",
    FailureCode.STOP_EARLY: "Premature completion — declared the task done while tests/build are still failing.",
    FailureCode.STOP_GIVEUP: "Surrender — declared inability to proceed without exhausting reasonable options.",
    FailureCode.LOOP_INF: "Infinite loop / no progress — repeated the same action across iterations with no state change.",
    FailureCode.WRONG_ABS: "Wrong abstraction — solution structure does not match the task (e.g., sync when async required).",
    FailureCode.MISREAD: "Spec misread — solution targets the wrong requirement (changed wrong function, broke contract).",
    FailureCode.ENV_ERR: "Environment failure — failure caused by sandbox or tool infrastructure, not the agent.",
    FailureCode.SCOPE_DRIFT: "Scope drift — modified files outside the expected scope for a brownfield task.",
    FailureCode.TIMEOUT: "Wall-clock timeout — run exceeded the time budget before completion.",
    FailureCode.SILENT_SKIP: "Silent failure — agent encountered an error and ignored it in subsequent turns.",
}


class EvidenceSpan(BaseModel):
    """A verbatim quote from the transcript justifying a failure label.

    Pydantic-validated so the classifier output passes through the gRPC
    boundary without surprises. `quote` length is capped to keep the
    serialized response small.
    """

    code: FailureCode
    quote: str = Field(max_length=300, description="Verbatim transcript text supporting the label")
    turn_index: int = Field(ge=0, description="0-based turn index in the original transcript")


class FailureClassification(BaseModel):
    """Multi-label, evidence-grounded failure verdict.

    Constraints enforced at the type level:
        * primary is always set (NONE for clean runs)
        * secondary holds up to 3 codes; never includes NONE
        * NONE is mutually exclusive with all other codes
        * confidence ∈ [0, 1]
        * rationale capped at 400 chars to keep responses small
    """

    primary: FailureCode
    secondary: List[FailureCode] = Field(default_factory=list, max_length=3)
    evidence: List[EvidenceSpan] = Field(default_factory=list)
    confidence: float = Field(ge=0.0, le=1.0)
    rationale: str = Field(max_length=400, default="")

    @model_validator(mode="after")
    def _none_is_exclusive(self) -> "FailureClassification":
        # NONE must not co-occur with any failure label, in either field.
        if self.primary == FailureCode.NONE and self.secondary:
            raise ValueError(
                "FailureClassification.primary=NONE requires empty secondary; "
                f"got secondary={self.secondary}"
            )
        if FailureCode.NONE in self.secondary:
            raise ValueError("FailureClassification.secondary must not contain NONE")
        return self
