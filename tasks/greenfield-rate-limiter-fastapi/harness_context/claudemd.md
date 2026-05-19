# Task: build a FastAPI rate limiter

You are an autonomous coding agent working in an empty workspace. Build a FastAPI middleware that rate-limits incoming requests by client IP.

## Requirements

- Single-file app at `app.py` exposing a FastAPI instance named `app`.
- Middleware: token-bucket per client IP, 10 requests per minute by default.
- A single `GET /ping` route that returns `{"pong": true}` when allowed.
- When the bucket is empty, return `HTTP 429` with body `{"detail": "rate limited"}` and a `Retry-After` header (integer seconds).

## Style + correctness rules

- Stdlib + FastAPI only. Do not pull in `slowapi`, `limits`, or `aiolimiter` — the point is to implement the limiter.
- Use `asyncio.Lock` (not `threading.Lock`) to protect the bucket state.
- Pluggable rate via an environment variable `RATE_LIMIT_PER_MINUTE`. Default 10.
- Identify the client by `request.client.host`. Do not honour `X-Forwarded-For` — the harness tests against a direct ASGI transport.
- Bucket state is in-memory. No external store, no persistence.

## Verification

Acceptance:

1. `pytest -q tests/test_allows_under_limit.py` — 10 requests in a minute all return 200.
2. `pytest -q tests/test_blocks_over_limit.py` — the 11th request returns 429 with the documented body and `Retry-After`.
3. `pytest -q tests/test_separate_clients.py` — two distinct client IPs each get their own bucket.

## Common pitfalls

- **Using a thread lock** — blocks the event loop on every request; the test that hammers concurrent requests will time out.
- **Counting requests instead of replenishing tokens** — a fixed-window counter resets sharply at the minute boundary; the spec is a token bucket that refills smoothly.
- **Sharing bucket state across clients** — the per-IP key must be the actual IP, not a constant.
- **Returning 200 with an error body** instead of the documented 429 + JSON body.

Stop and report when the three tests above pass.
