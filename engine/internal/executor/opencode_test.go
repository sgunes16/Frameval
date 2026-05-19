package executor

import (
	"strings"
	"testing"
)

// TestOpenCodeParseTranscriptHappyPath pins the event-type → BlockKind
// mapping for the event types opencode emits in steady state. Each
// line of the stream is one JSON event; the parser translates them
// into structured ParsedTurns with no heuristics (unlike the aider
// parser which has to recover structure from chat output).
//
// step_start / step_finish are intentionally dropped — they're agent
// iteration boundaries with no human-readable payload and just create
// "System: step_start" noise rows between every real action.
func TestOpenCodeParseTranscriptHappyPath(t *testing.T) {
	raw := strings.Join([]string{
		`{"type":"step_start","timestamp":1,"sessionID":"s"}`,
		`{"type":"reasoning","timestamp":2,"sessionID":"s","part":{"text":"need to lock the read-modify-write"}}`,
		`{"type":"tool_use","timestamp":3,"sessionID":"s","part":{"tool":"Edit","state":{"input":{"path":"app/user_service.py","content":"…"}}}}`,
		`{"type":"text","timestamp":4,"sessionID":"s","part":{"text":"applied the lock"}}`,
		`{"type":"step_finish","timestamp":5,"sessionID":"s"}`,
		``,
	}, "\n")
	e := &OpenCodeExecutor{}
	turns, err := e.ParseTranscript([]byte(raw))
	if err != nil {
		t.Fatalf("ParseTranscript: %v", err)
	}
	if len(turns) != 3 {
		t.Fatalf("expected 3 turns (step_* dropped), got %d: %+v", len(turns), turns)
	}
	kinds := []string{turns[0].BlockKind, turns[1].BlockKind, turns[2].BlockKind}
	want := []string{BlockKindThinking, BlockKindToolUse, BlockKindText}
	for i, k := range want {
		if kinds[i] != k {
			t.Errorf("turn %d: kind = %q, want %q", i, kinds[i], k)
		}
	}
	if turns[1].ToolName != "Edit" {
		t.Errorf("tool_use turn: tool_name = %q, want Edit", turns[1].ToolName)
	}
	if len(turns[1].FilesTouched) != 1 || turns[1].FilesTouched[0] != "app/user_service.py" {
		t.Errorf("tool_use turn: files_touched = %v, want [app/user_service.py]", turns[1].FilesTouched)
	}
}

func TestOpenCodeParseTranscriptIgnoresJunkLines(t *testing.T) {
	raw := strings.Join([]string{
		"some non-JSON banner line",
		`{"type":"text","timestamp":1,"sessionID":"s","part":{"text":"hi"}}`,
		"another bare line",
		`{"type":"unknown_future_event","timestamp":2,"sessionID":"s"}`,
		``,
	}, "\n")
	e := &OpenCodeExecutor{}
	turns, err := e.ParseTranscript([]byte(raw))
	if err != nil {
		t.Fatalf("ParseTranscript: %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn (junk + unknown event dropped), got %d", len(turns))
	}
	if turns[0].BlockKind != BlockKindText {
		t.Errorf("expected text turn, got %q", turns[0].BlockKind)
	}
}

func TestOpenCodeParseTranscriptError(t *testing.T) {
	raw := `{"type":"error","timestamp":1,"sessionID":"s","error":"ollama refused connection"}` + "\n"
	e := &OpenCodeExecutor{}
	turns, err := e.ParseTranscript([]byte(raw))
	if err != nil {
		t.Fatalf("ParseTranscript: %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("expected 1 error turn, got %d", len(turns))
	}
	if turns[0].BlockKind != BlockKindSystem || turns[0].Stage != "error" {
		t.Errorf("error turn: kind=%q stage=%q, want system + error", turns[0].BlockKind, turns[0].Stage)
	}
}

func TestOpenCodeFallbackModelTranslatesAiderEnv(t *testing.T) {
	// Reuse-from-aider path: AIDER_MODEL=openai/llama3.1:8b should
	// translate to ollama/llama3.1:8b for opencode without the
	// user having to set OPENCODE_MODEL separately.
	got := fallbackOpenCodeModel("", map[string]string{"AIDER_MODEL": "openai/llama3.1:8b"})
	if got != "ollama/llama3.1:8b" {
		t.Errorf("got %q, want ollama/llama3.1:8b", got)
	}
}
