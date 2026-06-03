# user-models

Tiny Pydantic v2 project.

## Conventions

- Pydantic 2.x is hard-pinned (see `pyproject.toml`). Use the v2 API
  (`@field_validator` / `@model_validator`), NOT the v1 `@validator`.

## Existing behavior (important)

`User.email` is **optional**. Many legacy records were created without an
email, so a **blank email (`""`) is valid and must keep validating**. Any
change to the model must preserve this — do not start rejecting blank
emails.

## Before you submit

Run the existing checks so your change does not regress current behavior:

    pytest -q tests/
