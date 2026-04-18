#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cleanup() {
  local status=0
  if [[ -n "${ENGINE_PID:-}" ]] && kill -0 "$ENGINE_PID" 2>/dev/null; then
    kill "$ENGINE_PID" 2>/dev/null || true
    wait "$ENGINE_PID" || status=$?
  fi
  if [[ -n "${FRONTEND_PID:-}" ]] && kill -0 "$FRONTEND_PID" 2>/dev/null; then
    kill "$FRONTEND_PID" 2>/dev/null || true
    wait "$FRONTEND_PID" || status=$?
  fi
  return "$status"
}

trap cleanup EXIT INT TERM

"$ROOT_DIR/scripts/dev-engine.sh" \
  > >(sed 's/^/[engine] /') \
  2> >(sed 's/^/[engine] /' >&2) &
ENGINE_PID=$!

ENGINE_PORT="${FRAMEVAL_PORT:-8080}"
ENGINE_HEALTH_URL="http://127.0.0.1:${ENGINE_PORT}/api/health"

echo "[dev-local] waiting for engine at ${ENGINE_HEALTH_URL}"
for _ in $(seq 1 120); do
  if ! kill -0 "$ENGINE_PID" 2>/dev/null; then
    wait "$ENGINE_PID"
    exit $?
  fi
  if command -v curl >/dev/null 2>&1 && curl -fsS "$ENGINE_HEALTH_URL" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if ! curl -fsS "$ENGINE_HEALTH_URL" >/dev/null 2>&1; then
  echo "[dev-local] engine did not become healthy in time" >&2
  exit 1
fi

"$ROOT_DIR/scripts/dev-frontend.sh" \
  > >(sed 's/^/[frontend] /') \
  2> >(sed 's/^/[frontend] /' >&2) &
FRONTEND_PID=$!

while kill -0 "$ENGINE_PID" 2>/dev/null && kill -0 "$FRONTEND_PID" 2>/dev/null; do
  sleep 1
done

if ! kill -0 "$ENGINE_PID" 2>/dev/null; then
  wait "$ENGINE_PID"
fi

if ! kill -0 "$FRONTEND_PID" 2>/dev/null; then
  wait "$FRONTEND_PID"
fi
