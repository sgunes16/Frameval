package executor

import (
	"context"
	"errors"

	"github.com/mustafaselman/frameval/engine/internal/models"
)

type APIModeExecutor struct{}

func (e *APIModeExecutor) Name() string                    { return "api" }
func (e *APIModeExecutor) SupportedModes() []ExecutionMode { return []ExecutionMode{ExecutionModeAPI} }
func (e *APIModeExecutor) Execute(ctx context.Context, cfg RunConfig) (*RunResult, error) {
	return nil, errors.New("api mode executor not implemented yet")
}
func (e *APIModeExecutor) ParseTranscript(raw []byte) (*models.Transcript, error) {
	return &models.Transcript{RawOutput: string(raw)}, nil
}
