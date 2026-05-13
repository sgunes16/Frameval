package executor

import "context"

// ExecutionMode describes how an executor invokes an agent.
type ExecutionMode string

const (
	// ExecutionModeCLI runs an agent CLI binary inside a sandbox container.
	ExecutionModeCLI ExecutionMode = "cli"
	// ExecutionModeAPI calls an LLM provider API directly with tool-use simulation.
	ExecutionModeAPI ExecutionMode = "api"
)

// RunConfig is the input contract for a single agent invocation.
//
// Harnesses build RunConfigs and pass them to AgentExecutor.Execute. The same
// RunConfig may be invoked multiple times by a harness (Ralph loop, multi-agent
// roles, etc.) within a single AgentDx run.
//
// Stage / Role are optional metadata harnesses use to differentiate sub-calls
// within a single run (e.g., spec-kit's 4 stages, planner+coder's 2 roles).
// Executors that don't care can ignore them; downstream AgentDx readers can
// surface the distinction in the Compare view.
type RunConfig struct {
	WorkspacePath string
	Prompt        string
	Model         string
	Environment   map[string]string
	OnOutput      func(line string)
	Stage         string
	Role          string
}

// RunResult is what an executor returns after one invocation.
//
// The orchestrator combines this with run-level metadata (run ID, container ID,
// timing) when persisting a full Transcript in the database.
type RunResult struct {
	RawOutput      string
	ParsedTurns    []ParsedTurn
	StreamedOutput bool
}

// ParsedTurn is a single structured turn in an agent transcript.
//
// Defined here (in the public package) because third-party harnesses, executors,
// and diagnostic-pipeline consumers all need to read transcripts.
//
// Stage is set by harnesses that perform multi-stage invocations (e.g., speckit)
// so downstream consumers can group turns by stage.
type ParsedTurn struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
	Stage     string `json:"stage,omitempty"`
}

// AgentExecutor abstracts a single agent CLI or API backend.
//
// Implementations live under engine/internal/builtin/executor (for shipped
// executors) or in third-party packages. Register an external executor by
// satisfying this interface and inserting it into the Registry at startup.
type AgentExecutor interface {
	// Name is the stable identifier used in configs (e.g., "aider", "cursor").
	Name() string

	// SupportedModes lists which execution modes the executor implements.
	SupportedModes() []ExecutionMode

	// Execute performs one agent invocation against the supplied workspace and prompt.
	Execute(ctx context.Context, cfg RunConfig) (*RunResult, error)

	// ParseTranscript converts raw executor output into a structured transcript.
	// Used when re-grading historical runs from stored raw output.
	ParseTranscript(raw []byte) ([]ParsedTurn, error)
}
