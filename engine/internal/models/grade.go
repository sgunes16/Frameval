package models

type Grade struct {
	ID                        string              `json:"id"`
	RunID                     string              `json:"run_id,omitempty"`
	TestPassRate              float64             `json:"test_pass_rate"`
	TestPassCount             int                 `json:"test_pass_count"`
	TestFailCount             int                 `json:"test_fail_count"`
	LintScore                 float64             `json:"lint_score"`
	TypeCheckPass             bool                `json:"type_check_pass"`
	FileStateValid            bool                `json:"file_state_valid"`
	TurnCount                 int                 `json:"turn_count"`
	TotalTokens               int                 `json:"total_tokens"`
	CostUSD                   float64             `json:"cost_usd"`
	TokenEfficiency           float64             `json:"token_efficiency"`
	BacktrackCount            int                 `json:"backtrack_count"`
	SelfValidationRate        float64             `json:"self_validation_rate"`
	PrematureCompletion       bool                `json:"premature_completion"`
	IdleTurns                 int                 `json:"idle_turns"`
	ErrorRecoveryCount        int                 `json:"error_recovery_count"`
	ToolCallAccuracy          float64             `json:"tool_call_accuracy"`
	ContextUtilization        float64             `json:"context_utilization"`
	JudgeCorrectness          float64             `json:"judge_correctness"`
	JudgeMaintainability      float64             `json:"judge_maintainability"`
	JudgeCompleteness         float64             `json:"judge_completeness"`
	JudgeBestPractices        float64             `json:"judge_best_practices"`
	JudgeErrorHandling        float64             `json:"judge_error_handling"`
	JudgeIRRAlpha             float64             `json:"judge_irr_alpha"`
	RawJudgeResponses         []string            `json:"raw_judge_responses,omitempty"`
	SpecInstructionCompliance float64             `json:"spec_instruction_compliance"`
	SpecConstraintViolations  int                 `json:"spec_constraint_violations"`
	SpecConventionAdherence   float64             `json:"spec_convention_adherence"`
	SpecPerInstruction        []InstructionResult `json:"spec_per_instruction,omitempty"`
	CompositeScore            float64             `json:"composite_score"`
	GradedAt                  string              `json:"graded_at"`
	TestResults               []TestResult        `json:"test_results,omitempty"`
}

type TestResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Output string `json:"output"`
}

type InstructionResult struct {
	Instruction string `json:"instruction"`
	Status      string `json:"status"`
	Reasoning   string `json:"reasoning"`
}
