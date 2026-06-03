"""The User validators enforce two rules with the Pydantic v2 API:

  1. email must contain '@'        -> ValueError("invalid email")
  2. password_confirm == password  -> ValueError("passwords do not match")

Rule 2 is cross-field. The common v1->v2 mistake is a field_validator that
keeps the v1 `values` dict; under v2 the second positional arg is a
ValidationInfo (not subscriptable), so a naive `values["password"]` raises
TypeError on EVERY construction -- which is why test_valid_user_accepted (a
fully valid user) is the load-bearing trap: a buggy cross-field validator
fails it even though nothing about the input is invalid.
"""
from __future__ import annotations

import sys
from pathlib import Path

import pytest
from pydantic import ValidationError

sys.path.insert(0, str(Path(__file__).parent.parent))

from app.models import User


def _valid(**overrides):
    base = dict(
        name="A",
        email="a@b.com",
        password="hunter2pw",
        password_confirm="hunter2pw",
    )
    base.update(overrides)
    return base


def test_valid_user_accepted():
    user = User(**_valid())
    assert user.email == "a@b.com"
    assert user.password == user.password_confirm


def test_invalid_email_rejected():
    with pytest.raises(ValidationError) as exc_info:
        User(**_valid(email="no-at-sign"))
    assert "invalid email" in str(exc_info.value)


def test_password_mismatch_rejected():
    with pytest.raises(ValidationError) as exc_info:
        User(**_valid(password_confirm="different"))
    assert "passwords do not match" in str(exc_info.value)
