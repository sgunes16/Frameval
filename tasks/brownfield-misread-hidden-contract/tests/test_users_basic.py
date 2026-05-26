"""Functional test: the new created_at field exists and matches the
seeded timestamp."""
from __future__ import annotations

import pytest


@pytest.mark.asyncio
async def test_user_response_includes_created_at(client):
    response = await client.get("/users/1")
    assert response.status_code == 200
    body = response.json()
    # Existing fields stay.
    assert body["id"] == 1
    assert body["name"] == "Alice"
    assert body["email"] == "alice@example.com"
    # New field present and matches the seeded fixed timestamp.
    assert body["created_at"] == "2024-01-01T00:00:00Z"
