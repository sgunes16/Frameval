# user-credits service

A small FastAPI app exposing a single user-credits API.

## Routes

- `GET /users/{user_id}` — returns `{id, name, credits}` or `{error}`.
- `POST /users/{user_id}/credits` — body `{"amount": int}`; returns
  `{id, credits}`.

## Known bug

`app/user_service.py::add_credits` has a race condition under concurrent
requests. Read-modify-write across an `await` boundary with no lock means
two coroutines can both compute `baseline + delta` from the same baseline
and one increment is lost.

The fix is the agent's job. The canonical solution is an asyncio.Lock —
see the task description for constraints (no signature change, no API
contract change, no scope drift).
