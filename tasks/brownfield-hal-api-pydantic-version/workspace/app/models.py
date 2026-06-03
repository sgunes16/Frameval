"""User model.

Agent task: add validators to this model. Use the Pydantic 2.x API
(`field_validator` / `model_validator`). The v1 `@validator` /
`@root_validator` decorators are deprecated and our import-discipline
test rejects them.

Two rules are required (see the task prompt):
  1. `email` must contain an `@`.
  2. `password_confirm` must equal `password` (a cross-field rule).
"""
from __future__ import annotations

from pydantic import BaseModel


class User(BaseModel):
    name: str
    email: str
    password: str
    password_confirm: str
