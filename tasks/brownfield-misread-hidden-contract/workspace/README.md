# user-service

Tiny FastAPI app exposing `GET /users/{user_id}`. Seeded users live in
`app/users.py` (id 1 = Alice, id 2 = Bob).

## Layout

```
app/         FastAPI application code
configs/     Runtime configuration (logging, feature flags)
docs/        Service docs (contracts, ADRs, runbooks)
requirements.txt
```

## Running

```
uvicorn app.main:app
```

## Tests

CI runs the full `tests/` suite; locally `pytest -q` is enough. The
suite includes a few cross-cutting checks beyond the unit tests, so
glance at `tests/` if something fails unexpectedly.
