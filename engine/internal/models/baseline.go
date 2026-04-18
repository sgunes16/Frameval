package models

type Baseline struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	Source          string `json:"source"`
	ArtifactType    string `json:"artifact_type,omitempty"`
	ArtifactContent string `json:"artifact_content,omitempty"`
	TaskID          string `json:"task_id"`
	Model           string `json:"model"`
	AgentCLI        string `json:"agent_cli"`
	TotalRuns       int    `json:"total_runs"`
	EvaluatedAt     string `json:"evaluated_at"`
}
