package executor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mustafaselman/frameval/engine/internal/sandbox"
)

// ErrCursorNotConfigured is surfaced inside the returned RunResult when the
// Cursor agent backend cannot be reached. Callers receive a non-nil RunResult
// with a descriptive RawOutput rather than a hard error, so downstream
// AgentDx still classifies the run (typically as ENV_ERR) instead of crashing.
var ErrCursorNotConfigured = errors.New("CURSOR_API_KEY not set; cursor executor cannot reach the agent backend")

// CursorExecutor invokes Cursor's CLI agent against the Cursor cloud backend.
// Requires the `agent` binary on PATH (installed via the sandbox Dockerfile)
// and CURSOR_API_KEY in the executor environment.
type CursorExecutor struct {
	sandbox *sandbox.Manager
}

// NewCursorExecutor constructs the Cursor adapter. The sandbox manager owns
// the container lifecycle; this executor only issues shell commands inside it.
func NewCursorExecutor(manager *sandbox.Manager) *CursorExecutor {
	return &CursorExecutor{sandbox: manager}
}

func (e *CursorExecutor) Name() string { return "cursor" }

func (e *CursorExecutor) SupportedModes() []ExecutionMode {
	return []ExecutionMode{ExecutionModeCLI}
}

// Execute invokes Cursor's agent (auto mode) inside the sandbox workspace.
//
// Behavior on missing prerequisites:
//   - CURSOR_API_KEY unset → returns a RunResult with ErrCursorNotConfigured
//     mention in RawOutput; downstream classifies as ENV_ERR.
//   - agent binary not on PATH → returns a RunResult with a "binary not found"
//     RawOutput; downstream classifies as ENV_ERR.
//
// Both paths return a non-nil RunResult and nil error so the orchestrator can
// still persist a transcript and run AgentDx against it.
func (e *CursorExecutor) Execute(ctx context.Context, cfg RunConfig) (*RunResult, error) {
	prompt := promptWithDefaultCLILanguage(cfg.Prompt)
	if os.Getenv("CURSOR_API_KEY") == "" {
		raw := ErrCursorNotConfigured.Error() + "\nPrompt:\n" + prompt
		turns, _ := e.ParseTranscript([]byte(raw))
		return &RunResult{RawOutput: raw, ParsedTurns: turns}, nil
	}
	command := os.Getenv("FRAMEVAL_CURSOR_COMMAND")
	if command == "" {
		if _, err := exec.LookPath("agent"); err != nil {
			raw := "cursor agent binary not found on PATH; execution skipped\nPrompt:\n" + prompt
			turns, _ := e.ParseTranscript([]byte(raw))
			return &RunResult{RawOutput: raw, ParsedTurns: turns}, nil
		}
		command = `agent -p --force --output-format stream-json --stream-partial-output --model "$FRAMEVAL_MODEL_ID" "$FRAMEVAL_PROMPT"`
	}
	env := mergeEnv(map[string]string{
		"FRAMEVAL_PROMPT":   prompt,
		"FRAMEVAL_MODEL_ID": fallbackModel(cfg.Model),
	}, cfg.Environment)
	output, err := e.sandbox.RunShellWithOutput(ctx, cfg.WorkspacePath, env, command, cfg.OnOutput)
	turns, _ := e.ParseTranscript([]byte(output))
	return &RunResult{RawOutput: output, ParsedTurns: turns, StreamedOutput: cfg.OnOutput != nil}, err
}

// ParseTranscript turns Cursor's stream-json output into structured turns.
//
// Cursor emits one JSON object per line in stream-json mode; lines that aren't
// valid JSON or don't carry a recognized `type` field fall back to a raw
// "assistant" turn. The orchestrator wraps the returned turns into a
// models.Transcript with run-level metadata before persisting.
type cursorStreamEvent struct {
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (e *CursorExecutor) ParseTranscript(raw []byte) ([]ParsedTurn, error) {
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	turns := make([]ParsedTurn, 0, len(lines))
	now := time.Now().UTC().Format(time.RFC3339)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		var event cursorStreamEvent
		role := "assistant"
		content := line
		if strings.HasPrefix(trimmed, "{") && json.Unmarshal([]byte(trimmed), &event) == nil {
			if event.Role != "" {
				role = event.Role
			} else if event.Type != "" {
				role = event.Type
			}
			if event.Content != "" {
				content = event.Content
			}
		}
		turns = append(turns, ParsedTurn{
			Role:      role,
			Content:   content,
			Timestamp: now,
		})
	}
	return turns, nil
}

func mergeEnv(base map[string]string, additions map[string]string) map[string]string {
	merged := map[string]string{}
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range additions {
		merged[key] = value
	}
	return merged
}

func fallbackModel(model string) string {
	if strings.TrimSpace(model) == "" {
		return "auto"
	}
	return model
}
