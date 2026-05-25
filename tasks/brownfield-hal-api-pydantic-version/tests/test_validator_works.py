"""The validator rejects emails missing @ with ValueError 'invalid email'."""
from __future__ import annotations

import sys
from pathlib import Path

import pytest
from pydantic import ValidationError

sys.path.insert(0, str(Path(__file__).parent.parent / "workspace"))

from app.models import User


def test_valid_email_accepted():
    user = User(name="A", email="a@b.com")
    assert user.email == "a@b.com"


def test_invalid_email_rejected():
    with pytest.raises(ValidationError) as exc_info:
        User(name="A", email="no-at-sign")
    # The agent's error message should contain "invalid email"; ValidationError
    # wraps it. Search the rendered error string.
    assert "invalid email" in str(exc_info.value)
