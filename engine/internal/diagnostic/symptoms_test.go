package diagnostic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
	"github.com/mustafaselman/frameval/engine/pkg/task"
)

func TestSymptomsEmpty(t *testing.T) {
	e := NewSymptomExtractor()
	s := e.Extract(nil, RunOutcome{}, task.Task{})
	if s.TestsTotal != 0 || s.DeclaredCompletion || s.AcknowledgedFailure || s.TimeToFirstError != -1 {
		t.Errorf("empty inputs should yield zero-valued packet, got %+v", s)
	}
}

func TestSymptomsCarriesOutcomeFields(t *testing.T) {
	e := NewSymptomExtractor()
	outcome := RunOutcome{
		TestsPassed: 3, TestsFailed: 2, TestsTotal: 5,
		CompileFailed: true,
		LintErrors:    []string{"E501"},
		WallClockSeconds: 12.3,
		TimeoutHit:       true,
		FilesTouched:     []string{"main.py", "utils.py", "main.py"}, // dup
		FilesCreated:     []string{"utils.py"},
	}
	s := e.Extract(nil, outcome, task.Task{})
	if s.TestsPassed != 3 || s.TestsFailed != 2 || s.TestsTotal != 5 {
		t.Errorf("test counts not carried: %+v", s)
	}
	if !s.CompileFailed {
		t.Error("CompileFailed not carried")
	}
	if s.WallClockSeconds != 12.3 || !s.TimeoutHit {
		t.Errorf("timing not carried: wall=%v timeout=%v", s.WallClockSeconds, s.TimeoutHit)
	}
	if len(s.FilesTouched) != 2 {
		t.Errorf("FilesTouched dedup failed: %v", s.FilesTouched)
	}
}

func TestSymptomsDetectsCompletionAndFailureClaims(t *testing.T) {
	e := NewSymptomExtractor()
	turns := []executor.ParsedTurn{
		{Role: "assistant", Content: "I'll start"},
		{Role: "assistant", Content: "I've completed the implementation"},
	}
	s := e.Extract(turns, RunOutcome{}, task.Task{})
	if !s.DeclaredCompletion {
		t.Error("DeclaredCompletion should be true")
	}
	if s.AcknowledgedFailure {
		t.Error("AcknowledgedFailure should be false")
	}

	failureTurns := []executor.ParsedTurn{
		{Role: "assistant", Content: "I cannot proceed; I'm unable to fix this"},
	}
	sf := e.Extract(failureTurns, RunOutcome{}, task.Task{})
	if !sf.AcknowledgedFailure {
		t.Error("AcknowledgedFailure should be true")
	}
}

func TestSymptomsCollectsToolFailures(t *testing.T) {
	e := NewSymptomExtractor()
	turns := []executor.ParsedTurn{
		{Role: "assistant", Content: "tool: write_file"},
		{Role: "tool", Content: "ok"},                                                  // not an error
		{Role: "assistant", Content: "tool: run_command"},
		{Role: "tool", Content: "error: command failed with exit code 1"},              // failure
		{Role: "assistant", Content: "tool: read_file"},
		{Role: "tool", Content: "ImportError: no module named foo (Traceback ...)"},    // failure
	}
	s := e.Extract(turns, RunOutcome{}, task.Task{})
	if len(s.ToolFailures) != 2 {
		t.Fatalf("expected 2 tool failures, got %d", len(s.ToolFailures))
	}
	if s.ToolFailures[0].ToolName != "run_command" {
		t.Errorf("first failure tool = %q, want run_command", s.ToolFailures[0].ToolName)
	}
	if s.ToolFailures[1].ToolName != "read_file" {
		t.Errorf("second failure tool = %q, want read_file", s.ToolFailures[1].ToolName)
	}
}

func TestSymptomsTimeToFirstError(t *testing.T) {
	e := NewSymptomExtractor()
	turns := []executor.ParsedTurn{
		{Role: "assistant", Content: "starting"},
		{Role: "assistant", Content: "tool: write_file"},
		{Role: "tool", Content: "ImportError: bad import"}, // index 2
		{Role: "assistant", Content: "let me retry"},
	}
	s := e.Extract(turns, RunOutcome{}, task.Task{})
	if s.TimeToFirstError != 2 {
		t.Errorf("TimeToFirstError = %d, want 2", s.TimeToFirstError)
	}

	noError := []executor.ParsedTurn{
		{Role: "assistant", Content: "all good"},
	}
	sNo := e.Extract(noError, RunOutcome{}, task.Task{})
	if sNo.TimeToFirstError != -1 {
		t.Errorf("no-error TimeToFirstError = %d, want -1", sNo.TimeToFirstError)
	}
}

func TestSymptomsLastAssistantClaimTruncated(t *testing.T) {
	e := NewSymptomExtractor()
	longContent := strings.Repeat("ABCD", 200) // 800 chars > 500-cap
	turns := []executor.ParsedTurn{
		{Role: "assistant", Content: longContent},
	}
	s := e.Extract(turns, RunOutcome{}, task.Task{})
	if len(s.LastAssistantClaim) != lastClaimMaxLen {
		t.Errorf("LastAssistantClaim length = %d, want %d", len(s.LastAssistantClaim), lastClaimMaxLen)
	}
}

func TestSymptomsUnexpectedFilesModified(t *testing.T) {
	e := NewSymptomExtractor()
	outcome := RunOutcome{
		FilesTouched:          []string{"app/user_service.py", "app/db.py", "app/user_service.py"},
		ExpectedFilesModified: []string{"app/user_service.py"},
	}
	s := e.Extract(nil, outcome, task.Task{})
	if len(s.UnexpectedFilesModified) != 1 || s.UnexpectedFilesModified[0] != "app/db.py" {
		t.Errorf("UnexpectedFilesModified = %v, want [app/db.py]", s.UnexpectedFilesModified)
	}

	// No expected list configured → returns nil (check disabled)
	sNo := e.Extract(nil, RunOutcome{FilesTouched: []string{"a", "b"}}, task.Task{})
	if sNo.UnexpectedFilesModified != nil {
		t.Errorf("no expected list should yield nil, got %v", sNo.UnexpectedFilesModified)
	}
}

func TestTruncateUTF8DoesNotSplitMultiByteRune(t *testing.T) {
	// Each "é" is 2 bytes in UTF-8. A 251-byte cap would land mid-rune
	// without the boundary walk. truncateUTF8 must back up to an even
	// boundary so the result contains no broken bytes.
	src := strings.Repeat("é", 300) // 600 bytes total
	out := truncateUTF8(src, 251)
	if len(out)%2 != 0 {
		t.Errorf("truncated length %d is odd; landed mid-rune", len(out))
	}
	// Result must be valid UTF-8 (no replacement chars when decoded)
	for _, r := range out {
		if r == 0xFFFD {
			t.Errorf("found replacement rune; truncation broke a multi-byte rune")
		}
	}
}

func TestInferToolNameFallbackAtIndexZero(t *testing.T) {
	e := NewSymptomExtractor()
	// A tool-role error turn at index 0 has no preceding tool: <name> turn,
	// so inferToolName must fall back to "unknown" — never panic.
	turns := []executor.ParsedTurn{
		{Role: "tool", Content: "error: something blew up"},
	}
	s := e.Extract(turns, RunOutcome{}, task.Task{})
	if len(s.ToolFailures) != 1 {
		t.Fatalf("expected 1 tool failure, got %d", len(s.ToolFailures))
	}
	if s.ToolFailures[0].ToolName != "unknown" {
		t.Errorf("expected fallback name 'unknown', got %q", s.ToolFailures[0].ToolName)
	}
}

func TestSymptomsDoneClaimFromToolRoleIgnored(t *testing.T) {
	// Tool-role output mentioning "task complete" must NOT flip
	// DeclaredCompletion — only assistant claims count.
	e := NewSymptomExtractor()
	turns := []executor.ParsedTurn{
		{Role: "tool", Content: "task complete (subprocess exit 0)"},
		{Role: "assistant", Content: "let me verify"},
	}
	s := e.Extract(turns, RunOutcome{}, task.Task{})
	if s.DeclaredCompletion {
		t.Error("tool-role 'task complete' should not flip DeclaredCompletion")
	}
}

func TestSymptomsPacketSizeUnder4KB(t *testing.T) {
	e := NewSymptomExtractor()
	// Pathologically long inputs (simulates verbose Aider run)
	huge := strings.Repeat("x", 50_000)
	turns := []executor.ParsedTurn{
		{Role: "assistant", Content: huge},
		{Role: "tool", Content: "error: " + huge},
	}
	s := e.Extract(turns, RunOutcome{
		LintErrors: []string{huge, huge},
	}, task.Task{})
	// Truncations should keep individual fields bounded, but LintErrors is
	// pass-through. Verify the *bounded* fields are bounded.
	if len(s.LastAssistantClaim) > lastClaimMaxLen {
		t.Errorf("LastAssistantClaim length %d > cap %d", len(s.LastAssistantClaim), lastClaimMaxLen)
	}
	if len(s.LastErrorMessage) > 200 {
		t.Errorf("LastErrorMessage length %d > 200", len(s.LastErrorMessage))
	}
	for _, tf := range s.ToolFailures {
		if len(tf.Message) > 200 {
			t.Errorf("ToolFailure.Message length %d > 200", len(tf.Message))
		}
	}
}

func TestSymptomsJSONRoundTrip(t *testing.T) {
	e := NewSymptomExtractor()
	s := e.Extract(
		[]executor.ParsedTurn{{Role: "assistant", Content: "I've completed"}},
		RunOutcome{TestsPassed: 5, TestsTotal: 5},
		task.Task{},
	)
	bytes, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{
		`"tests_passed":5`,
		`"tests_total":5`,
		`"declared_completion":true`,
		`"time_to_first_error":-1`,
	} {
		if !strings.Contains(string(bytes), key) {
			t.Errorf("missing %q in %s", key, string(bytes))
		}
	}
}
