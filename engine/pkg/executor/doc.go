// Package executor defines the public AgentExecutor interface used by AgentDx
// harnesses to invoke agent CLIs or LLM APIs.
//
// AgentDx ships two executors out of the box (under engine/internal/builtin/executor):
//   - "aider" — talks to a local Ollama server via Aider's OpenAI-compatible mode
//   - "cursor" — uses Cursor's CLI agent/auto mode against the Cursor cloud backend
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
