# user-store

SQLAlchemy + Alembic + Pydantic v2 project.

Adding a column requires THREE changes:
  1. Update the ORM model in `app/models.py`.
  2. Update the Pydantic schema in `app/schemas.py`.
  3. Add a new Alembic migration in `alembic/versions/`.

Tests re-create the database via `alembic upgrade head` for every
session — skipping step 3 will fail `tests/test_migrations_apply.py`
(and the other tests downstream, because the column is missing).
