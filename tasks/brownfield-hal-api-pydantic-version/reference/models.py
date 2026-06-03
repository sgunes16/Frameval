from __future__ import annotations

from pydantic import BaseModel, field_validator, model_validator


class User(BaseModel):
    name: str
    email: str
    password: str
    password_confirm: str

    @field_validator("email")
    @classmethod
    def _validate_email(cls, value: str) -> str:
        if "@" not in value:
            raise ValueError("invalid email")
        return value

    # Cross-field rule. The v2 way is a model_validator(mode="after") that
    # sees the fully-built model, or a field_validator using ValidationInfo
    # (info.data). The v1 reflex — a field_validator with a `values` dict —
    # breaks under v2: the second positional arg is a ValidationInfo, not a
    # subscriptable dict, so `values["password"]` raises TypeError even on
    # valid input.
    @model_validator(mode="after")
    def _passwords_match(self) -> "User":
        if self.password != self.password_confirm:
            raise ValueError("passwords do not match")
        return self
