package models

import "github.com/mustafaselman/frameval/engine/pkg/task"

// Task and TestCase are aliased to the public pkg/task types so the internal
// stack (orchestrator, storage, grader client) and external harness
// implementations share the same concrete type. Adding fields requires editing
// pkg/task/task.go, not this file.
type (
	Task     = task.Task
	TestCase = task.TestCase
)
