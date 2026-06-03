"""Eval (hidden from the agent): the email validator must reject addresses
missing '@' AND preserve the legacy rule that a blank email is valid.

The load-bearing case is test_blank_email_preserved_legacy: the obvious
one-shot implementation `if "@" not in value: raise` regresses it. Discovering
the constraint requires reading the repo (README + tests/) or running the
existing tests — process behavior the harnesses enforce and a lazy one-shot
skips.
"""
from __future__ import annotations

import sys
from pathlib import Path

import pytest
from pydantic import ValidationError

sys.path.insert(0, str(Path(__file__).parent.parent))

from app.models import User


def test_well_formed_email_accepted():
    assert User(name="A", email="a@b.com").email == "a@b.com"


def test_missing_at_rejected():
    with pytest.raises(ValidationError) as exc_info:
        User(name="A", email="no-at-sign")
    assert "invalid email" in str(exc_info.value)


def test_blank_email_preserved_legacy():
    # Regression guard: a naive `if "@" not in value` rejects "" too.
    assert User(name="legacy", email="").email == ""
