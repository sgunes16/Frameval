package diagnostic

import (
	"math"
	"testing"

	"github.com/mustafaselman/frameval/engine/pkg/diagnostic"
	"github.com/mustafaselman/frameval/engine/pkg/executor"
)

func TestRecoveryEmptyTranscript(t *testing.T) {
	a := NewRecoveryAnalyzer()
	r := a.Analyze(nil)
	if len(r.ErrorEvents) != 0 {
		t.Errorf("expected no events, got %d", len(r.ErrorEvents))
	}
	if r.SilentSkipCount != 0 || r.ErrorAcknowledgmentRate != 0 {
		t.Errorf("expected zero profile, got %+v", r)
	}
}

func TestRecoveryClassifiesErrorKinds(t *testing.T) {
	a := NewRecoveryAnalyzer()
	turns := []executor.ParsedTurn{
		{Role: "tool", Content: "SyntaxError: invalid syntax at line 5"},          // compile
		{Role: "assistant", Content: "ok"},
		{Role: "tool", Content: "FAILED 3 tests failed"},                          // test failure
		{Role: "assistant", Content: "ok"},
		{Role: "tool", Content: "error: command not found"},                        // tool failure
		{Role: "assistant", Content: "ok"},
		{Role: "assistant", Content: "Traceback (most recent call last):"},        // stderr
	}
	r := a.Analyze(turns)
	if len(r.ErrorEvents) != 4 {
		t.Fatalf("expected 4 events, got %d", len(r.ErrorEvents))
	}
	want := []diagnostic.ErrorKind{
		diagnostic.ErrorKindCompileError,
		diagnostic.ErrorKindTestFailure,
		diagnostic.ErrorKindToolFailure,
		diagnostic.ErrorKindStderr,
	}
	for i, w := range want {
		if r.ErrorEvents[i].Type != w {
			t.Errorf("event %d kind = %q, want %q", i, r.ErrorEvents[i].Type, w)
		}
	}
}

func TestRecoveryAcknowledgmentRate(t *testing.T) {
	a := NewRecoveryAnalyzer()
	// 3 errors: 2 acknowledged in next assistant turn, 1 not.
	turns := []executor.ParsedTurn{
		{Role: "tool", Content: "error: foo"},                              // 0
		{Role: "assistant", Content: "Let me investigate"},                 // 1 — acks #0
		{Role: "tool", Content: "error: bar"},                              // 2
		{Role: "assistant", Content: "I'll fix that"},                      // 3 — acks #2
		{Role: "tool", Content: "error: baz"},                              // 4
		{Role: "tool", Content: "still broken"},                            // 5 — NOT assistant
	}
	r := a.Analyze(turns)
	// 2 / 3 ≈ 0.6667
	if math.Abs(r.ErrorAcknowledgmentRate-2.0/3.0) > 0.01 {
		t.Errorf("ErrorAcknowledgmentRate = %v, want ~0.667", r.ErrorAcknowledgmentRate)
	}
}

func TestRecoveryCorrectionLatencyAndSuccess(t *testing.T) {
	a := NewRecoveryAnalyzer()
	// Error at turn 0; correction (write_file) at turn 2. delta = 2.
	// No recurrence of the same kind in the next 3 turns → success.
	turns := []executor.ParsedTurn{
		{Role: "tool", Content: "error: dep missing"},               // 0
		{Role: "assistant", Content: "investigating"},                // 1
		{Role: "assistant", Content: "tool: write_file deps.txt"},   // 2 — correction
		{Role: "assistant", Content: "done"},                         // 3
	}
	r := a.Analyze(turns)
	if math.Abs(r.CorrectionLatencyMean-2.0) > 0.01 {
		t.Errorf("CorrectionLatencyMean = %v, want 2.0", r.CorrectionLatencyMean)
	}
	if r.CorrectionSuccessRate != 1.0 {
		t.Errorf("CorrectionSuccessRate = %v, want 1.0", r.CorrectionSuccessRate)
	}
}

func TestRecoveryCorrectionFailsWhenErrorRecurs(t *testing.T) {
	a := NewRecoveryAnalyzer()
	turns := []executor.ParsedTurn{
		{Role: "tool", Content: "error: dep missing"},                       // 0 (tool_failure)
		{Role: "assistant", Content: "tool: write_file"},                    // 1 — correction
		{Role: "tool", Content: "error: dep missing"},                       // 2 — recurrence
	}
	r := a.Analyze(turns)
	if r.CorrectionSuccessRate != 0 {
		t.Errorf("expected 0 success rate when error recurs; got %v", r.CorrectionSuccessRate)
	}
}

func TestRecoverySilentSkip(t *testing.T) {
	a := NewRecoveryAnalyzer()
	// Error at turn 0; subsequent turns make no reference and no correction.
	turns := []executor.ParsedTurn{
		{Role: "tool", Content: "error: thing went wrong"},
		{Role: "assistant", Content: "moving on to something unrelated"},
		{Role: "assistant", Content: "totally different"},
		{Role: "assistant", Content: "no correction here"},
	}
	r := a.Analyze(turns)
	if r.SilentSkipCount != 1 {
		t.Errorf("expected 1 silent skip, got %d", r.SilentSkipCount)
	}
	if r.ErrorAcknowledgmentRate != 0 {
		t.Errorf("expected 0 ack rate, got %v", r.ErrorAcknowledgmentRate)
	}
}

func TestRecoveryAckSkipsToolResultBetweenErrorAndAssistantReply(t *testing.T) {
	// A tool result frequently interposes between the error and the
	// assistant's reply. Acknowledgment scan must walk past it (bounded by
	// correctionWindow) so tool-heavy transcripts aren't unfairly counted
	// as silent skips.
	a := NewRecoveryAnalyzer()
	turns := []executor.ParsedTurn{
		{Role: "tool", Content: "error: missing dep"},
		{Role: "tool", Content: "additional context (still tool-role)"},
		{Role: "assistant", Content: "Let me fix that"},
	}
	r := a.Analyze(turns)
	if r.ErrorAcknowledgmentRate != 1.0 {
		t.Errorf("acknowledgment with interposed tool turn = %v, want 1.0", r.ErrorAcknowledgmentRate)
	}
}

func TestRecoveryEventAtLastTurnIsSilentSkip(t *testing.T) {
	// Spec says: an error at the very last turn has no follow-up window, so
	// it can't be acknowledged or corrected. Counts as silent skip.
	a := NewRecoveryAnalyzer()
	turns := []executor.ParsedTurn{
		{Role: "assistant", Content: "doing work"},
		{Role: "tool", Content: "error: at the end"},
	}
	r := a.Analyze(turns)
	if r.SilentSkipCount != 1 {
		t.Errorf("expected last-turn error to count as silent skip; got %d", r.SilentSkipCount)
	}
}

func TestRecoveryErrorEventMetadata(t *testing.T) {
	a := NewRecoveryAnalyzer()
	turns := []executor.ParsedTurn{
		{Role: "assistant", Content: "tool: write_file"},
		{Role: "tool", Content: "error: permission denied at /etc/foo"},
	}
	r := a.Analyze(turns)
	if len(r.ErrorEvents) != 1 {
		t.Fatalf("expected 1 event, got %d", len(r.ErrorEvents))
	}
	ev := r.ErrorEvents[0]
	if ev.TurnIndex != 1 {
		t.Errorf("TurnIndex = %d, want 1", ev.TurnIndex)
	}
	if ev.ToolName != "write_file" {
		t.Errorf("ToolName = %q, want write_file", ev.ToolName)
	}
	if len(ev.Message) > 200 {
		t.Errorf("Message length %d > 200 cap", len(ev.Message))
	}
}
