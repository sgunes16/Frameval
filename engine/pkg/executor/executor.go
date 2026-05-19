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

// Stable BlockKind values. Executors that don't distinguish should set
// "text" (the default semantic) so consumers don't have to special-case
// the empty string.
const (
	BlockKindThinking   = "thinking"
	BlockKindText       = "text"
	BlockKindToolUse    = "tool_use"
	BlockKindToolResult = "tool_result"
	BlockKindSystem     = "system"
)

// ParsedTurn is a single structured turn in an agent transcript.
//
// Defined here (in the public package) because third-party harnesses, executors,
// and diagnostic-pipeline consumers all need to read transcripts.
//
// Stage is set by harnesses that perform multi-stage invocations (e.g., speckit)
// so downstream consumers can group turns by stage.
//
// The Inspector-V2 fields (TurnIndex through TokensOut) are stamped by
// executors and/or AssignTurnGrouping. Legacy transcripts that predate the
// schema extension deserialize with zero values for these fields — UIs
// must treat zeroes as "not stamped" rather than "stamped to zero".
type ParsedTurn struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
	Stage     string `json:"stage,omitempty"`

	// TurnIndex is a 0-based monotonic counter across the run's transcript.
	// Assigned by AssignTurnGrouping. NOT `omitempty` because the first
	// turn legitimately has index 0 and omitting it would make a stamped
	// first turn indistinguishable from a legacy unstamped block on
	// re-marshal. Consumers check BlockKind != "" to know whether the
	// turn was stamped.
	TurnIndex int `json:"turn_index"`

	// BlockKind classifies the payload: thinking, text, tool_use,
	// tool_result, system. Empty == legacy "we didn't classify this".
	BlockKind string `json:"block_kind,omitempty"`

	// ToolUseID pairs a tool_use with the tool_result that answered it.
	// Empty on non-tool blocks.
	ToolUseID string `json:"tool_use_id,omitempty"`

	// ParentTurnIndex is the TurnIndex of the first block in this
	// "decision" (e.g., thinking → tool_use → tool_result all share the
	// thinking's TurnIndex). NOT `omitempty` for the same reason as
	// TurnIndex — a block that is its own parent legitimately has
	// ParentTurnIndex == 0.
	ParentTurnIndex int `json:"parent_turn_index"`

	// ToolName is set on tool_use blocks ("Edit", "Bash", "Read"). The
	// Tool Histogram sidebar groups on this.
	ToolName string `json:"tool_name,omitempty"`

	// FilesTouched names workspace paths this turn modified (tool_use
	// only). Compare V2's anchor algorithm hashes on (tool_name, files).
	FilesTouched []string `json:"files_touched,omitempty"`

	// DurationMs records how long this block took to produce, when the
	// executor can measure it (streamed CLIs can).
	DurationMs int `json:"duration_ms,omitempty"`

	// TokensIn / TokensOut are per-block token counts; useful for the
	// turn-by-turn cost view.
	TokensIn  int `json:"tokens_in,omitempty"`
	TokensOut int `json:"tokens_out,omitempty"`
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
