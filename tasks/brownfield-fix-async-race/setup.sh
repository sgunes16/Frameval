#!/usr/bin/env bash
# Sandbox-side setup for brownfield-fix-async-race.
#
# Installs deps, initializes the workspace as a git repo, and tags the
# baseline commit so test_scope.sh can diff against it later. The agent
# never sees this script; it runs before the agent's invocation.
#
# Path note: when run via Frameval's PrepareWorkspace the cwd is
# already the workspace root (/workspace inside the sandbox container)
# — the host layout's task_root/workspace/ subdir has been flattened
# so requirements.txt and app/ sit at the top level. setup.sh is
# loaded as a script string, not as a file on disk, so we can't
# compute SCRIPT_DIR; we just trust cwd.
set -euo pipefail

pip install --no-cache-dir -r requirements.txt

if [[ ! -d .git ]]; then
    git init -q
    git config user.email "agentdx@frameval.local"
    git config user.name "AgentDx"
    git add -A
    git commit -q -m "AgentDx baseline (brownfield-fix-async-race)"
    git tag baseline
fi
