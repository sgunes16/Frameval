#!/usr/bin/env bash
# Eval runner. Invoked by the orchestrator after the agent finishes.
# tests/ is mounted alongside workspace/; we cd into workspace and run pytest
# pointing at the sibling tests dir. The agent's working directory was
# workspace/ — it never sees tests/.
#
# Exit code 0 = all tests passed; non-zero = at least one failure.
# We rely on pytest's `-q` reporter; orchestrator's code grader parses the
# stdout summary line for per-test results.
set -e
cd "$(dirname "$0")/workspace"
exec pytest -q --tb=short ../tests
