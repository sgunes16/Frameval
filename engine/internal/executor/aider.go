package executor

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/mustafaselman/frameval/engine/internal/sandbox"
)

// AiderExecutor runs Aider against a local Ollama server (or any OpenAI-compatible endpoint).
//
// Defaults (overridable via env):
//
//	FRAMEVAL_AIDER_COMMAND   — full shell command (escape hatch; if set, used verbatim)
//	OLLAMA_BASE_URL          — OpenAI-compatible endpoint (default http://host.docker.internal:11434/v1)
//	AIDER_MODEL              — model identifier passed to Aider (default openai/qwen2.5-coder:7b)
//
// Aider's invocation:
//
//	aider --model "$AIDER_MODEL"
//	      --openai-api-base "$OLLAMA_BASE_URL"
//	      --openai-api-key "ollama"
//	      --no-stream --yes-always --no-pretty
//	      --message "$FRAMEVAL_PROMPT"
//
// Output capture:
//
//	stdout is streamed via cfg.OnOutput line-by-line (if set) and accumulated
//	for the final RunResult.RawOutput. Aider also writes .aider.chat.history.md
//	into the workspace; ParseTranscript prefers it when present, otherwise falls
//	back to stdout heuristics.
type AiderExecutor struct {
	sandbox *sandbox.Manager
}

// NewAiderExecutor constructs the Aider adapter. The sandbox manager owns the
// container lifecycle; this executor only issues shell commands inside it.
func NewAiderExecutor(manager *sandbox.Manager) *AiderExecutor {
	return &AiderExecutor{sandbox: manager}
}

func (e *AiderExecutor) Name() string { return "aider" }

func (e *AiderExecutor) SupportedModes() []ExecutionMode {
	return []ExecutionMode{ExecutionModeCLI}
}

// Execute invokes Aider in the sandbox workspace with the configured model and endpoint.
//
// If the aider binary is not on PATH, returns a fast-fail RunResult with a
// descriptive raw message; the run is not aborted so downstream diagnostic still
// receives a transcript (it will be classified as ENV_ERR).
func (e *AiderExecutor) Execute(ctx context.Context, cfg RunConfig) (*RunResult, error) {
	prompt := promptWithDefaultCLILanguage(cfg.Prompt)
	command := os.Getenv("FRAMEVAL_AIDER_COMMAND")
	if command == "" {
		if _, err := exec.LookPath("aider"); err != nil {
			raw := "aider binary not found on PATH; execution skipped\nPrompt:\n" + prompt
			turns, _ := e.ParseTranscript([]byte(raw))
			return &RunResult{RawOutput: raw, ParsedTurns: turns}, nil
		}
		command = `aider --model "$AIDER_MODEL" --openai-api-base "$OPENAI_API_BASE" --openai-api-key "$OPENAI_API_KEY" --no-stream --yes-always --no-pretty --message "$FRAMEVAL_PROMPT"`
	}
	// Defaults first, caller's cfg.Environment overrides last (caller wins).
	env := mergeEnv(map[string]string{
		"FRAMEVAL_PROMPT": prompt,
		"AIDER_MODEL":     fallbackAiderModel(cfg.Model),
		"OPENAI_API_BASE": fallbackOllamaBase(),
		"OPENAI_API_KEY":  fallbackOllamaKey(),
	}, cfg.Environment)
	output, err := e.sandbox.RunShellWithOutput(ctx, cfg.WorkspacePath, env, command, cfg.OnOutput)
	turns, _ := e.ParseTranscript([]byte(output))
	return &RunResult{RawOutput: output, ParsedTurns: turns, StreamedOutput: cfg.OnOutput != nil}, err
}

// ParseTranscript turns Aider's stdout into structured turns.
//
// Aider's --no-stream output is line-oriented. Lines that start with the agent
// role prefix Aider emits (e.g., "user: ", "assistant: ", "tool: ") are tagged
// accordingly; everything else is treated as assistant output. Empty lines are
// skipped. This is a pragmatic parser tuned for Aider's current text output;
// the orchestrator wraps the returned turns in a models.Transcript with run-level
// metadata before persisting.
func (e *AiderExecutor) ParseTranscript(raw []byte) ([]ParsedTurn, error) {
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	turns := make([]ParsedTurn, 0, len(lines))
	now := time.Now().UTC().Format(time.RFC3339)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		role := detectAiderRole(trimmed)
		turns = append(turns, ParsedTurn{
			Role:      role,
			Content:   line,
			Timestamp: now,
		})
	}
	return turns, nil
}

var aiderRolePrefix = regexp.MustCompile(`^(user|assistant|tool|system)\s*:\s*`)

func detectAiderRole(line string) string {
	match := aiderRolePrefix.FindStringSubmatch(strings.ToLower(line))
	if len(match) == 2 {
		return match[1]
	}
	return "assistant"
}

func fallbackAiderModel(model string) string {
	if strings.TrimSpace(model) != "" {
		return model
	}
	if env := os.Getenv("AIDER_MODEL"); env != "" {
		return env
	}
	return "openai/qwen2.5-coder:7b"
}

func fallbackOllamaBase() string {
	if env := os.Getenv("OLLAMA_BASE_URL"); env != "" {
		return env
	}
	return "http://host.docker.internal:11434/v1"
}

func fallbackOllamaKey() string {
	// Ollama ignores the key but Aider's OpenAI client requires one to be set.
	if env := os.Getenv("OPENAI_API_KEY"); env != "" {
		return env
	}
	return "ollama"
}

