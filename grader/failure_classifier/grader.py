"""Failure-classifier callable — the only LLM stage in AgentDx.

Wraps Anthropic's Claude Haiku 4.5 with `instructor` so the response is
guaranteed to satisfy the Pydantic `FailureClassification` schema (with
retries on validation failures). On hard failure, returns the
`unclassified()` sentinel so the orchestrator never crashes mid-pipeline.

The classify() top-level function is the simple entry point used by the
gRPC handler (Story #23); FailureClassifier is the instantiable class
when you need a custom model / client (e.g., calibration ablation runs
with a different model id).
"""
from __future__ import annotations

import logging
import os
from typing import Mapping, Protocol

from grader.failure_classifier.prompts import SYSTEM_PROMPT, render_user_prompt
from grader.failure_classifier.taxonomy import (
    FailureClassification,
    FailureCode,
)

# Default model id; pinned to Haiku 4.5 per spec §4.7.4.
DEFAULT_MODEL = "claude-haiku-4-5-20251001"

# Hard upper bound on retries the instructor wrapper performs internally.
# Each retry costs an API call, so 2 is the sweet spot — see the
# instructor docs for the validation-failure retry semantics.
INSTRUCTOR_MAX_RETRIES = 2

# Cap response tokens. The classifier output (4 codes + a few short
# evidence quotes + 400-char rationale) fits in well under 1024 tokens.
RESPONSE_MAX_TOKENS = 1024


logger = logging.getLogger(__name__)


class _ClassifierClient(Protocol):
    """Narrow interface the FailureClassifier needs from the LLM client.

    Lets tests inject a fake without depending on the real anthropic /
    instructor SDKs. The actual client created in __init__ adheres to this
    via duck typing — instructor's wrapper returns a callable that takes
    these exact kwargs.
    """

    def create(
        self,
        *,
        model: str,
        max_tokens: int,
        response_model: type,
        max_retries: int,
        messages: list[dict[str, str]],
    ) -> FailureClassification: ...


class FailureClassifier:
    """Stateful classifier holding a configured LLM client.

    Construction is the only thing that touches the anthropic/instructor
    imports — pure-data callers (tests, gRPC marshalling) can avoid
    pulling that import surface entirely by going through `classify()`
    after `set_default_classifier()`, or by passing a fake client.
    """

    def __init__(
        self,
        *,
        model: str = DEFAULT_MODEL,
        client: _ClassifierClient | None = None,
    ) -> None:
        self.model = model
        self._client: _ClassifierClient | None = client

    def _client_lazy(self) -> _ClassifierClient:
        if self._client is not None:
            return self._client
        # Lazy import so test fixtures using injected clients don't need
        # ANTHROPIC_API_KEY or the anthropic SDK installed at import time.
        import instructor
        from anthropic import Anthropic

        api_key = os.getenv("ANTHROPIC_API_KEY")
        if not api_key:
            raise RuntimeError(
                "ANTHROPIC_API_KEY is not set — cannot construct default classifier client. "
                "Set the env var or pass a client= kwarg explicitly."
            )
        self._client = instructor.from_anthropic(Anthropic(api_key=api_key)).messages
        return self._client

    def classify(
        self,
        *,
        symptoms: Mapping[str, object],
        task_description: str,
        transcript_tail: str,
    ) -> FailureClassification:
        """Classify one run; never raises on LLM errors.

        Returns FailureClassification on success. On hard LLM failure
        (Pydantic still invalid after retries, network error, etc.) logs
        the exception and returns the `unclassified()` sentinel so the
        orchestrator can persist a row indicating "classifier ran but
        returned no usable verdict" rather than crashing the run.
        """
        try:
            client = self._client_lazy()
        except Exception as exc:  # noqa: BLE001 — boundary; we own the fallback
            logger.warning("classifier client init failed: %s", exc)
            return unclassified(str(exc))

        user_prompt = render_user_prompt(
            symptoms=symptoms,
            task_description=task_description,
            transcript_tail=transcript_tail,
        )
        try:
            return client.create(
                model=self.model,
                max_tokens=RESPONSE_MAX_TOKENS,
                response_model=FailureClassification,
                max_retries=INSTRUCTOR_MAX_RETRIES,
                messages=[
                    {"role": "system", "content": SYSTEM_PROMPT},
                    {"role": "user", "content": user_prompt},
                ],
            )
        except Exception as exc:  # noqa: BLE001 — boundary; never crash the orchestrator
            logger.warning("classifier call failed: %s", exc)
            return unclassified(str(exc))


_UNCLASSIFIED_PREFIX = "classifier_unavailable: "
_RATIONALE_CAP = 400


def unclassified(reason: str = "classifier call failed") -> FailureClassification:
    """Sentinel returned on hard LLM failures.

    primary=NONE, confidence=0, rationale carries the failure reason.
    The orchestrator can detect this case via `confidence == 0`.

    Reserves space for the fixed prefix so the rationale never overflows
    Pydantic's 400-char cap regardless of the supplied reason length.
    """
    reason_budget = _RATIONALE_CAP - len(_UNCLASSIFIED_PREFIX)
    truncated_reason = reason[:reason_budget]
    rationale = f"{_UNCLASSIFIED_PREFIX}{truncated_reason}"
    return FailureClassification(
        primary=FailureCode.NONE,
        secondary=[],
        evidence=[],
        confidence=0.0,
        rationale=rationale,
    )


# Module-level default classifier; lazily instantiated by classify().
_default: FailureClassifier | None = None


def set_default_classifier(classifier: FailureClassifier | None) -> None:
    """Override the module-level default — primarily for tests.

    Passing None resets state to the lazy-construct path so the next
    classify() call rebuilds a fresh default. Tests should call this with
    None in teardown to prevent cross-test bleed.
    """
    global _default
    _default = classifier


def reset_default_classifier() -> None:
    """Convenience alias for set_default_classifier(None)."""
    set_default_classifier(None)


def classify(
    *,
    symptoms: Mapping[str, object],
    task_description: str,
    transcript_tail: str,
) -> FailureClassification:
    """Convenience entry point used by the gRPC handler.

    Lazily constructs a FailureClassifier with the default Haiku model on
    first call. Tests should call `set_default_classifier(...)` with a
    fake-client FailureClassifier to avoid hitting the real API.
    """
    global _default
    if _default is None:
        _default = FailureClassifier()
    return _default.classify(
        symptoms=symptoms,
        task_description=task_description,
        transcript_tail=transcript_tail,
    )
