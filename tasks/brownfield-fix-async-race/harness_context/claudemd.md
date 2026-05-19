# Task: fix the async race in `add_credits`

You are an autonomous coding agent working in this repository. The workspace contains a small FastAPI service whose `add_credits()` function has a race condition under concurrent requests. Fix it.

## Scope rules

Touch **only** `app/user_service.py`. Do not modify other application files, tests, or configuration. The harness verifies scope via `git diff --name-only baseline`; any other modified path fails the run.

Do not change:
- The function signature of `add_credits`.
- The API contract: URL path, request body schema, response schema, or status codes.

## Style + correctness rules

- The codebase uses FastAPI + asyncio. Reach for an `asyncio.Lock`, not a `threading.Lock` — the latter blocks the event loop and is a `WRONG_ABS` failure for this task.
- Keep the fix minimal. A single shared lock guarding the read-modify-write inside `add_credits` is enough.
- Preserve existing imports and structure where it doesn't conflict with the fix.
- Do not introduce new dependencies.

## Verification

When you believe the fix is in place, run the task's tests to verify. The acceptance criteria are:

1. `pytest -q tests/test_race_fixed.py` — 100 concurrent `+1` requests must end with credits == 100.
2. `pytest -q tests/test_api_unchanged.py` — `GET /users/{id}` response shape must remain `{"id": int, "name": str, "credits": int}`.
3. `bash tests/test_scope.sh` — only `app/user_service.py` is modified vs the baseline commit.

## Common pitfalls

- **Using `threading.Lock` instead of `asyncio.Lock`** — blocks the event loop; a `WRONG_ABS` failure here.
- **Modifying the function signature** to thread the lock through — the test asserts the original signature is preserved.
- **Touching the route handler in `app/main.py`** to add locking there — scope check fails.
- **Adding a global lock at module import time** without scoping it to the user service — acceptable, but a class-level lock keyed by user_id is the cleaner idiom.

Stop and report when the tests pass.
