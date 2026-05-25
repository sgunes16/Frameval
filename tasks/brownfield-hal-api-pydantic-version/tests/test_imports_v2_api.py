"""Regex check enforcing Pydantic v2 API usage in app/models.py.

Bare agents with stale priors often write:
    from pydantic import validator
    @validator("email")

We reject that. v2 wants:
    from pydantic import field_validator
    @field_validator("email")
"""
from __future__ import annotations

import re
from pathlib import Path


MODELS_PATH = Path(__file__).parent.parent / "workspace" / "app" / "models.py"


def test_uses_field_validator_not_legacy_validator():
    text = MODELS_PATH.read_text()
    # Must import field_validator from pydantic.
    assert re.search(r"\bfield_validator\b", text), \
        "expected `field_validator` import (Pydantic v2 API)"
    # Must NOT import or use the legacy v1 `validator` decorator.
    # The regex excludes field_validator (the v2 name contains 'validator').
    illegal = re.findall(r"(?<!field_)\bvalidator\b", text)
    assert not illegal, \
        f"found legacy v1 `validator` decorator/import (got {illegal}); use field_validator"
