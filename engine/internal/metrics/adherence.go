package metrics

import (
	"strings"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
)

// Check is a single named boolean test within an Adherence evaluation.
type Check struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
}

// Adherence summarises how faithfully an agent followed its harness's
// prescribed methodology, derived purely from the structured transcript.
type Adherence struct {
	// Score is the fraction of checks that passed (0..1).
	// Score is 1.0 when the harness defines no checks (bare, unknown).
	Score  float64 `json:"score"`
	Checks []Check `json:"checks"`
}

// scoreChecks computes Adherence from a completed check slice.
// If the slice is empty the score is 1.0 (no checks ≡ fully adherent).
func scoreChecks(checks []Check) Adherence {
	if len(checks) == 0 {
		return Adherence{Score: 1.0, Checks: checks}
	}
	var passed int
	for _, c := range checks {
		if c.Passed {
			passed++
		}
	}
	return Adherence{
		Score:  float64(passed) / float64(len(checks)),
		Checks: checks,
	}
}

// firstEditTurnIndex returns the minimum TurnIndex among tool turns that
// touch at least one file satisfying pathPredicate.
// ok is false if no such turn exists.
func firstEditTurnIndex(turns []executor.ParsedTurn, pathPredicate func(string) bool) (int, bool) {
	min := -1
	for _, t := range turns {
		if t.BlockKind != executor.BlockKindToolUse {
			continue
		}
		for _, f := range t.FilesTouched {
			if pathPredicate(f) {
				if min < 0 || t.TurnIndex < min {
					min = t.TurnIndex
				}
				break
			}
		}
	}
	if min < 0 {
		return 0, false
	}
	return min, true
}

// referencesClaudeMD returns true if a tool turn references CLAUDE.md either
// in its FilesTouched list or in its Content string.
func referencesClaudeMD(t executor.ParsedTurn) bool {
	for _, f := range t.FilesTouched {
		if strings.EqualFold(f, "CLAUDE.md") || strings.HasSuffix(f, "/CLAUDE.md") {
			return true
		}
	}
	return strings.Contains(t.Content, "CLAUDE.md")
}

// isAppSourceFile returns true when path is inside an app/ directory, which
// we treat as "implementation / source" code.
func isAppSourceFile(path string) bool {
	return strings.HasPrefix(path, "app/") || strings.Contains(path, "/app/")
}

// isSpecArtifact returns true when path looks like a spec output:
// anything under specs/ OR any filename ending in spec.md.
func isSpecArtifact(path string) bool {
	return strings.HasPrefix(path, "specs/") ||
		strings.Contains(path, "/specs/") ||
		strings.HasSuffix(path, "spec.md")
}

// isPlannerTurn returns true if the turn represents a planning role or stage.
// Recognised: Role in {"architect","planner"} OR Stage containing "plan".
func isPlannerTurn(t executor.ParsedTurn) bool {
	role := strings.ToLower(t.Role)
	if role == "architect" || role == "planner" {
		return true
	}
	return strings.Contains(strings.ToLower(t.Stage), "plan")
}

// isValidationTurn reuses the same patterns as Process() to identify
// test/lint invocations.
func isValidationTurn(t executor.ParsedTurn) bool {
	if t.BlockKind != executor.BlockKindToolUse {
		return false
	}
	lower := strings.ToLower(t.Content)
	for _, pat := range validationPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}

// HarnessAdherence scores how faithfully the agent followed the harness's
// prescribed process, from the structured turns alone.
//
// Recognised harnessID values: "bare", "agent_instructions", "multiagent",
// "ralph", "speckit". Any other value is treated like "bare" (Score=1.0).
func HarnessAdherence(harnessID string, turns []executor.ParsedTurn) Adherence {
	switch harnessID {
	case "bare":
		return scoreChecks(nil)

	case "agent_instructions":
		return checkAgentInstructions(turns)

	case "multiagent":
		return checkMultiagent(turns)

	case "ralph":
		return checkRalph(turns)

	case "speckit":
		return checkSpeckit(turns)

	default:
		// Unknown harness → no checks, fully adherent by definition.
		return scoreChecks(nil)
	}
}

// checkAgentInstructions evaluates the "agent_instructions" harness.
// Single check: at least one tool turn references CLAUDE.md.
func checkAgentInstructions(turns []executor.ParsedTurn) Adherence {
	found := false
	for _, t := range turns {
		if t.BlockKind != executor.BlockKindToolUse {
			continue
		}
		if referencesClaudeMD(t) {
			found = true
			break
		}
	}
	return scoreChecks([]Check{
		{Name: "instructions_present", Passed: found},
	})
}

// checkMultiagent evaluates the "multiagent" harness.
// Single check: a planning turn (architect/planner role or "plan" stage)
// occurs at a TurnIndex BEFORE the first turn that edits an app/ source file.
func checkMultiagent(turns []executor.ParsedTurn) Adherence {
	// Find the earliest app/ source edit.
	firstAppEdit, hasAppEdit := firstEditTurnIndex(turns, isAppSourceFile)

	// Find the earliest planner turn.
	firstPlannerIdx := -1
	for _, t := range turns {
		if isPlannerTurn(t) {
			if firstPlannerIdx < 0 || t.TurnIndex < firstPlannerIdx {
				firstPlannerIdx = t.TurnIndex
			}
		}
	}

	var passed bool
	if firstPlannerIdx >= 0 {
		if !hasAppEdit {
			// No app edit at all; planner ran ≡ pass.
			passed = true
		} else {
			passed = firstPlannerIdx < firstAppEdit
		}
	}

	return scoreChecks([]Check{
		{Name: "plan_before_code", Passed: passed},
	})
}

// checkRalph evaluates the "ralph" harness.
// Single check: the agent both edited a source file AND ran a validation
// command at a TurnIndex AFTER the first source edit. This is the hallmark
// of the ralph iteration loop.
func checkRalph(turns []executor.ParsedTurn) Adherence {
	// Find the first turn that touches any file (proxy for "source edit").
	firstEditIdx := -1
	for _, t := range turns {
		if t.BlockKind != executor.BlockKindToolUse {
			continue
		}
		if len(t.FilesTouched) > 0 {
			if firstEditIdx < 0 || t.TurnIndex < firstEditIdx {
				firstEditIdx = t.TurnIndex
			}
		}
	}

	passed := false
	if firstEditIdx >= 0 {
		// Look for a validation turn that comes AFTER the first source edit.
		for _, t := range turns {
			if isValidationTurn(t) && t.TurnIndex > firstEditIdx {
				passed = true
				break
			}
		}
	}

	return scoreChecks([]Check{
		{Name: "iterated", Passed: passed},
	})
}

// checkSpeckit evaluates the "speckit" harness.
// Two checks:
//  1. spec_before_impl: a spec artifact (specs/ or *spec.md) is written
//     BEFORE the first app/ source edit. If no source edit happened, passes.
//  2. stages_ran: at least 2 distinct non-empty Stage values appear in the
//     transcript (evidence the agent engaged ≥2 spec-kit stages).
func checkSpeckit(turns []executor.ParsedTurn) Adherence {
	// --- spec_before_impl ---
	firstAppEdit, hasAppEdit := firstEditTurnIndex(turns, isAppSourceFile)
	firstSpecWrite, hasSpecWrite := firstEditTurnIndex(turns, isSpecArtifact)

	var specBeforeImpl bool
	if !hasAppEdit {
		// No app/ edits at all → spec constraint is vacuously satisfied.
		specBeforeImpl = true
	} else if hasSpecWrite {
		// Both exist: spec must precede app edit.
		specBeforeImpl = firstSpecWrite < firstAppEdit
	} else {
		// App edit exists but no spec written → failed.
		specBeforeImpl = false
	}

	// --- stages_ran ---
	stageSet := make(map[string]struct{})
	for _, t := range turns {
		if t.Stage != "" {
			stageSet[t.Stage] = struct{}{}
		}
	}
	stagesRan := len(stageSet) >= 2

	return scoreChecks([]Check{
		{Name: "spec_before_impl", Passed: specBeforeImpl},
		{Name: "stages_ran", Passed: stagesRan},
	})
}
