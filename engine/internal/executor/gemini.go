package executor

import (
	"context"
	"os"
	"os/exec"

	"github.com/mustafaselman/frameval/engine/internal/models"
	"github.com/mustafaselman/frameval/engine/internal/sandbox"
)

type GeminiExecutor struct {
	sandbox *sandbox.Manager
}

func NewGeminiExecutor(manager *sandbox.Manager) *GeminiExecutor {
	return &GeminiExecutor{sandbox: manager}
}

func (e *GeminiExecutor) Name() string                    { return "gemini" }
func (e *GeminiExecutor) SupportedModes() []ExecutionMode { return []ExecutionMode{ExecutionModeCLI} }

func (e *GeminiExecutor) Execute(ctx context.Context, cfg RunConfig) (*RunResult, error) {
	prompt := promptWithDefaultCLILanguage(cfg.Prompt)
	command := os.Getenv("FRAMEVAL_GEMINI_COMMAND")
	if command == "" {
		if _, err := exec.LookPath("gemini"); err != nil {
			raw := "gemini binary not found; execution skipped\nPrompt:\n" + prompt
			transcript, _ := e.ParseTranscript([]byte(raw))
			return &RunResult{RawOutput: raw, ParsedTurns: transcript.ParsedTurns}, nil
		}
		command = `printf '%s' "$FRAMEVAL_PROMPT" | gemini`
	}
	output, err := e.sandbox.RunShell(ctx, cfg.WorkspacePath, mergeEnv(cfg.Environment, map[string]string{"FRAMEVAL_PROMPT": prompt}), command)
	transcript, _ := e.ParseTranscript([]byte(output))
	return &RunResult{RawOutput: output, ParsedTurns: transcript.ParsedTurns}, err
}

func (e *GeminiExecutor) ParseTranscript(raw []byte) (*models.Transcript, error) {
	return parseTranscript(raw), nil
}
