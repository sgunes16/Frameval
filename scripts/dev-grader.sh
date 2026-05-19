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

cd "$ROOT_DIR/grader"

# Prefer uv (pyproject.toml has its lockfile); fall back to plain
# python so this script also works on systems without uv.
if command -v uv >/dev/null 2>&1; then
  exec uv run python server.py
fi

if [[ ! -d .venv ]]; then
  python3 -m venv .venv
  ./.venv/bin/pip install -q -e .
fi
exec ./.venv/bin/python server.py
