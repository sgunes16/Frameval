#!/usr/bin/env bash
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/workspace"
pytest -q --tb=short ../tests/test_validator_works.py ../tests/test_imports_v2_api.py
bash "$SCRIPT_DIR/tests/test_scope.sh"
