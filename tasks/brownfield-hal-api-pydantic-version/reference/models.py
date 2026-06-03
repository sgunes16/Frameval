from __future__ import annotations

from pydantic import BaseModel, field_validator


class User(BaseModel):
    name: str
    email: str = ""

    @field_validator("email")
    @classmethod
    def _validate_email(cls, value: str) -> str:
        # Blank email is a valid legacy state (see README) — only non-empty
        # addresses are checked for an '@'. The one-shot trap is to drop the
        # `value and` guard, which regresses legacy blank-email records.
        if value and "@" not in value:
            raise ValueError("invalid email")
        return value
