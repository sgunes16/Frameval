"""Test-side fixtures for the failure_classifier package.

Autouse fixture resets the module-level _default classifier between tests
so a test that calls `set_default_classifier(fake)` can't leak the fake
into a sibling test that expects the lazy-init path.
"""
from __future__ import annotations

import pytest

from grader.failure_classifier.grader import reset_default_classifier


@pytest.fixture(autouse=True)
def _reset_default_classifier():
    """Reset the module-level _default classifier before AND after each test."""
    reset_default_classifier()
    yield
    reset_default_classifier()
