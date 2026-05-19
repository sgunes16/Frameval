#!/usr/bin/env bash
set -u

# dev-stop — kill anything left over from `make dev-full`. Useful
# when the foreground script was Ctrl-C'd in a different terminal,
# or when an Air rebuild stranded an engine, or when uv left a
# zombie python process bound to the grader port.
#
# We resolve PIDs by port (frontend:5173, engine:8080, grader:50051)
# via lsof, send SIGTERM, wait briefly, then SIGKILL anything still
# alive. Listing every PID we touch so the user sees what got
# stopped (or that nothing was running).

ports=(
  "frontend:5173"
  "engine:8080"
  "grader:50051"
)

found_any=0
to_kill=()

for entry in "${ports[@]}"; do
  label="${entry%%:*}"
  port="${entry##*:}"
  # -t prints PIDs only; -i selects internet sockets bound to the
  # given port. Filtering by LISTEN restricts to server processes
  # so we don't kill a transient curl client also touching the port.
  pids=$(lsof -t -iTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)
  if [[ -z "$pids" ]]; then
    echo "[dev-stop] $label (:$port) — not running"
    continue
  fi
  found_any=1
  for pid in $pids; do
    echo "[dev-stop] $label (:$port) — pid $pid"
    to_kill+=("$pid")
  done
done

if [[ "$found_any" -eq 0 ]]; then
  echo "[dev-stop] nothing to stop"
  exit 0
fi

echo "[dev-stop] sending SIGTERM"
for pid in "${to_kill[@]}"; do
  kill "$pid" 2>/dev/null || true
done

# Give them a moment to flush logs and close ports.
sleep 2

stragglers=()
for pid in "${to_kill[@]}"; do
  if kill -0 "$pid" 2>/dev/null; then
    stragglers+=("$pid")
  fi
done

if [[ "${#stragglers[@]}" -gt 0 ]]; then
  echo "[dev-stop] forcing SIGKILL on stragglers: ${stragglers[*]}"
  for pid in "${stragglers[@]}"; do
    kill -9 "$pid" 2>/dev/null || true
  done
fi

echo "[dev-stop] done"
