#!/usr/bin/env bash
# Sandbox-side setup. Runs inside the sandbox container before the agent starts.
# Installs the allowed dependency (click) and the test-side dependency (pytest).
set -euo pipefail
pip install --no-cache-dir click pytest
