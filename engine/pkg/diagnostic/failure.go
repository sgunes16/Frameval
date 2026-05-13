package diagnostic

// FailureCode is the canonical AgentDx failure-mode taxonomy.
//
// 12 failure categories plus a NONE sentinel for successful runs.
// Definitions are documented in
// docs/superpowers/specs/2026-05-12-agentdx-design.md Appendix A.
type FailureCode string

const (
	FailureNone       FailureCode = "NONE"
	FailureHalAPI     FailureCode = "HAL_API"
	FailureHalFile    FailureCode = "HAL_FILE"
	FailureDepMiss    FailureCode = "DEP_MISS"
	FailureStopEarly  FailureCode = "STOP_EARLY"
	FailureStopGiveup FailureCode = "STOP_GIVEUP"
	FailureLoopInf    FailureCode = "LOOP_INF"
	FailureWrongAbs   FailureCode = "WRONG_ABS"
	FailureMisread    FailureCode = "MISREAD"
	FailureEnvErr     FailureCode = "ENV_ERR"
	FailureScopeDrift FailureCode = "SCOPE_DRIFT"
	FailureTimeout    FailureCode = "TIMEOUT"
	FailureSilentSkip FailureCode = "SILENT_SKIP"
)

// EvidenceSpan is a verbatim quote from the transcript justifying a failure label.
type EvidenceSpan struct {
	Code      FailureCode `json:"code"`
	Quote     string      `json:"quote"`
	TurnIndex int         `json:"turn_index"`
}

// FailureClassification is the structured-output verdict from the failure
// classifier (an LLM call with Pydantic-typed return).
//
// Multi-label: Primary is the dominant failure, Secondary captures up to 3
// co-occurring failures. NONE is mutually exclusive with all others — when
// Primary == NONE, Secondary must be empty (validator enforced grader-side).
type FailureClassification struct {
	Primary    FailureCode    `json:"primary"`
	Secondary  []FailureCode  `json:"secondary,omitempty"`
	Evidence   []EvidenceSpan `json:"evidence,omitempty"`
	Confidence float64        `json:"confidence"`
	Rationale  string         `json:"rationale,omitempty"`
}
