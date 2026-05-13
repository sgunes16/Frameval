package diagnostic

// Fingerprint is the 10-dimensional behavioral vector AgentDx computes from
// every transcript. All values are deterministic functions of the transcript
// alone — no LLM is involved.
//
// The full formulas are documented in
// docs/superpowers/specs/2026-05-12-agentdx-design.md Appendix B.
type Fingerprint struct {
	PlanningDepth        float64 `json:"planning_depth"`
	ToolCallDiversity    float64 `json:"tool_call_diversity"`
	SelfValidationRate   float64 `json:"self_validation_rate"`
	BacktrackRate        float64 `json:"backtrack_rate"`
	FileFocus            float64 `json:"file_focus"`
	RecoveryLatency      float64 `json:"recovery_latency"`
	PrematureCompletion  float64 `json:"premature_completion"`
	TurnEfficiency       float64 `json:"turn_efficiency"`
	ContextReferenceRate float64 `json:"context_reference_rate"`
	IdleThinkingRatio    float64 `json:"idle_thinking_ratio"`
}
