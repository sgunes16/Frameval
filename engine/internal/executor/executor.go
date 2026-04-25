package executor

import (
	"context"

	"github.com/mustafaselman/frameval/engine/internal/models"
)

type ExecutionMode string

const (
	ExecutionModeCLI ExecutionMode = "cli"
	ExecutionModeAPI ExecutionMode = "api"
)

type RunConfig struct {
	WorkspacePath string
	Prompt        string
	Model         string
	Environment   map[string]string
	OnOutput      func(line string)
}

type RunResult struct {
	RawOutput      string
	ParsedTurns    []models.ParsedTurn
	StreamedOutput bool
}

type AgentExecutor interface {
	Name() string
	SupportedModes() []ExecutionMode
	Execute(ctx context.Context, cfg RunConfig) (*RunResult, error)
	ParseTranscript(raw []byte) (*models.Transcript, error)
}
