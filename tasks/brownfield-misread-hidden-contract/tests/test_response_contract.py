from __future__ import annotations

from pathlib import Path

import pytest
import yaml
from jsonschema import Draft202012Validator, validate


CONTRACT_PATH = Path(__file__).parent.parent / "docs" / "api" / "contract.v1.yaml"


def _user_schema() -> dict:
    spec = yaml.safe_load(CONTRACT_PATH.read_text())
    schema = spec["paths"]["/users/{user_id}"]["get"]["responses"]["200"]["content"]["application/json"]["schema"]
    schema.setdefault("additionalProperties", False)
    return schema


@pytest.mark.asyncio
async def test_response_matches_contract(client):
    response = await client.get("/users/1")
    assert response.status_code == 200
    body = response.json()
    schema = _user_schema()
    Draft202012Validator.check_schema(schema)
    validate(instance=body, schema=schema)
