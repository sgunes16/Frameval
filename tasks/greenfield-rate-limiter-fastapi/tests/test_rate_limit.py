"""Tests for the per-IP rate-limited /api/data endpoint.

Uses FastAPI's TestClient (synchronous, runs the app in-process). The
window-reset test uses freezegun to advance time deterministically rather
than sleeping for real 60 seconds.
"""
from __future__ import annotations

import pytest
from fastapi.testclient import TestClient
from freezegun import freeze_time


def _client(app):
    return TestClient(app)


def test_first_10_ok(fresh_app):
    """The first 10 requests from a given IP return 200 with the success body."""
    client = _client(fresh_app)
    for i in range(10):
        r = client.get("/api/data", headers={"X-Forwarded-For": "1.2.3.4"})
        assert r.status_code == 200, f"request {i + 1} returned {r.status_code}: {r.text}"
        body = r.json()
        assert body == {"data": "ok"}, f"request {i + 1} body mismatch: {body}"


def test_11th_returns_429(fresh_app):
    """The 11th request from the same IP returns 429 with the documented body."""
    client = _client(fresh_app)
    for _ in range(10):
        r = client.get("/api/data", headers={"X-Forwarded-For": "5.6.7.8"})
        assert r.status_code == 200, r.text

    r11 = client.get("/api/data", headers={"X-Forwarded-For": "5.6.7.8"})
    assert r11.status_code == 429, f"expected 429, got {r11.status_code}: {r11.text}"
    assert r11.json() == {"error": "rate limit exceeded"}, f"unexpected body: {r11.json()}"


def test_window_resets(fresh_app):
    """After 60 seconds, the per-IP budget refills and the next request succeeds.

    freezegun advances the clock without a real sleep so the test stays fast.
    """
    client = _client(fresh_app)
    headers = {"X-Forwarded-For": "9.9.9.9"}
    with freeze_time("2026-05-12 12:00:00") as frozen:
        for _ in range(10):
            assert client.get("/api/data", headers=headers).status_code == 200
        # Bucket exhausted
        assert client.get("/api/data", headers=headers).status_code == 429
        # Move past the 60-second window
        frozen.tick(61.0)
        # Should succeed again
        r = client.get("/api/data", headers=headers)
        assert r.status_code == 200, f"after window reset expected 200, got {r.status_code}: {r.text}"


def test_per_ip_independent(fresh_app):
    """Different X-Forwarded-For values have independent buckets."""
    client = _client(fresh_app)
    for _ in range(10):
        assert client.get("/api/data", headers={"X-Forwarded-For": "1.1.1.1"}).status_code == 200
    # IP 1 is now exhausted
    assert client.get("/api/data", headers={"X-Forwarded-For": "1.1.1.1"}).status_code == 429
    # IP 2 should still have a full bucket
    r = client.get("/api/data", headers={"X-Forwarded-For": "2.2.2.2"})
    assert r.status_code == 200, f"IP 2 should be unaffected; got {r.status_code}: {r.text}"
