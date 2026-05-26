#!/usr/bin/env bash
# Sandbox-side setup for brownfield-hal-api-pydantic-version.
# Same shape as brownfield-fix-async-race/setup.sh. Cwd is /workspace
# (flattened from tasks/brownfield-hal-api-pydantic-version/workspace/) inside the sandbox container.
set -euo pipefail
pip install --no-cache-dir -r requirements.txt
if [[ ! -d .git ]]; then
    git init -q
    git config user.email "agentdx@frameval.local"
    git config user.name "AgentDx"
    git add -A
    git commit -q -m "AgentDx baseline (brownfield-hal-api-pydantic-version)"
    git tag baseline
fi
