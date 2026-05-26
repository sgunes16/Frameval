"""Reference solution — Pydantic schema with verified field."""
from __future__ import annotations

from pydantic import BaseModel, ConfigDict


class UserOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)
    id: int
    name: str
    verified: bool
