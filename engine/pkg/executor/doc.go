// Package executor defines the public AgentExecutor interface used by AgentDx
// harnesses to invoke agent CLIs or LLM APIs.
//
// AgentDx ships an "aider" and "cursor" executor (the latter is in MVP; aider
// lands in Story #9). Both live under engine/internal/builtin/executor.
//
// Third parties implement this interface to plug in their own agent backend
// (e.g., Claude Code, Codex, OpenHands) without forking the framework.
//
// # Example: minimal external executor
//
//	package myexec
//
//	import (
//	    "context"
//	    "github.com/mustafaselman/frameval/engine/pkg/executor"
//	)
//
//	type MyExec struct{}
//
//	func (e *MyExec) Name() string                       { return "myexec" }
//	func (e *MyExec) SupportedModes() []executor.ExecutionMode { return []executor.ExecutionMode{executor.ExecutionModeCLI} }
//
//	func (e *MyExec) Execute(ctx context.Context, cfg executor.RunConfig) (*executor.RunResult, error) {
//	    // ... invoke your agent, capture output ...
//	    return &executor.RunResult{RawOutput: "...", ParsedTurns: []executor.ParsedTurn{...}}, nil
//	}
//
//	func (e *MyExec) ParseTranscript(raw []byte) ([]executor.ParsedTurn, error) {
//	    // ... parse historical output ...
//	    return nil, nil
//	}
package executor
