#!/usr/bin/env bash
# Eval runner for brownfield-fix-async-race.
#
# Runs both pytest tests (race + api contract) and the scope-discipline
# shell test. Exit 0 only when all three pass.
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

cd "$SCRIPT_DIR/workspace"
pytest -q --tb=short ../tests/test_race_fixed.py ../tests/test_api_unchanged.py

bash "$SCRIPT_DIR/tests/test_scope.sh"
