package metrics

import (
	"testing"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
)

// mkTurn builds a ParsedTurn with the given index and kind set to tool_use.
func mkToolTurn(index int, content string, files []string) executor.ParsedTurn {
	return executor.ParsedTurn{
		TurnIndex:    index,
		BlockKind:    executor.BlockKindToolUse,
		Content:      content,
		FilesTouched: files,
	}
}

// mkStagedToolTurn builds a tool_use turn with an explicit Stage.
func mkStagedToolTurn(index int, stage, content string, files []string) executor.ParsedTurn {
	t := mkToolTurn(index, content, files)
	t.Stage = stage
	return t
}

// mkRoleTurn builds a tool_use turn with an explicit Role.
func mkRoleTurn(index int, role, content string, files []string) executor.ParsedTurn {
	t := mkToolTurn(index, content, files)
	t.Role = role
	return t
}

// TestHarnessAdherence_Bare asserts that the "bare" harness produces Score=1.0
// with no checks regardless of transcript content.
func TestHarnessAdherence_Bare(t *testing.T) {
	turns := []executor.ParsedTurn{
		mkToolTurn(0, "edit app/main.py", []string{"app/main.py"}),
		mkToolTurn(1, "cat README", nil),
	}
	got := HarnessAdherence("bare", turns)

	if got.Score != 1.0 {
		t.Errorf("bare: expected Score=1.0, got %v", got.Score)
	}
	if len(got.Checks) != 0 {
		t.Errorf("bare: expected 0 checks, got %v", got.Checks)
	}
}

// TestHarnessAdherence_Bare_Empty asserts empty turns also get Score=1.0.
func TestHarnessAdherence_Bare_Empty(t *testing.T) {
	got := HarnessAdherence("bare", nil)
	if got.Score != 1.0 {
		t.Errorf("bare empty: expected Score=1.0, got %v", got.Score)
	}
}

// TestHarnessAdherence_Unknown asserts that unknown harness IDs get Score=1.0.
func TestHarnessAdherence_Unknown(t *testing.T) {
	got := HarnessAdherence("some-future-harness", nil)
	if got.Score != 1.0 {
		t.Errorf("unknown: expected Score=1.0, got %v", got.Score)
	}
	if len(got.Checks) != 0 {
		t.Errorf("unknown: expected 0 checks, got %d", len(got.Checks))
	}
}

// --- speckit ---

// TestHarnessAdherence_Speckit_ShortcutFails is the "deepseek-shortcut" case:
// the model edited an app/ source file before writing a spec artifact.
// spec_before_impl must FAIL, driving Score < 1.
func TestHarnessAdherence_Speckit_ShortcutFails(t *testing.T) {
	turns := []executor.ParsedTurn{
		// Turn 1: coder edits app source directly
		mkStagedToolTurn(1, "implement", "edit app/models.py", []string{"app/models.py"}),
		// Turn 5: then (too late) writes spec
		mkStagedToolTurn(5, "specify", "write spec", []string{"specs/master/spec.md"}),
	}
	got := HarnessAdherence("speckit", turns)

	if got.Score >= 1.0 {
		t.Errorf("speckit shortcut: expected Score<1.0, got %v", got.Score)
	}

	specCheck := findCheck(got.Checks, "spec_before_impl")
	if specCheck == nil {
		t.Fatal("speckit shortcut: check 'spec_before_impl' not found")
	}
	if specCheck.Passed {
		t.Error("speckit shortcut: spec_before_impl should FAIL when app edit precedes spec write")
	}
}

// TestHarnessAdherence_Speckit_CorrectOrder verifies the happy path:
// spec written first (turn 2), app edit later (turn 6), ≥2 distinct stages → Score=1.0.
func TestHarnessAdherence_Speckit_CorrectOrder(t *testing.T) {
	turns := []executor.ParsedTurn{
		mkStagedToolTurn(2, "specify", "write spec", []string{"specs/master/spec.md"}),
		mkStagedToolTurn(4, "plan", "create plan", nil),
		mkStagedToolTurn(6, "implement", "edit source", []string{"app/models.py"}),
	}
	got := HarnessAdherence("speckit", turns)

	if got.Score != 1.0 {
		t.Errorf("speckit correct order: expected Score=1.0, got %v", got.Score)
	}

	specCheck := findCheck(got.Checks, "spec_before_impl")
	if specCheck == nil {
		t.Fatal("spec_before_impl check missing")
	}
	if !specCheck.Passed {
		t.Error("speckit correct order: spec_before_impl should PASS")
	}

	stagesCheck := findCheck(got.Checks, "stages_ran")
	if stagesCheck == nil {
		t.Fatal("stages_ran check missing")
	}
	if !stagesCheck.Passed {
		t.Error("speckit correct order: stages_ran should PASS when ≥2 distinct stages ran")
	}
}

// TestHarnessAdherence_Speckit_OnlyOneStage: spec before impl but only one stage → stages_ran FAIL.
func TestHarnessAdherence_Speckit_OnlyOneStage(t *testing.T) {
	turns := []executor.ParsedTurn{
		mkStagedToolTurn(1, "specify", "write spec", []string{"specs/master/spec.md"}),
		mkStagedToolTurn(3, "specify", "edit source", []string{"app/models.py"}),
	}
	got := HarnessAdherence("speckit", turns)

	stagesCheck := findCheck(got.Checks, "stages_ran")
	if stagesCheck == nil {
		t.Fatal("stages_ran check missing")
	}
	if stagesCheck.Passed {
		t.Error("speckit one stage: stages_ran should FAIL when only 1 distinct stage")
	}
}

// TestHarnessAdherence_Speckit_NoSourceEdit: spec written but no app/ edit yet → spec_before_impl PASS.
func TestHarnessAdherence_Speckit_NoSourceEdit(t *testing.T) {
	turns := []executor.ParsedTurn{
		mkStagedToolTurn(1, "specify", "write spec", []string{"specs/master/spec.md"}),
		mkStagedToolTurn(2, "plan", "write plan", nil),
	}
	got := HarnessAdherence("speckit", turns)

	specCheck := findCheck(got.Checks, "spec_before_impl")
	if specCheck == nil {
		t.Fatal("spec_before_impl check missing")
	}
	if !specCheck.Passed {
		t.Error("speckit no source edit: spec_before_impl should PASS when no app/ edit exists at all")
	}
}

// --- multiagent ---

// TestHarnessAdherence_Multiagent_PlanBeforeCode: architect turn at 0, coder edit at 3 → passes.
func TestHarnessAdherence_Multiagent_PlanBeforeCode(t *testing.T) {
	turns := []executor.ParsedTurn{
		// architect role at index 0
		mkRoleTurn(0, "architect", "design the system", nil),
		// coder edits app source at index 3
		mkRoleTurn(3, "coder", "edit app/main.py", []string{"app/main.py"}),
	}
	got := HarnessAdherence("multiagent", turns)

	if got.Score != 1.0 {
		t.Errorf("multiagent plan first: expected Score=1.0, got %v", got.Score)
	}
	check := findCheck(got.Checks, "plan_before_code")
	if check == nil {
		t.Fatal("plan_before_code check missing")
	}
	if !check.Passed {
		t.Error("multiagent: plan_before_code should PASS when architect precedes coder edit")
	}
}

// TestHarnessAdherence_Multiagent_NoPlan: code edit with no architect/planner turn → FAIL.
func TestHarnessAdherence_Multiagent_NoPlan(t *testing.T) {
	turns := []executor.ParsedTurn{
		mkRoleTurn(0, "coder", "edit app/main.py", []string{"app/main.py"}),
		mkRoleTurn(2, "coder", "edit app/utils.py", []string{"app/utils.py"}),
	}
	got := HarnessAdherence("multiagent", turns)

	if got.Score >= 1.0 {
		t.Errorf("multiagent no plan: expected Score<1.0, got %v", got.Score)
	}
	check := findCheck(got.Checks, "plan_before_code")
	if check == nil {
		t.Fatal("plan_before_code check missing")
	}
	if check.Passed {
		t.Error("multiagent: plan_before_code should FAIL when no planner turn exists")
	}
}

// TestHarnessAdherence_Multiagent_PlanStageBeforeCode: a turn with Stage containing "plan" also satisfies the check.
func TestHarnessAdherence_Multiagent_PlanStageBeforeCode(t *testing.T) {
	turns := []executor.ParsedTurn{
		mkStagedToolTurn(0, "planning", "write plan doc", nil),
		mkToolTurn(5, "edit app/service.py", []string{"app/service.py"}),
	}
	got := HarnessAdherence("multiagent", turns)

	check := findCheck(got.Checks, "plan_before_code")
	if check == nil {
		t.Fatal("plan_before_code check missing")
	}
	if !check.Passed {
		t.Error("multiagent: plan_before_code should PASS when Stage contains 'plan'")
	}
}

// --- ralph ---

// TestHarnessAdherence_Ralph_Iterated: edit then pytest → iterated passes.
func TestHarnessAdherence_Ralph_Iterated(t *testing.T) {
	turns := []executor.ParsedTurn{
		mkToolTurn(2, "edit app/main.py", []string{"app/main.py"}),
		mkToolTurn(5, "pytest tests/", nil),
	}
	got := HarnessAdherence("ralph", turns)

	if got.Score != 1.0 {
		t.Errorf("ralph iterated: expected Score=1.0, got %v", got.Score)
	}
	check := findCheck(got.Checks, "iterated")
	if check == nil {
		t.Fatal("iterated check missing")
	}
	if !check.Passed {
		t.Error("ralph: iterated should PASS when validation follows source edit")
	}
}

// TestHarnessAdherence_Ralph_NoValidation: source edit but no test run → iterated FAIL.
func TestHarnessAdherence_Ralph_NoValidation(t *testing.T) {
	turns := []executor.ParsedTurn{
		mkToolTurn(2, "edit app/main.py", []string{"app/main.py"}),
		mkToolTurn(3, "cat app/main.py", nil),
	}
	got := HarnessAdherence("ralph", turns)

	check := findCheck(got.Checks, "iterated")
	if check == nil {
		t.Fatal("iterated check missing")
	}
	if check.Passed {
		t.Error("ralph: iterated should FAIL when no validation run follows edit")
	}
}

// TestHarnessAdherence_Ralph_ValidationBeforeEdit: validation before edit doesn't count.
func TestHarnessAdherence_Ralph_ValidationBeforeEdit(t *testing.T) {
	turns := []executor.ParsedTurn{
		mkToolTurn(0, "pytest tests/", nil), // validation first
		mkToolTurn(3, "edit app/main.py", []string{"app/main.py"}),
		// no validation after the edit
	}
	got := HarnessAdherence("ralph", turns)

	check := findCheck(got.Checks, "iterated")
	if check == nil {
		t.Fatal("iterated check missing")
	}
	if check.Passed {
		t.Error("ralph: iterated should FAIL when validation only occurs BEFORE the source edit")
	}
}

// TestHarnessAdherence_Ralph_IterationStages: ≥2 distinct "iteration-N" stages
// is the primary signal — it passes even when the agent verified with an inline
// command not in validationPatterns (the real-world ralph case that previously
// false-failed).
func TestHarnessAdherence_Ralph_IterationStages(t *testing.T) {
	turns := []executor.ParsedTurn{
		mkStagedToolTurn(0, "iteration-0", "edit app/main.py", []string{"app/main.py"}),
		mkStagedToolTurn(1, "iteration-0", "python3 -c 'import app.main'", nil),
		mkStagedToolTurn(2, "iteration-1", "edit app/main.py again", []string{"app/main.py"}),
	}
	got := HarnessAdherence("ralph", turns)

	if got.Score != 1.0 {
		t.Errorf("ralph iteration stages: expected Score=1.0, got %v", got.Score)
	}
	check := findCheck(got.Checks, "iterated")
	if check == nil {
		t.Fatal("iterated check missing")
	}
	if !check.Passed {
		t.Error("ralph: iterated should PASS with ≥2 distinct iteration-N stages")
	}
}

// TestHarnessAdherence_Ralph_SingleIterationNoValidation: one iteration stage and
// no validation run → iterated FAIL (neither signal satisfied).
func TestHarnessAdherence_Ralph_SingleIterationNoValidation(t *testing.T) {
	turns := []executor.ParsedTurn{
		mkStagedToolTurn(0, "iteration-0", "edit app/main.py", []string{"app/main.py"}),
		mkStagedToolTurn(1, "iteration-0", "cat app/main.py", nil),
	}
	got := HarnessAdherence("ralph", turns)

	check := findCheck(got.Checks, "iterated")
	if check == nil {
		t.Fatal("iterated check missing")
	}
	if check.Passed {
		t.Error("ralph: iterated should FAIL with a single iteration and no validation")
	}
}

// TestHarnessAdherence_Ralph_GoTest: go test also counts as validation.
func TestHarnessAdherence_Ralph_GoTest(t *testing.T) {
	turns := []executor.ParsedTurn{
		mkToolTurn(1, "edit app/handler.go", []string{"app/handler.go"}),
		mkToolTurn(4, "go test ./...", nil),
	}
	got := HarnessAdherence("ralph", turns)

	check := findCheck(got.Checks, "iterated")
	if check == nil {
		t.Fatal("iterated check missing")
	}
	if !check.Passed {
		t.Error("ralph: iterated should PASS with 'go test'")
	}
}

// --- agent_instructions ---

// TestHarnessAdherence_AgentInstructions_NoDeterministicCheck: agent_instructions
// has no transcript-derived process check (opencode auto-loads CLAUDE.md into
// context, never reading it via a tool), so it always scores 1.0 with 0 checks —
// regardless of whether CLAUDE.md is "referenced" in the turns.
func TestHarnessAdherence_AgentInstructions_NoDeterministicCheck(t *testing.T) {
	cases := [][]executor.ParsedTurn{
		// No explicit CLAUDE.md reference (the realistic opencode case).
		{mkToolTurn(0, "edit app/main.py", []string{"app/main.py"})},
		// Even an explicit reference yields the same result.
		{mkToolTurn(0, "read CLAUDE.md", []string{"CLAUDE.md"})},
		nil,
	}
	for i, turns := range cases {
		got := HarnessAdherence("agent_instructions", turns)
		if got.Score != 1.0 {
			t.Errorf("case %d: expected Score=1.0, got %v", i, got.Score)
		}
		if len(got.Checks) != 0 {
			t.Errorf("case %d: expected 0 checks, got %v", i, got.Checks)
		}
	}
}

// findCheck returns the named Check from the slice, or nil.
func findCheck(checks []Check, name string) *Check {
	for i := range checks {
		if checks[i].Name == name {
			return &checks[i]
		}
	}
	return nil
}
