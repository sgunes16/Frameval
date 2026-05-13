package executor

import (
	"os"
	"strings"
	"testing"
)

func TestAiderParseTranscriptTagsRoles(t *testing.T) {
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
	if len(turns) != 5 {
		t.Fatalf("expected 5 turns, got %d", len(turns))
	}

	wantRoles := []string{"user", "assistant", "tool", "system", "assistant"}
	for i, want := range wantRoles {
		if turns[i].Role != want {
			t.Errorf("turn %d: want role %q, got %q (content=%q)", i, want, turns[i].Role, turns[i].Content)
		}
	}
}

func TestAiderParseTranscriptSkipsEmpty(t *testing.T) {
	e := &AiderExecutor{}
	raw := []byte("assistant: line one\n\n  \nassistant: line two\n")
	turns, err := e.ParseTranscript(raw)
	if err != nil {
		t.Fatalf("ParseTranscript returned error: %v", err)
	}
	if len(turns) != 2 {
		t.Fatalf("expected 2 turns (empty lines skipped), got %d", len(turns))
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

func TestDetectAiderRole(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"user: hello", "user"},
		{"assistant: ok", "assistant"},
		{"tool: file_write x", "tool"},
		{"system: ready", "system"},
		{"USER: caps ok", "user"},
		{"no prefix", "assistant"},
		{"asst: not a known prefix", "assistant"},
	}
	for _, tc := range cases {
		if got := detectAiderRole(tc.line); got != tc.want {
			t.Errorf("detectAiderRole(%q) = %q, want %q", tc.line, got, tc.want)
		}
	}
}

func TestFallbackAiderModelPrecedence(t *testing.T) {
	t.Setenv("AIDER_MODEL", "")
	if got := fallbackAiderModel(""); got != "openai/qwen2.5-coder:7b" {
		t.Errorf("default model: got %q", got)
	}
	if got := fallbackAiderModel("openai/explicit-model"); got != "openai/explicit-model" {
		t.Errorf("explicit cfg.Model not used: got %q", got)
	}
	t.Setenv("AIDER_MODEL", "openai/env-model")
	if got := fallbackAiderModel(""); got != "openai/env-model" {
		t.Errorf("env override not used: got %q", got)
	}
	// cfg.Model still wins over env.
	if got := fallbackAiderModel("openai/explicit-model"); got != "openai/explicit-model" {
		t.Errorf("cfg.Model should win over env: got %q", got)
	}
}

func TestFallbackOllamaBase(t *testing.T) {
	t.Setenv("OLLAMA_BASE_URL", "")
	if got := fallbackOllamaBase(); got != "http://host.docker.internal:11434/v1" {
		t.Errorf("default base: got %q", got)
	}
	t.Setenv("OLLAMA_BASE_URL", "http://customhost:9999/v1")
	if got := fallbackOllamaBase(); got != "http://customhost:9999/v1" {
		t.Errorf("env override: got %q", got)
	}
}

func TestFallbackOllamaKeyDefault(t *testing.T) {
	_ = os.Unsetenv("OPENAI_API_KEY")
	if got := fallbackOllamaKey(); got != "ollama" {
		t.Errorf("default key: got %q", got)
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
