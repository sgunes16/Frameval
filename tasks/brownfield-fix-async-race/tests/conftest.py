"""Pytest fixtures for the async-race task.

Adds the workspace directory to sys.path so `from app...` imports resolve
to the agent's modified codebase. Tests live OUTSIDE the workspace so the
agent never sees them.
"""
from __future__ import annotations

import sys
from pathlib import Path

WORKSPACE = Path(__file__).resolve().parent.parent / "workspace"
if str(WORKSPACE) not in sys.path:
    sys.path.insert(0, str(WORKSPACE))
