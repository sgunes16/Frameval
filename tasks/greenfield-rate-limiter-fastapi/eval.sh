#!/usr/bin/env bash
# Eval runner for greenfield-rate-limiter-fastapi.
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/workspace"
exec pytest -q --tb=short ../tests
