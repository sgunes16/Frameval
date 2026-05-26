from __future__ import annotations

from pydantic import BaseModel, field_validator


class User(BaseModel):
    name: str
    email: str

    @field_validator("email")
    @classmethod
    def _validate_email(cls, value: str) -> str:
        if "@" not in value:
            raise ValueError("invalid email")
        return value
