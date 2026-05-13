package executor

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mustafaselman/frameval/engine/internal/sandbox"
)

type CursorExecutor struct {
	sandbox *sandbox.Manager
}

func NewCursorExecutor(manager *sandbox.Manager) *CursorExecutor {
	return &CursorExecutor{sandbox: manager}
}

func (e *CursorExecutor) Name() string { return "cursor" }

func (e *CursorExecutor) SupportedModes() []ExecutionMode { return []ExecutionMode{ExecutionModeCLI} }

func (e *CursorExecutor) Execute(ctx context.Context, cfg RunConfig) (*RunResult, error) {
	prompt := promptWithDefaultCLILanguage(cfg.Prompt)
	command := os.Getenv("FRAMEVAL_CURSOR_COMMAND")
	if command == "" {
		if _, err := exec.LookPath("agent"); err != nil {
			raw := "cursor agent binary not found; execution skipped\nPrompt:\n" + prompt
			turns, _ := e.ParseTranscript([]byte(raw))
			return &RunResult{RawOutput: raw, ParsedTurns: turns}, nil
		}
		command = `agent -p --force --output-format stream-json --stream-partial-output --model "$FRAMEVAL_MODEL_ID" "$FRAMEVAL_PROMPT"`
	}
	output, err := e.sandbox.RunShellWithOutput(ctx, cfg.WorkspacePath, mergeEnv(cfg.Environment, map[string]string{"FRAMEVAL_PROMPT": prompt, "FRAMEVAL_MODEL_ID": fallbackModel(cfg.Model)}), command, cfg.OnOutput)
	turns, _ := e.ParseTranscript([]byte(output))
	return &RunResult{RawOutput: output, ParsedTurns: turns, StreamedOutput: cfg.OnOutput != nil}, err
}

// ParseTranscript splits raw streaming output into structured turns.
//
// The orchestrator wraps the returned turns into a models.Transcript with
// run-level metadata (run ID, fs diff, patch) before persisting.
func (e *CursorExecutor) ParseTranscript(raw []byte) ([]ParsedTurn, error) {
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	turns := make([]ParsedTurn, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		turns = append(turns, ParsedTurn{
			Role:      "assistant",
			Content:   line,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
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
