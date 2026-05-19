// Package executor wraps the public pkg/executor types with internal helpers.
//
// Public consumers should import engine/pkg/executor directly. This file
// exposes type aliases so existing internal code keeps compiling during the
// pkg refactor; the aliases resolve to the same underlying types as pkg/executor.
package executor

import (
	pkgexec "github.com/mustafaselman/frameval/engine/pkg/executor"
)

// Re-exported public types (identical to pkg/executor; aliases preserve
// existing call sites without churn).
type (
	AgentExecutor = pkgexec.AgentExecutor
	ExecutionMode = pkgexec.ExecutionMode
	RunConfig     = pkgexec.RunConfig
	RunResult     = pkgexec.RunResult
	ParsedTurn    = pkgexec.ParsedTurn
)

// Re-exported execution-mode constants.
const (
	ExecutionModeCLI = pkgexec.ExecutionModeCLI
	ExecutionModeAPI = pkgexec.ExecutionModeAPI
)

// Re-exported BlockKind constants for the structured parsed-turn payload.
const (
	BlockKindThinking   = pkgexec.BlockKindThinking
	BlockKindText       = pkgexec.BlockKindText
	BlockKindToolUse    = pkgexec.BlockKindToolUse
	BlockKindToolResult = pkgexec.BlockKindToolResult
	BlockKindSystem     = pkgexec.BlockKindSystem
)

// AssignTurnGrouping is the internal-package alias for the public helper
// in pkg/executor. Built-in parsers call it before returning their
// ParsedTurns slice so consumers get TurnIndex / ParentTurnIndex stamps
// without each parser re-implementing the logic.
var AssignTurnGrouping = pkgexec.AssignTurnGrouping

// defaultCLILanguageInstruction is an internal helper used by built-in CLI
// executors to nudge agents toward English output for reproducible grading.
const defaultCLILanguageInstruction = "Frameval evaluation instruction: respond in English unless the task explicitly asks for another language."

// promptWithDefaultCLILanguage prepends the language nudge to a raw prompt.
func promptWithDefaultCLILanguage(prompt string) string {
	return defaultCLILanguageInstruction + "\n\n" + prompt
}
