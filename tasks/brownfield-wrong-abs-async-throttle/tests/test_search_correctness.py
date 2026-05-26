"""Single-request correctness."""
from __future__ import annotations

import pytest


@pytest.mark.asyncio
async def test_search_returns_expected_shape(client):
    response = await client.get("/search?q=foo")
    assert response.status_code == 200
    body = response.json()
    assert body["query"] == "foo"
    assert len(body["hits"]) == 3
