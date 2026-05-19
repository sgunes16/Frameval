#!/usr/bin/env bash
set -euo pipefail

# dev-full — single-command local dev environment:
#   1. Python grader sidecar (gRPC, localhost:50051)
#   2. Go engine (no Air; plain `go run`, so no rebuild churn)
#   3. Vite frontend dev server (localhost:5173)
#
# Logs from all three are interleaved with a [grader] / [engine] /
# [frontend] prefix. Ctrl-C tears everything down cleanly.
#
# Why no Air? When Air detects a change it kills+restarts the engine
# process. Every restart drops open WebSocket connections, every
# Inspector tab then re-tries /ws on its backoff, and the dev log
# fills with proxy ECONNRESET errors. For UI-only iteration you
# don't need Air — Ctrl-C and re-run when you touch Go code.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Track child PIDs for the cleanup trap.
GRADER_PID=""
ENGINE_PID=""
FRONTEND_PID=""

cleanup() {
  local status=0
  trap - EXIT INT TERM
  for pid_var in FRONTEND_PID ENGINE_PID GRADER_PID; do
    pid="${!pid_var:-}"
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      # SIGTERM first; the subprocesses propagate it to their own
      # children (uv, vite, etc.). Wait briefly, then force.
      kill "$pid" 2>/dev/null || true
    fi
  done
  # Brief grace period for the trees to exit.
  sleep 1
  for pid_var in FRONTEND_PID ENGINE_PID GRADER_PID; do
    pid="${!pid_var:-}"
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      kill -9 "$pid" 2>/dev/null || true
    fi
  done
  exit "$status"
}
trap cleanup EXIT INT TERM

echo "[dev-full] starting grader → engine → frontend"

# 1) Grader. Start first because the engine dials it on every run.
"$ROOT_DIR/scripts/dev-grader.sh" \
  > >(sed 's/^/[grader] /') \
  2> >(sed 's/^/[grader] /' >&2) &
GRADER_PID=$!

GRADER_PORT="${GRADER_PORT:-50051}"
echo "[dev-full] waiting for grader on 127.0.0.1:${GRADER_PORT}"
for _ in $(seq 1 60); do
  if ! kill -0 "$GRADER_PID" 2>/dev/null; then
    echo "[dev-full] grader exited before listening" >&2
    wait "$GRADER_PID" || true
    exit 1
  fi
  if nc -z 127.0.0.1 "$GRADER_PORT" 2>/dev/null; then
    break
  fi
  sleep 1
done
if ! nc -z 127.0.0.1 "$GRADER_PORT" 2>/dev/null; then
  echo "[dev-full] grader did not start listening within 60s — continuing anyway; engine will use fallback grades" >&2
fi

# 2) Engine (no Air).
"$ROOT_DIR/scripts/dev-engine-no-air.sh" \
  > >(sed 's/^/[engine] /') \
  2> >(sed 's/^/[engine] /' >&2) &
ENGINE_PID=$!

ENGINE_PORT="${FRAMEVAL_PORT:-8080}"
ENGINE_HEALTH_URL="http://127.0.0.1:${ENGINE_PORT}/api/health"
echo "[dev-full] waiting for engine at ${ENGINE_HEALTH_URL}"
for _ in $(seq 1 120); do
  if ! kill -0 "$ENGINE_PID" 2>/dev/null; then
    echo "[dev-full] engine exited before becoming healthy" >&2
    wait "$ENGINE_PID" || true
    exit 1
  fi
  if curl -fsS "$ENGINE_HEALTH_URL" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
if ! curl -fsS "$ENGINE_HEALTH_URL" >/dev/null 2>&1; then
  echo "[dev-full] engine did not become healthy in 120s" >&2
  exit 1
fi

# 3) Frontend.
"$ROOT_DIR/scripts/dev-frontend.sh" \
  > >(sed 's/^/[frontend] /') \
  2> >(sed 's/^/[frontend] /' >&2) &
FRONTEND_PID=$!

echo "[dev-full] all three running — open http://localhost:5173/"

# Wait until ANY child exits, then trigger cleanup. -n waits for
# the next process in the job table to finish; we don't care which.
wait -n "$GRADER_PID" "$ENGINE_PID" "$FRONTEND_PID"
echo "[dev-full] one of the children exited; tearing down the rest"
