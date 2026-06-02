package metrics

import (
	"strings"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
)

// ProcessMetrics holds deterministic process-level metrics computed
// from a run's structured transcript. All fields are derived from
// ParsedTurn data only — no string-grepping of raw output required.
type ProcessMetrics struct {
	// TurnCount is the number of meaningful turns: assistant text/thinking
	// turns with non-empty content, plus all tool-use turns (even those
	// with empty content, since a tool invocation is an event regardless).
	// Empty non-tool turns and system turns are excluded.
	TurnCount int

	// ToolCallCount is the number of tool-use turns (BlockKind == "tool_use").
	ToolCallCount int

	// ToolErrorRate is the fraction of tool calls that resulted in an error
	// (0..1). A tool turn is considered errored if Stage == "error" OR if
	// its ToolOutput contains an "<error>" tag.
	ToolErrorRate float64

	// RanValidation is true if at least one tool turn invoked a known
	// test/lint command (pytest, go test, ruff, mypy, npm test,
	// npm run test, bash tests/).
	RanValidation bool

	// ValidationCount is the total number of validation tool turns.
	ValidationCount int

	// BacktrackCount is the number of distinct file paths that appear in
	// two or more separate edit/write tool turns. A file edited ≥ 2 times
	// signals rework.
	BacktrackCount int
}

// validationPatterns are the case-insensitive substrings that mark a
// tool turn as a validation/test invocation. The command text lives in
// the turn's Content field (the bash input the agent passed to the tool).
var validationPatterns = []string{
	"pytest",
	"go test",
	"ruff",
	"mypy",
	"npm test",
	"npm run test",
	"bash tests/",
}

// isToolTurn reports whether a turn represents a tool invocation.
// We rely on BlockKind == BlockKindToolUse ("tool_use") as the
// canonical indicator; this is stamped by all executors that produce
// structured transcripts.
func isToolTurn(t executor.ParsedTurn) bool {
	return t.BlockKind == executor.BlockKindToolUse
}

// isToolError reports whether a tool turn resulted in an error.
// Two signals are checked:
//  1. Stage == "error" (set by the executor, e.g. opencode's "invalid" tool)
//  2. ToolOutput contains "<error>" (non-zero exit / tool-level error text)
func isToolError(t executor.ParsedTurn) bool {
	if t.Stage == "error" {
		return true
	}
	if strings.Contains(t.ToolOutput, "<error>") {
		return true
	}
	return false
}

// isValidation reports whether a tool turn invoked a known test/lint
// command. The command is carried in Content (the input the agent
// supplied to the Bash tool). Matching is case-insensitive substring.
func isValidation(t executor.ParsedTurn) bool {
	lower := strings.ToLower(t.Content)
	for _, pat := range validationPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}

// isMeaningfulTurn reports whether a turn should contribute to TurnCount.
// Rules:
//   - Tool-use turns always count (they represent agent actions).
//   - Non-tool turns count only when they have non-empty content AND are
//     not system-role / BlockKindSystem turns.
func isMeaningfulTurn(t executor.ParsedTurn) bool {
	if isToolTurn(t) {
		return true
	}
	if t.Role == "system" || t.BlockKind == executor.BlockKindSystem {
		return false
	}
	return strings.TrimSpace(t.Content) != ""
}

// Process computes ProcessMetrics from a structured transcript.
func Process(turns []executor.ParsedTurn) ProcessMetrics {
	if len(turns) == 0 {
		return ProcessMetrics{}
	}

	// fileEditCount tracks how many distinct tool turns touched each path.
	fileEditCount := make(map[string]int)

	var m ProcessMetrics
	var toolErrors int

	for _, t := range turns {
		if !isMeaningfulTurn(t) {
			continue
		}
		m.TurnCount++

		if !isToolTurn(t) {
			continue
		}

		m.ToolCallCount++

		if isToolError(t) {
			toolErrors++
		}

		if isValidation(t) {
			m.ValidationCount++
		}

		// Record files touched by this edit/write turn. We count per-file
		// across turns, not per-file within a single turn, so that a single
		// turn touching "a.py" twice still counts as one touch for "a.py".
		seen := make(map[string]bool)
		for _, path := range t.FilesTouched {
			if path == "" || seen[path] {
				continue
			}
			seen[path] = true
			fileEditCount[path]++
		}
	}

	if m.ToolCallCount > 0 {
		m.ToolErrorRate = float64(toolErrors) / float64(m.ToolCallCount)
	}

	m.RanValidation = m.ValidationCount > 0

	for _, count := range fileEditCount {
		if count >= 2 {
			m.BacktrackCount++
		}
	}

	return m
}
