package executor

import (
	"os"
	"strings"
	"testing"
)

func TestAiderParseTranscriptGroupsByRole(t *testing.T) {
	e := &AiderExecutor{}
	raw := []byte(strings.Join([]string{
		"user: please add CLI scaffolding",
		"assistant: sure, creating main.py",
		"tool: file_write main.py",
		"system: edit applied",
		"random line without role prefix",
	}, "\n"))

	turns, err := e.ParseTranscript(raw)
	if err != nil {
		t.Fatalf("ParseTranscript returned error: %v", err)
	}
	// Unmarked lines fold into the PREVIOUS role's turn (continuation),
	// so the trailing "random line" merges into the system turn — yielding
	// 4 turns total, not 5.
	if len(turns) != 4 {
		t.Fatalf("expected 4 turns (unmarked line folded into prior), got %d", len(turns))
	}
	wantRoles := []string{"user", "assistant", "tool", "system"}
	for i, want := range wantRoles {
		if turns[i].Role != want {
			t.Errorf("turn %d: want role %q, got %q (content=%q)", i, want, turns[i].Role, turns[i].Content)
		}
	}
	if !strings.Contains(turns[3].Content, "random line without role prefix") {
		t.Errorf("trailing unmarked line should have folded into system turn; got %q", turns[3].Content)
	}
}

func TestAiderParseTranscriptMultilineAssistant(t *testing.T) {
	// Reproduces the bug screenshot: a multi-line assistant response should
	// NOT explode into one turn per line. Before the fix, this raw output
	// produced 6 turns; after, it produces 1 assistant turn whose Content
	// is the full multi-line body.
	e := &AiderExecutor{}
	raw := []byte("assistant: Building wordfreq CLI.\n  -k for top-K\n  -c for case-sensitive\nThis is the plan.")
	turns, err := e.ParseTranscript(raw)
	if err != nil {
		t.Fatalf("ParseTranscript returned error: %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn (multi-line assistant), got %d", len(turns))
	}
	if !strings.Contains(turns[0].Content, "-k for top-K") {
		t.Errorf("turn should preserve continuation lines, got %q", turns[0].Content)
	}
}

func TestAiderParseTranscriptUnstructuredOutput(t *testing.T) {
	// Tool stdout with no role prefixes anywhere — common when Aider's
	// CLI emits raw shell-style output. Should render as a single
	// assistant turn rather than per-line cards.
	e := &AiderExecutor{}
	raw := []byte("Loading model...\nReady.\nGenerating output\n")
	turns, err := e.ParseTranscript(raw)
	if err != nil {
		t.Fatalf("ParseTranscript returned error: %v", err)
	}
	if len(turns) != 1 || turns[0].Role != "assistant" {
		t.Fatalf("expected 1 assistant turn for unmarked output, got %d turns role=%v", len(turns), turns)
	}
}

func TestAiderParseTranscriptEmptyInput(t *testing.T) {
	e := &AiderExecutor{}
	turns, err := e.ParseTranscript([]byte(""))
	if err != nil {
		t.Fatalf("ParseTranscript returned error: %v", err)
	}
	if len(turns) != 0 {
		t.Fatalf("expected 0 turns for empty input, got %d", len(turns))
	}
}

func TestDetectAiderRoleStrict(t *testing.T) {
	// Strict variant returns "" for unprefixed lines so the parser can
	// treat them as continuations of the previous turn rather than
	// fabricating a new assistant turn per line.
	cases := []struct {
		line string
		want string
	}{
		{"user: hello", "user"},
		{"assistant: ok", "assistant"},
		{"tool: file_write x", "tool"},
		{"system: ready", "system"},
		{"USER: caps ok", "user"},
		{"no prefix", ""},
		{"asst: not a known prefix", ""},
	}
	for _, tc := range cases {
		if got := detectAiderRoleStrict(tc.line); got != tc.want {
			t.Errorf("detectAiderRoleStrict(%q) = %q, want %q", tc.line, got, tc.want)
		}
	}
}

func TestFallbackAiderModelPrecedence(t *testing.T) {
	t.Setenv("AIDER_MODEL", "")
	if got := fallbackAiderModel("", nil); got != "openai/qwen2.5-coder:7b" {
		t.Errorf("default model: got %q", got)
	}
	if got := fallbackAiderModel("openai/explicit-model", nil); got != "openai/explicit-model" {
		t.Errorf("explicit cfg.Model not used: got %q", got)
	}
	t.Setenv("AIDER_MODEL", "openai/env-model")
	if got := fallbackAiderModel("", nil); got != "openai/env-model" {
		t.Errorf("OS env override not used: got %q", got)
	}
	// cfg.Model still wins over env.
	if got := fallbackAiderModel("openai/explicit-model", nil); got != "openai/explicit-model" {
		t.Errorf("cfg.Model should win over env: got %q", got)
	}
	// cfg.Environment beats OS env.
	if got := fallbackAiderModel("", map[string]string{"AIDER_MODEL": "openai/cfg-env-model"}); got != "openai/cfg-env-model" {
		t.Errorf("cfg.Environment should win over OS env: got %q", got)
	}
}

func TestFallbackOllamaBase(t *testing.T) {
	t.Setenv("OLLAMA_BASE_URL", "")
	if got := fallbackOllamaBase(nil); got != "http://host.docker.internal:11434/v1" {
		t.Errorf("default base: got %q", got)
	}
	t.Setenv("OLLAMA_BASE_URL", "http://customhost:9999/v1")
	if got := fallbackOllamaBase(nil); got != "http://customhost:9999/v1" {
		t.Errorf("OS env override: got %q", got)
	}
	if got := fallbackOllamaBase(map[string]string{"OLLAMA_BASE_URL": "http://cfg-env/v1"}); got != "http://cfg-env/v1" {
		t.Errorf("cfg.Environment should win over OS env: got %q", got)
	}
}

func TestFallbackOllamaKeyDefault(t *testing.T) {
	_ = os.Unsetenv("OPENAI_API_KEY")
	if got := fallbackOllamaKey(nil); got != "ollama" {
		t.Errorf("default key: got %q", got)
	}
	if got := fallbackOllamaKey(map[string]string{"OPENAI_API_KEY": "cfg-env-key"}); got != "cfg-env-key" {
		t.Errorf("cfg.Environment should win: got %q", got)
	}
}

func TestMergeEnvCallerWins(t *testing.T) {
	defaults := map[string]string{
		"AIDER_MODEL":     "openai/qwen2.5-coder:7b",
		"OPENAI_API_BASE": "http://host.docker.internal:11434/v1",
		"FRAMEVAL_PROMPT": "default-prompt",
	}
	callerOverrides := map[string]string{
		"AIDER_MODEL": "openai/different-model",
		"NEW_VAR":     "added",
	}
	merged := mergeEnv(defaults, callerOverrides)

	if merged["AIDER_MODEL"] != "openai/different-model" {
		t.Errorf("caller override should win: got %q", merged["AIDER_MODEL"])
	}
	if merged["OPENAI_API_BASE"] != "http://host.docker.internal:11434/v1" {
		t.Errorf("default should remain when not overridden: got %q", merged["OPENAI_API_BASE"])
	}
	if merged["FRAMEVAL_PROMPT"] != "default-prompt" {
		t.Errorf("default should remain: got %q", merged["FRAMEVAL_PROMPT"])
	}
	if merged["NEW_VAR"] != "added" {
		t.Errorf("caller-only key should be present: got %q", merged["NEW_VAR"])
	}
}

func TestAiderRegistered(t *testing.T) {
	r := &Registry{executors: map[string]AgentExecutor{
		"aider": &AiderExecutor{},
	}}
	exec, err := r.Get("aider")
	if err != nil {
		t.Fatalf("Get aider: %v", err)
	}
	if exec.Name() != "aider" {
		t.Errorf("expected Name=aider, got %q", exec.Name())
	}
	modes := exec.SupportedModes()
	if len(modes) != 1 || modes[0] != ExecutionModeCLI {
		t.Errorf("expected [cli], got %v", modes)
	}
}
