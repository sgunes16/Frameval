// Package task defines the public Task spec consumed by harnesses and executors.
//
// A task on disk follows the standard layout:
//
//	tasks/<task-id>/
//	├── task.yaml           # populates Task via YAML unmarshal
//	├── workspace/          # initial files (empty for greenfield)
//	├── tests/              # hidden from agent, mounted read-only into sandbox
//	├── harness_context/    # optional per-harness context bundles (CLAUDE.md, constitution.md)
//	├── setup.sh            # installs deps in sandbox before agent invocation
//	└── eval.sh             # runs tests after agent invocation; exit code is the verdict
//
// Test-writing recipes and anti-patterns will be documented in
// docs/task-authoring.md (added in Story #26).
package task
