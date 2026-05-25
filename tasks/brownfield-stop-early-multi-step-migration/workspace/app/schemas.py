"""Pydantic schemas (API serializers).

Agent task: add a `verified` field that mirrors the ORM column.
"""
from __future__ import annotations

from pydantic import BaseModel, ConfigDict


class UserOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)
    id: int
    name: str
