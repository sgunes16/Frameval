package executor

import (
	"context"
	"strings"
	"testing"
)

func TestCursorExecuteReturnsErrNotConfiguredWhenKeyMissing(t *testing.T) {
	t.Setenv("CURSOR_API_KEY", "")
	exec := &CursorExecutor{}
	result, err := exec.Execute(context.Background(), RunConfig{Prompt: "hello"})
	if err != nil {
		t.Fatalf("expected nil error (downstream classifies as ENV_ERR), got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil RunResult")
	}
	if !strings.Contains(result.RawOutput, "CURSOR_API_KEY not set") {
		t.Errorf("expected RawOutput to mention missing key; got %q", result.RawOutput)
	}
	if len(result.ParsedTurns) == 0 {
		t.Error("expected at least one parsed turn from the skip-marker raw output")
	}
}

func TestCursorParseTranscriptHandlesJSON(t *testing.T) {
	e := &CursorExecutor{}
	raw := []byte(strings.Join([]string{
		`{"type":"assistant","role":"assistant","content":"hello"}`,
		`{"type":"tool_call","role":"tool","content":"write file foo.py"}`,
		`raw plain line without json`,
	}, "\n"))
	turns, err := e.ParseTranscript(raw)
	if err != nil {
		t.Fatalf("ParseTranscript: %v", err)
	}
	if len(turns) != 3 {
		t.Fatalf("expected 3 turns, got %d", len(turns))
	}
	if turns[0].Role != "assistant" || turns[0].Content != "hello" {
		t.Errorf("turn 0 mismatch: %+v", turns[0])
	}
	if turns[1].Role != "tool" || turns[1].Content != "write file foo.py" {
		t.Errorf("turn 1 mismatch: %+v", turns[1])
	}
	// Plain line falls back to assistant with the original content
	if turns[2].Role != "assistant" || turns[2].Content != "raw plain line without json" {
		t.Errorf("turn 2 mismatch: %+v", turns[2])
	}
}

func TestCursorParseTranscriptSkipsEmpty(t *testing.T) {
	e := &CursorExecutor{}
	raw := []byte("\n\n   \n{\"role\":\"assistant\",\"content\":\"a\"}\n")
	turns, _ := e.ParseTranscript(raw)
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn after skipping blanks, got %d", len(turns))
	}
	if turns[0].Content != "a" {
		t.Errorf("content mismatch: %q", turns[0].Content)
	}
}

func TestFallbackModelDefaultsToAuto(t *testing.T) {
	if got := fallbackModel(""); got != "auto" {
		t.Errorf("default model: got %q", got)
	}
	if got := fallbackModel("opus-1m"); got != "opus-1m" {
		t.Errorf("explicit model: got %q", got)
	}
}
