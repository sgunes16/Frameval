package diagnostic

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
)

func assistantTurn(content string) executor.ParsedTurn {
	return executor.ParsedTurn{Role: "assistant", Content: content}
}

func toolTurn(content string) executor.ParsedTurn {
	return executor.ParsedTurn{Role: "tool", Content: content}
}

func TestExtractEmptyTranscript(t *testing.T) {
	e := NewFingerprintExtractor()
	fp := e.Extract(nil, TaskContext{})
	if fp.PlanningDepth != 0 || fp.TurnEfficiency != 0 || fp.RecoveryLatency != 0 {
		t.Errorf("empty transcript should yield zero fingerprint, got %+v", fp)
	}
}

func TestPlanningDepthCountsIntentWithoutTool(t *testing.T) {
	e := NewFingerprintExtractor()
	turns := []executor.ParsedTurn{
		assistantTurn("My plan is to refactor the parser"),       // counts
		assistantTurn("First, I'll inspect main.py"),             // counts
		assistantTurn("Let me try write_file foo.py"),            // does NOT count (has tool call)
		toolTurn("file_write succeeded"),                         // does NOT count (tool role)
		assistantTurn("Next, run pytest"),                        // counts
	}
	fp := e.Extract(turns, TaskContext{})
	// 3 planning turns / 5 total = 0.6
	if math.Abs(fp.PlanningDepth-0.6) > 0.001 {
		t.Errorf("PlanningDepth = %v, want 0.6", fp.PlanningDepth)
	}
}

func TestToolCallDiversityShannonEntropy(t *testing.T) {
	e := NewFingerprintExtractor()
	turns := []executor.ParsedTurn{
		assistantTurn("tool: write_file"),
		assistantTurn("tool: write_file"),
		assistantTurn("tool: run_command"),
		assistantTurn("tool: run_command"),
	}
	fp := e.Extract(turns, TaskContext{})
	// 2 unique tools, equal frequencies → normalized entropy = 1.0
	if math.Abs(fp.ToolCallDiversity-1.0) > 0.01 {
		t.Errorf("ToolCallDiversity for balanced 2-tool dist = %v, want ~1.0", fp.ToolCallDiversity)
	}

	turns2 := []executor.ParsedTurn{
		assistantTurn("tool: write_file"),
		assistantTurn("tool: write_file"),
		assistantTurn("tool: write_file"),
	}
	fp2 := e.Extract(turns2, TaskContext{})
	if fp2.ToolCallDiversity != 0 {
		t.Errorf("ToolCallDiversity for single-tool monoculture = %v, want 0", fp2.ToolCallDiversity)
	}
}

func TestSelfValidationRate(t *testing.T) {
	e := NewFingerprintExtractor()
	turns := []executor.ParsedTurn{
		assistantTurn("write_file main.py"),       // producing
		assistantTurn("write_file utils.py"),      // producing
		assistantTurn("run pytest tests/"),        // validating (testRunRE matches "pytest")
		assistantTurn("read_file main.py"),        // validating (fileReadRE)
	}
	fp := e.Extract(turns, TaskContext{})
	// 2 validations / 2 producing = 1.0
	if math.Abs(fp.SelfValidationRate-1.0) > 0.01 {
		t.Errorf("SelfValidationRate = %v, want 1.0", fp.SelfValidationRate)
	}
}

func TestBacktrackRate(t *testing.T) {
	e := NewFingerprintExtractor()
	turns := []executor.ParsedTurn{
		assistantTurn("write_file foo.py"),
		assistantTurn("Actually, let me try a different approach"), // backtrack signal
		assistantTurn("write_file bar.py"),
	}
	fp := e.Extract(turns, TaskContext{})
	// 1 backtrack mention / 2 tool calls = 0.5
	if math.Abs(fp.BacktrackRate-0.5) > 0.01 {
		t.Errorf("BacktrackRate = %v, want 0.5", fp.BacktrackRate)
	}
}

func TestFileFocus(t *testing.T) {
	e := NewFingerprintExtractor()
	// 4 file ops, all touching main.py → very focused
	turns := []executor.ParsedTurn{
		assistantTurn("write_file main.py"),
		assistantTurn("read_file main.py"),
		assistantTurn("edit_file main.py"),
		assistantTurn("write_file main.py"),
	}
	fp := e.Extract(turns, TaskContext{})
	// unique=1, ops=4 → ratio=0.25 → focus = 1 - 0.25 = 0.75
	if math.Abs(fp.FileFocus-0.75) > 0.05 {
		t.Errorf("FileFocus = %v, want ~0.75", fp.FileFocus)
	}

	// Scattered: 4 ops, 4 unique files → ratio=1 → focus=0
	turnsScatter := []executor.ParsedTurn{
		assistantTurn("write_file main.py"),
		assistantTurn("write_file utils.py"),
		assistantTurn("write_file db.py"),
		assistantTurn("write_file api.py"),
	}
	fpScatter := e.Extract(turnsScatter, TaskContext{})
	if fpScatter.FileFocus > 0.1 {
		t.Errorf("scattered FileFocus = %v, want near 0", fpScatter.FileFocus)
	}
}

func TestRecoveryLatency(t *testing.T) {
	e := NewFingerprintExtractor()
	turns := []executor.ParsedTurn{
		assistantTurn("write_file foo.py"), // 0
		toolTurn("error: ImportError: no module named foo"), // 1 — error
		assistantTurn("Let me investigate"),                  // 2 — narrative, no state change
		assistantTurn("pip install foo"),                     // 3 — state-changing (shell_exec proxy via bash)
	}
	// Actually shell_exec/bash matches stateChangingRE; "pip install" by itself doesn't.
	// Use an explicit run_command marker so the test is unambiguous.
	turns[3] = assistantTurn("tool: run_command pip install foo")
	fp := e.Extract(turns, TaskContext{})
	// 1 error event at index 1; next state-changing at index 3; delta = 2
	if math.Abs(fp.RecoveryLatency-2.0) > 0.01 {
		t.Errorf("RecoveryLatency = %v, want 2.0", fp.RecoveryLatency)
	}
}

func TestPrematureCompletion(t *testing.T) {
	e := NewFingerprintExtractor()
	turns := []executor.ParsedTurn{
		assistantTurn("write_file main.py"),
		toolTurn("error: ImportError: missing dep"),
		assistantTurn("I'll handle that"),
		assistantTurn("I've completed the task"), // claims done with error in recent tail
	}
	fp := e.Extract(turns, TaskContext{})
	if fp.PrematureCompletion != 1 {
		t.Errorf("PrematureCompletion = %v, want 1", fp.PrematureCompletion)
	}

	// Clean completion (no error in last 5)
	clean := []executor.ParsedTurn{
		assistantTurn("write_file main.py"),
		assistantTurn("write_file utils.py"),
		assistantTurn("run pytest"),
		toolTurn("3 passed"),
		assistantTurn("I've completed the task"),
	}
	fpClean := e.Extract(clean, TaskContext{})
	if fpClean.PrematureCompletion != 0 {
		t.Errorf("clean PrematureCompletion = %v, want 0", fpClean.PrematureCompletion)
	}
}

func TestTurnEfficiency(t *testing.T) {
	e := NewFingerprintExtractor()
	turns := []executor.ParsedTurn{
		assistantTurn("write_file main.py"),
		assistantTurn("thinking..."),
		assistantTurn("edit_file main.py"),
		assistantTurn("more thinking"),
	}
	fp := e.Extract(turns, TaskContext{})
	// 2 state-changing / 4 total = 0.5
	if math.Abs(fp.TurnEfficiency-0.5) > 0.01 {
		t.Errorf("TurnEfficiency = %v, want 0.5", fp.TurnEfficiency)
	}
}

func TestContextReferenceRate(t *testing.T) {
	e := NewFingerprintExtractor()
	ctx := TaskContext{InstructionFiles: []string{"CLAUDE.md"}}
	turns := []executor.ParsedTurn{
		assistantTurn("Per CLAUDE.md I should run tests first"),
		assistantTurn("write_file main.py"),
		assistantTurn("As CLAUDE.md says, validate"),
		assistantTurn("done"),
	}
	fp := e.Extract(turns, ctx)
	// 2 / 4 = 0.5
	if math.Abs(fp.ContextReferenceRate-0.5) > 0.01 {
		t.Errorf("ContextReferenceRate = %v, want 0.5", fp.ContextReferenceRate)
	}

	// Without instruction files configured → returns 0
	fpNoCtx := e.Extract(turns, TaskContext{})
	if fpNoCtx.ContextReferenceRate != 0 {
		t.Errorf("no instruction files → got %v, want 0", fpNoCtx.ContextReferenceRate)
	}
}

func TestIdleThinkingRatio(t *testing.T) {
	e := NewFingerprintExtractor()
	turns := []executor.ParsedTurn{
		assistantTurn("thinking out loud, no action"),
		assistantTurn("write_file main.py"),
		assistantTurn("more thoughts"),
		toolTurn("ok"),
	}
	fp := e.Extract(turns, TaskContext{})
	// 2 idle assistant turns / 4 total = 0.5
	if math.Abs(fp.IdleThinkingRatio-0.5) > 0.01 {
		t.Errorf("IdleThinkingRatio = %v, want 0.5", fp.IdleThinkingRatio)
	}
}

func TestAllDimensionsBoundedAndNeverNaN(t *testing.T) {
	e := NewFingerprintExtractor()
	turns := []executor.ParsedTurn{
		assistantTurn("write_file main.py"),
		assistantTurn("tool: build"),
		toolTurn("error"),
		assistantTurn("I've completed the work"),
	}
	fp := e.Extract(turns, TaskContext{InstructionFiles: []string{"CLAUDE.md"}})

	check := func(name string, v float64, lo, hi float64) {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Errorf("%s = %v (NaN/Inf)", name, v)
			return
		}
		if v < lo || v > hi {
			t.Errorf("%s = %v out of [%v, %v]", name, v, lo, hi)
		}
	}
	check("PlanningDepth", fp.PlanningDepth, 0, 1)
	check("ToolCallDiversity", fp.ToolCallDiversity, 0, 1)
	check("SelfValidationRate", fp.SelfValidationRate, 0, 1)
	check("BacktrackRate", fp.BacktrackRate, 0, 1)
	check("FileFocus", fp.FileFocus, 0, 1)
	check("PrematureCompletion", fp.PrematureCompletion, 0, 1)
	check("TurnEfficiency", fp.TurnEfficiency, 0, 1)
	check("ContextReferenceRate", fp.ContextReferenceRate, 0, 1)
	check("IdleThinkingRatio", fp.IdleThinkingRatio, 0, 1)
	if fp.RecoveryLatency < 0 {
		t.Errorf("RecoveryLatency = %v, must be >= 0", fp.RecoveryLatency)
	}
}

func TestFingerprintJSONRoundTrip(t *testing.T) {
	e := NewFingerprintExtractor()
	turns := []executor.ParsedTurn{
		assistantTurn("First, I'll plan"),
		assistantTurn("write_file main.py"),
	}
	fp := e.Extract(turns, TaskContext{})
	bytes, err := json.Marshal(fp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(bytes) == 0 {
		t.Fatal("empty marshal output")
	}
	// All field names present
	for _, key := range []string{
		"planning_depth", "tool_call_diversity", "self_validation_rate",
		"backtrack_rate", "file_focus", "recovery_latency",
		"premature_completion", "turn_efficiency",
		"context_reference_rate", "idle_thinking_ratio",
	} {
		if !contains(string(bytes), `"`+key+`":`) {
			t.Errorf("JSON missing key %q in %s", key, string(bytes))
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(sub) > 0 && indexOf(s, sub) >= 0))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
