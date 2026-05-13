package diagnostic

// RecoveryProfile captures the agent's error-event timeline and how it reacted.
// Deterministic; computed from the transcript alone.
type RecoveryProfile struct {
	ErrorEvents             []ErrorEvent `json:"error_events"`
	ErrorAcknowledgmentRate float64      `json:"error_acknowledgment_rate"`
	CorrectionLatencyMean   float64      `json:"correction_latency_mean"`
	CorrectionSuccessRate   float64      `json:"correction_success_rate"`
	SilentSkipCount         int          `json:"silent_skip_count"`
}

// ErrorKind is the typed enum for ErrorEvent.Type values.
type ErrorKind string

const (
	ErrorKindToolFailure  ErrorKind = "tool_failure"
	ErrorKindTestFailure  ErrorKind = "test_failure"
	ErrorKindStderr       ErrorKind = "stderr"
	ErrorKindCompileError ErrorKind = "compile_error"
)

// ErrorEvent is a single error observed during the run.
type ErrorEvent struct {
	TurnIndex int       `json:"turn_index"`
	Type      ErrorKind `json:"type"`
	ToolName  string    `json:"tool_name,omitempty"`
	Message   string    `json:"message"`
}
