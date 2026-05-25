"""Pytest session fixture: fresh sqlite + alembic upgrade head per test."""
from __future__ import annotations

import os
import shutil
import subprocess
import sys
from pathlib import Path

import pytest

WORKSPACE = Path(__file__).parent.parent / "workspace"
sys.path.insert(0, str(WORKSPACE))


@pytest.fixture(autouse=True)
def fresh_db(tmp_path, monkeypatch):
    """Create a clean sqlite DB, run alembic upgrade head, then patch app.db."""
    db_file = tmp_path / "test.db"

    # Write a patched alembic.ini pointing at the ephemeral DB.
    alembic_ini = tmp_path / "alembic.ini"
    text = (WORKSPACE / "alembic.ini").read_text()
    text = text.replace("sqlite:///./test.db", f"sqlite:///{db_file}")
    alembic_ini.write_text(text)

    # Copy the alembic/ dir alongside the patched ini so alembic can find migrations.
    shutil.copytree(WORKSPACE / "alembic", tmp_path / "alembic")

    # Run migrations.
    result = subprocess.run(
        ["alembic", "-c", str(alembic_ini), "upgrade", "head"],
        cwd=tmp_path,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        raise RuntimeError(
            f"alembic upgrade head failed:\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )

    # Re-point app.db.engine and SessionLocal at the fresh DB.
    from sqlalchemy import create_engine
    from sqlalchemy.orm import sessionmaker
    import app.db as db_module

    new_engine = create_engine(f"sqlite:///{db_file}", future=True)
    db_module.engine = new_engine
    db_module.SessionLocal = sessionmaker(
        bind=new_engine, autoflush=False, autocommit=False, future=True
    )

    yield

    # Cleanup: dispose of the engine so the file can be removed on Windows too.
    new_engine.dispose()
