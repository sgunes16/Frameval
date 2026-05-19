#!/usr/bin/env bash
set -euo pipefail

# dev-grader — starts the Python grader sidecar on localhost:50051.
# Counterpart of dev-engine.sh; runs without Docker so the engine
# (also local) can dial 127.0.0.1:50051 directly.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ -f "$ROOT_DIR/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT_DIR/.env"
  set +a
fi

export GRADER_PORT="${GRADER_PORT:-50051}"
export FRAMEVAL_DB_PATH="${FRAMEVAL_DB_PATH:-$ROOT_DIR/frameval.db}"

# server.py uses absolute imports (`from grader.code_grader import …`)
# so `grader/` must be importable as a top-level package. Putting
# CWD inside `grader/` makes Python interpret those as circular
# relative imports — ModuleNotFoundError: No module named 'grader'.
# Run from the repo root and add it to PYTHONPATH so absolute
# imports resolve.
export PYTHONPATH="$ROOT_DIR:${PYTHONPATH:-}"
cd "$ROOT_DIR"

# Use module-mode (`-m grader.server`) so Python adds CWD (repo
# root) to sys.path and `from grader.X import …` resolves. Script
# mode (`python grader/server.py`) adds only the script's dir to
# sys.path, which makes the grader package itself unimportable.
# The docker/grader/Dockerfile CMD does the same.
if command -v uv >/dev/null 2>&1; then
  exec uv run --project grader python -m grader.server
fi

if [[ ! -d grader/.venv ]]; then
  python3 -m venv grader/.venv
  grader/.venv/bin/pip install -q grpcio grpcio-tools pydantic numpy scipy
fi
exec grader/.venv/bin/python -m grader.server
