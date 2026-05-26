#!/usr/bin/env bash
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/workspace"
pytest -q --tb=short ../tests/test_user_creates.py ../tests/test_user_list_response.py ../tests/test_migrations_apply.py
bash "$SCRIPT_DIR/tests/test_scope.sh"
