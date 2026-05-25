#!/usr/bin/env bash
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/workspace"
pytest -q --tb=short ../tests/test_search_correctness.py ../tests/test_throttle_under_load.py
bash "$SCRIPT_DIR/tests/test_scope.sh"
