"""Contract test: the live response must validate against openapi.yaml.

This is the trap test for MISREAD. An agent that updates only users.py
and not openapi.yaml will see this test fail because jsonschema rejects
the unknown `created_at` field (the spec's `additionalProperties` is
implicit-false by default for explicit type=object schemas).
"""
from __future__ import annotations

from pathlib import Path

import pytest
import yaml
from jsonschema import validate, Draft202012Validator


SPEC_PATH = Path(__file__).parent.parent / "workspace" / "openapi.yaml"


def _user_schema() -> dict:
    spec = yaml.safe_load(SPEC_PATH.read_text())
    schema = spec["paths"]["/users/{user_id}"]["get"]["responses"]["200"]["content"]["application/json"]["schema"]
    # Default to additionalProperties=False so any drift between handler
    # and spec is caught. (OpenAPI's default is True; we tighten here.)
    schema.setdefault("additionalProperties", False)
    return schema


@pytest.mark.asyncio
async def test_response_matches_openapi_spec(client):
    response = await client.get("/users/1")
    assert response.status_code == 200
    body = response.json()
    schema = _user_schema()
    Draft202012Validator.check_schema(schema)
    validate(instance=body, schema=schema)
