"""User model.

Agent task: add a validator that rejects emails without an @ symbol.
Use Pydantic 2.x API (field_validator). The v1 @validator decorator
is deprecated and our import-discipline test rejects it.
"""
from __future__ import annotations

from pydantic import BaseModel


class User(BaseModel):
    name: str
    email: str
