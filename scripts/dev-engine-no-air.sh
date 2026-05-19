#!/usr/bin/env bash
set -euo pipefail

# dev-engine-no-air — start the Go engine with `go run` instead of
# air. No file-watching, no rebuild loops; the server runs until
# you Ctrl-C it. Use this when you want a stable engine for
# UI / browser testing where Air's constant rebuilds were dropping
# open WebSocket connections.
#
# To pick up engine code changes you need to Ctrl-C and re-run.

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

if [[ -z "${DOCKER_HOST:-}" ]] && command -v docker >/dev/null 2>&1; then
  DOCKER_HOST="$(docker context inspect --format '{{json .Endpoints.docker.Host}}' 2>/dev/null | tr -d '"')"
  export DOCKER_HOST
fi

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
exec go run ./cmd/server
