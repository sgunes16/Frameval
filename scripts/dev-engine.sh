#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ -f "$ROOT_DIR/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT_DIR/.env"
  set +a
fi

export FRAMEVAL_DB_PATH="${FRAMEVAL_DB_PATH:-$ROOT_DIR/frameval.db}"
export FRAMEVAL_GRADER_ADDR="${FRAMEVAL_GRADER_ADDR:-localhost:50051}"
export FRAMEVAL_PORT="${FRAMEVAL_PORT:-8080}"
export FRAMEVAL_SANDBOX_IMAGE="${FRAMEVAL_SANDBOX_IMAGE:-frameval-sandbox:local}"
# Always resolve tasks root to an absolute path, even if .env supplies a relative one,
# because the engine binary runs from engine/ under air.
if [[ -n "${FRAMEVAL_TASKS_ROOT:-}" ]]; then
  case "$FRAMEVAL_TASKS_ROOT" in
    /*) : ;;
    *) FRAMEVAL_TASKS_ROOT="$ROOT_DIR/${FRAMEVAL_TASKS_ROOT#./}" ;;
  esac
else
  FRAMEVAL_TASKS_ROOT="$ROOT_DIR/tasks"
fi
export FRAMEVAL_TASKS_ROOT

if command -v docker >/dev/null 2>&1 && ! docker image inspect "$FRAMEVAL_SANDBOX_IMAGE" >/dev/null 2>&1; then
  echo "Missing sandbox image: $FRAMEVAL_SANDBOX_IMAGE" >&2
  echo "Build it once with: docker build -t $FRAMEVAL_SANDBOX_IMAGE -f docker/sandbox/Dockerfile ." >&2
fi

cd "$ROOT_DIR/engine"

if command -v air >/dev/null 2>&1; then
  exec air -c .air.toml
fi

exec go run github.com/air-verse/air@latest -c .air.toml
