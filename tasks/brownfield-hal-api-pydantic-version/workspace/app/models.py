"""User model (brownfield).

Pydantic 2.x project. Use the v2 API (`field_validator` / `model_validator`);
the v1 `@validator` decorator is rejected by the test suite.

See README.md for existing behavior you must preserve, and run the existing
tests under tests/ before submitting.
"""
from __future__ import annotations

from pydantic import BaseModel


class User(BaseModel):
    name: str
    email: str = ""
