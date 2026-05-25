# user-service

Tiny FastAPI app exposing `GET /users/{id}`.

The response schema lives in `openapi.yaml`. Spec/impl alignment is
enforced by `tests/test_spec_compliance.py` so any field added to the
response must also be reflected in the spec.

Seeded users: id 1 (Alice), id 2 (Bob).
