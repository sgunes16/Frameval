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

// TestOpenCodeTokenCostAccumulation verifies that step_finish events are used
// to accumulate real token counts and cost onto the RunResult, without producing
// any ParsedTurn rows (those would pollute the Inspector).
//
// Summation rule: per-step input GROWS (accumulating context), so summing
// input across steps double-counts. Correct formula:
//
//	TotalTokens = Σ per-step output + FINAL step's input
//	CostUSD     = Σ per-step cost   (safe to sum)
//
// With step_finish #1: output=101, input=4000, cost=0.001
//      step_finish #2: output=50,  input=8140, cost=0.002
//
// Expected TotalTokens = 101 + 50 + 8140 = 8291
// Expected CostUSD     = 0.001 + 0.002  = 0.003
func TestOpenCodeTokenCostAccumulation(t *testing.T) {
	raw := strings.Join([]string{
		`{"type":"step_start","timestamp":1,"sessionID":"s"}`,
		`{"type":"text","timestamp":2,"sessionID":"s","part":{"text":"thinking..."}}`,
		`{"type":"step_finish","timestamp":3,"sessionID":"s","part":{"tokens":{"total":4101,"input":4000,"output":101,"reasoning":0,"cache":{"write":0,"read":0}},"cost":0.001}}`,
		`{"type":"step_start","timestamp":4,"sessionID":"s"}`,
		`{"type":"text","timestamp":5,"sessionID":"s","part":{"text":"done"}}`,
		`{"type":"step_finish","timestamp":6,"sessionID":"s","part":{"tokens":{"total":8259,"input":8140,"output":50,"reasoning":18,"cache":{"write":0,"read":0}},"cost":0.002}}`,
		``,
	}, "\n")

	tokens, cost := accumulateOpenCodeTokensAndCost([]byte(raw))

	wantTokens := 8291 // 101 + 50 + 8140
	if tokens != wantTokens {
		t.Errorf("TotalTokens = %d, want %d", tokens, wantTokens)
	}

	const wantCost = 0.003
	const eps = 1e-9
	diff := cost - wantCost
	if diff < -eps || diff > eps {
		t.Errorf("CostUSD = %f, want %f", cost, wantCost)
	}
}

// TestOpenCodeTokenCostNoStepFinish verifies that a stream with no step_finish
// events yields zero tokens and zero cost (not a panic or junk value).
func TestOpenCodeTokenCostNoStepFinish(t *testing.T) {
	raw := strings.Join([]string{
		`{"type":"text","timestamp":1,"sessionID":"s","part":{"text":"hello"}}`,
		``,
	}, "\n")

	tokens, cost := accumulateOpenCodeTokensAndCost([]byte(raw))
	if tokens != 0 {
		t.Errorf("TotalTokens = %d, want 0 when no step_finish events", tokens)
	}
	if cost != 0 {
		t.Errorf("CostUSD = %f, want 0 when no step_finish events", cost)
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
