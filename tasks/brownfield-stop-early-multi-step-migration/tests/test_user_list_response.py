"""Serialize a User through the Pydantic schema and assert verified is present.

Partial fix (model only) makes the ORM object carry verified but if the
Pydantic schema is untouched, model_dump() will not include the field.
"""
from __future__ import annotations

from app.models import User
from app.schemas import UserOut


def test_pydantic_user_serializes_verified():
    u = User(id=1, name="Alice", verified=True)
    payload = UserOut.model_validate(u).model_dump()
    assert "verified" in payload, (
        f"'verified' missing from UserOut serialization; got keys: {list(payload)}"
    )
    assert payload["verified"] is True
    assert payload == {"id": 1, "name": "Alice", "verified": True}
