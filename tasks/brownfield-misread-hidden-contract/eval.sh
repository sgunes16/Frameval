#!/usr/bin/env bash
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/workspace"
pytest -q --tb=short ../tests/test_users_basic.py ../tests/test_spec_compliance.py
bash "$SCRIPT_DIR/tests/test_scope.sh"
