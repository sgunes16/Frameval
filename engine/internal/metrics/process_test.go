package metrics

import (
	"testing"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
)

func TestProcess(t *testing.T) {
	tests := []struct {
		name  string
		turns []executor.ParsedTurn
		want  ProcessMetrics
	}{
		{
			name: "three tool turns one errored",
			turns: []executor.ParsedTurn{
				{Role: "assistant", Content: "read file", BlockKind: executor.BlockKindToolUse, ToolName: "Read"},
				{Role: "assistant", Content: "write file", BlockKind: executor.BlockKindToolUse, ToolName: "Write"},
				{Role: "assistant", Content: "bash failed", BlockKind: executor.BlockKindToolUse, ToolName: "Bash", Stage: "error"},
			},
			want: ProcessMetrics{
				TurnCount:       3,
				ToolCallCount:   3,
				ToolErrorRate:   1.0 / 3.0,
				RanValidation:   false,
				ValidationCount: 0,
				BacktrackCount:  0,
			},
		},
		{
			name: "tool error detected via error tag in ToolOutput",
			turns: []executor.ParsedTurn{
				{Role: "assistant", Content: "run something", BlockKind: executor.BlockKindToolUse, ToolName: "Bash", ToolOutput: "<error>command not found</error>"},
				{Role: "assistant", Content: "another call", BlockKind: executor.BlockKindToolUse, ToolName: "Read"},
			},
			want: ProcessMetrics{
				TurnCount:       2,
				ToolCallCount:   2,
				ToolErrorRate:   0.5,
				RanValidation:   false,
				ValidationCount: 0,
				BacktrackCount:  0,
			},
		},
		{
			name: "bash tool turn with pytest command",
			turns: []executor.ParsedTurn{
				{Role: "assistant", Content: "Let me run tests", BlockKind: executor.BlockKindText},
				{Role: "assistant", Content: "pytest -q", BlockKind: executor.BlockKindToolUse, ToolName: "Bash"},
			},
			want: ProcessMetrics{
				TurnCount:       2,
				ToolCallCount:   1,
				ToolErrorRate:   0.0,
				RanValidation:   true,
				ValidationCount: 1,
				BacktrackCount:  0,
			},
		},
		{
			name: "go test command",
			turns: []executor.ParsedTurn{
				{Role: "assistant", Content: "go test ./...", BlockKind: executor.BlockKindToolUse, ToolName: "Bash"},
			},
			want: ProcessMetrics{
				TurnCount:       1,
				ToolCallCount:   1,
				ToolErrorRate:   0.0,
				RanValidation:   true,
				ValidationCount: 1,
				BacktrackCount:  0,
			},
		},
		{
			name: "npm test command",
			turns: []executor.ParsedTurn{
				{Role: "assistant", Content: "npm test", BlockKind: executor.BlockKindToolUse, ToolName: "Bash"},
			},
			want: ProcessMetrics{
				TurnCount:       1,
				ToolCallCount:   1,
				ToolErrorRate:   0.0,
				RanValidation:   true,
				ValidationCount: 1,
				BacktrackCount:  0,
			},
		},
		{
			name: "npm run test command",
			turns: []executor.ParsedTurn{
				{Role: "assistant", Content: "npm run test", BlockKind: executor.BlockKindToolUse, ToolName: "Bash"},
			},
			want: ProcessMetrics{
				TurnCount:       1,
				ToolCallCount:   1,
				ToolErrorRate:   0.0,
				RanValidation:   true,
				ValidationCount: 1,
				BacktrackCount:  0,
			},
		},
		{
			name: "ruff and mypy commands",
			turns: []executor.ParsedTurn{
				{Role: "assistant", Content: "ruff check .", BlockKind: executor.BlockKindToolUse, ToolName: "Bash"},
				{Role: "assistant", Content: "mypy src/", BlockKind: executor.BlockKindToolUse, ToolName: "Bash"},
			},
			want: ProcessMetrics{
				TurnCount:       2,
				ToolCallCount:   2,
				ToolErrorRate:   0.0,
				RanValidation:   true,
				ValidationCount: 2,
				BacktrackCount:  0,
			},
		},
		{
			name: "bash tests/ command",
			turns: []executor.ParsedTurn{
				{Role: "assistant", Content: "bash tests/run.sh", BlockKind: executor.BlockKindToolUse, ToolName: "Bash"},
			},
			want: ProcessMetrics{
				TurnCount:       1,
				ToolCallCount:   1,
				ToolErrorRate:   0.0,
				RanValidation:   true,
				ValidationCount: 1,
				BacktrackCount:  0,
			},
		},
		{
			name: "two edit turns touching same file",
			turns: []executor.ParsedTurn{
				{Role: "assistant", Content: "first edit", BlockKind: executor.BlockKindToolUse, ToolName: "Edit", FilesTouched: []string{"app/models.py"}},
				{Role: "assistant", Content: "some text", BlockKind: executor.BlockKindText},
				{Role: "assistant", Content: "second edit", BlockKind: executor.BlockKindToolUse, ToolName: "Edit", FilesTouched: []string{"app/models.py"}},
			},
			want: ProcessMetrics{
				TurnCount:       3,
				ToolCallCount:   2,
				ToolErrorRate:   0.0,
				RanValidation:   false,
				ValidationCount: 0,
				BacktrackCount:  1,
			},
		},
		{
			name: "multiple files some edited twice",
			turns: []executor.ParsedTurn{
				{Role: "assistant", Content: "edit a", BlockKind: executor.BlockKindToolUse, ToolName: "Edit", FilesTouched: []string{"a.py", "b.py"}},
				{Role: "assistant", Content: "edit b again", BlockKind: executor.BlockKindToolUse, ToolName: "Edit", FilesTouched: []string{"b.py"}},
				{Role: "assistant", Content: "edit c", BlockKind: executor.BlockKindToolUse, ToolName: "Write", FilesTouched: []string{"c.py"}},
				{Role: "assistant", Content: "edit a again", BlockKind: executor.BlockKindToolUse, ToolName: "Edit", FilesTouched: []string{"a.py"}},
			},
			want: ProcessMetrics{
				TurnCount:       4,
				ToolCallCount:   4,
				ToolErrorRate:   0.0,
				RanValidation:   false,
				ValidationCount: 0,
				BacktrackCount:  2, // a.py and b.py each edited >= 2 times
			},
		},
		{
			name: "empty content non-tool turn is skipped in TurnCount",
			turns: []executor.ParsedTurn{
				{Role: "assistant", Content: "", BlockKind: executor.BlockKindText},                                   // empty, skip
				{Role: "system", Content: "system init", BlockKind: executor.BlockKindSystem},                        // system, skip
				{Role: "assistant", Content: "real thought", BlockKind: executor.BlockKindText},                      // counted
				{Role: "assistant", Content: "tool call", BlockKind: executor.BlockKindToolUse, ToolName: "Read"},    // counted
				{Role: "assistant", Content: "", BlockKind: executor.BlockKindToolUse, ToolName: "EmptyToolStill"},   // tool with no content: still counted (it's a tool turn)
			},
			want: ProcessMetrics{
				TurnCount:       3,
				ToolCallCount:   2,
				ToolErrorRate:   0.0,
				RanValidation:   false,
				ValidationCount: 0,
				BacktrackCount:  0,
			},
		},
		{
			name: "empty turns slice",
			turns: []executor.ParsedTurn{},
			want: ProcessMetrics{
				TurnCount:       0,
				ToolCallCount:   0,
				ToolErrorRate:   0.0,
				RanValidation:   false,
				ValidationCount: 0,
				BacktrackCount:  0,
			},
		},
		{
			name: "nil turns",
			turns: nil,
			want: ProcessMetrics{
				TurnCount:       0,
				ToolCallCount:   0,
				ToolErrorRate:   0.0,
				RanValidation:   false,
				ValidationCount: 0,
				BacktrackCount:  0,
			},
		},
		{
			name: "mixed: validation + backtrack + error",
			turns: []executor.ParsedTurn{
				{Role: "assistant", Content: "first edit models", BlockKind: executor.BlockKindToolUse, ToolName: "Edit", FilesTouched: []string{"app/models.py"}},
				{Role: "assistant", Content: "pytest -v", BlockKind: executor.BlockKindToolUse, ToolName: "Bash"},
				{Role: "assistant", Content: "fix models again", BlockKind: executor.BlockKindToolUse, ToolName: "Edit", FilesTouched: []string{"app/models.py"}, Stage: "error"},
				{Role: "assistant", Content: "Looks good", BlockKind: executor.BlockKindText},
			},
			want: ProcessMetrics{
				TurnCount:       4,
				ToolCallCount:   3,
				ToolErrorRate:   1.0 / 3.0,
				RanValidation:   true,
				ValidationCount: 1,
				BacktrackCount:  1,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Process(tc.turns)

			if got.TurnCount != tc.want.TurnCount {
				t.Errorf("TurnCount: got %d, want %d", got.TurnCount, tc.want.TurnCount)
			}
			if got.ToolCallCount != tc.want.ToolCallCount {
				t.Errorf("ToolCallCount: got %d, want %d", got.ToolCallCount, tc.want.ToolCallCount)
			}
			// Use approximate comparison for float
			const epsilon = 1e-9
			diff := got.ToolErrorRate - tc.want.ToolErrorRate
			if diff < -epsilon || diff > epsilon {
				t.Errorf("ToolErrorRate: got %f, want %f", got.ToolErrorRate, tc.want.ToolErrorRate)
			}
			if got.RanValidation != tc.want.RanValidation {
				t.Errorf("RanValidation: got %v, want %v", got.RanValidation, tc.want.RanValidation)
			}
			if got.ValidationCount != tc.want.ValidationCount {
				t.Errorf("ValidationCount: got %d, want %d", got.ValidationCount, tc.want.ValidationCount)
			}
			if got.BacktrackCount != tc.want.BacktrackCount {
				t.Errorf("BacktrackCount: got %d, want %d", got.BacktrackCount, tc.want.BacktrackCount)
			}
		})
	}
}
