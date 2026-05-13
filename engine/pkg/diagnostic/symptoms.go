package diagnostic

// Symptoms is the compact (~500–1000 token) packet built deterministically from
// a completed run. The failure classifier consumes Symptoms (not the full
// transcript) to keep its inputs short and signal-dense.
type Symptoms struct {
	TestsPassed   int `json:"tests_passed"`
	TestsFailed   int `json:"tests_failed"`
	TestsTotal    int `json:"tests_total"`
	CompileFailed bool `json:"compile_failed"`
	LintErrors    []string `json:"lint_errors,omitempty"`

	ToolFailures     []ToolFailure `json:"tool_failures,omitempty"`
	LastErrorMessage string        `json:"last_error_message,omitempty"`

	FilesTouched []string `json:"files_touched,omitempty"`
	FilesCreated []string `json:"files_created,omitempty"`
	FilesDeleted []string `json:"files_deleted,omitempty"`

	LastAssistantClaim  string `json:"last_assistant_claim,omitempty"`
	DeclaredCompletion  bool   `json:"declared_completion"`
	AcknowledgedFailure bool   `json:"acknowledged_failure"`

	TimeToFirstError int     `json:"time_to_first_error"`
	TimeoutHit       bool    `json:"timeout_hit"`
	WallClockSeconds float64 `json:"wall_clock_seconds"`

	// UnexpectedFilesModified is populated for brownfield tasks where
	// task.expected_files_modified is set; surfaces SCOPE_DRIFT signal.
	UnexpectedFilesModified []string `json:"unexpected_files_modified,omitempty"`
}

// ToolFailure is a single failing tool invocation observed in the transcript.
type ToolFailure struct {
	TurnIndex int    `json:"turn_index"`
	ToolName  string `json:"tool_name"`
	Message   string `json:"message"`
}
