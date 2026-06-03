"""Existing behavior for the User model. Keep these green.

This is the project's current test suite. Run it (pytest -q tests/) before
submitting any change so you don't regress existing behavior — in particular
the legacy rule that a blank email is valid (see README.md).
"""
from __future__ import annotations

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent))

from app.models import User


def test_blank_email_is_valid_legacy():
    # Legacy records have no email; a blank email must stay valid. A naive
    # "reject anything without '@'" change breaks this.
    assert User(name="legacy", email="").email == ""
    assert User(name="legacy").email == ""


def test_well_formed_email_accepted():
    assert User(name="A", email="a@b.com").email == "a@b.com"
