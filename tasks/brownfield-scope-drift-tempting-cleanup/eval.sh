#!/usr/bin/env bash
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/workspace"
pytest -q --tb=short ../tests/test_charge_correctness.py ../tests/test_existing_behavior.py
bash "$SCRIPT_DIR/tests/test_scope.sh"
