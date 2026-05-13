package models

type Experiment struct {
	ID                  string             `json:"id"`
	Name                string             `json:"name"`
	Description         string             `json:"description,omitempty"`
	Status              string             `json:"status"`
	TaskID              string             `json:"task_id"`
	WorkspaceSourceType string             `json:"workspace_source_type,omitempty"`
	LocalPath           string             `json:"local_path,omitempty"`
	GitURL              string             `json:"git_url,omitempty"`
	GitRef              string             `json:"git_ref,omitempty"`
	Model               string             `json:"model"`
	AgentCLI            string             `json:"agent_cli"`
	ExecutionMode       string             `json:"execution_mode"`
	RunsPerVariant      int                `json:"runs_per_variant"`
	Temperature         float64            `json:"temperature"`
	TimeoutSeconds      int                `json:"timeout_seconds"`
	MaxConcurrent       int                `json:"max_concurrent"`
	JudgeModel          string             `json:"judge_model,omitempty"`
	Seed                *int               `json:"seed,omitempty"`
	EstimatedCostUSD    *float64           `json:"estimated_cost_usd,omitempty"`
	ActualCostUSD       *float64           `json:"actual_cost_usd,omitempty"`
	CompositeWeights    map[string]float64 `json:"composite_weights"`
	CreatedAt           string             `json:"created_at"`
	StartedAt           string             `json:"started_at,omitempty"`
	CompletedAt         string             `json:"completed_at,omitempty"`
	Variants            []Variant          `json:"variants,omitempty"`
	Stats               []ExperimentStat   `json:"stats,omitempty"`
}

type Variant struct {
	ID               string            `json:"id"`
	ExperimentID     string            `json:"experiment_id"`
	Name             string            `json:"name"`
	Description      string            `json:"description,omitempty"`
	IsControl        bool              `json:"is_control"`
	Ordering         int               `json:"ordering"`
	ArtifactVersions []ArtifactVersion `json:"artifact_versions,omitempty"`
}

type ArtifactVersion struct {
	ID           string         `json:"id"`
	VariantID    string         `json:"variant_id"`
	ArtifactType string         `json:"artifact_type"`
	SourceKind   string         `json:"source_kind,omitempty"`
	DisplayName  string         `json:"display_name,omitempty"`
	SourceRef    string         `json:"source_ref,omitempty"`
	FilePath     string         `json:"file_path"`
	Content      string         `json:"content"`
	ContentHash  string         `json:"content_hash"`
	Dimensions   map[string]any `json:"dimensions,omitempty"`
	CreatedAt    string         `json:"created_at"`
}

type ExperimentRequest struct {
	Name                string             `json:"name"`
	Description         string             `json:"description"`
	TaskID              string             `json:"task_id"`
	WorkspaceSourceType string             `json:"workspace_source_type"`
	LocalPath           string             `json:"local_path"`
	GitURL              string             `json:"git_url"`
	GitRef              string             `json:"git_ref"`
	Model               string             `json:"model"`
	AgentCLI            string             `json:"agent_cli"`
	ExecutionMode       string             `json:"execution_mode"`
	RunsPerVariant      int                `json:"runs_per_variant"`
	Temperature         float64            `json:"temperature"`
	TimeoutSeconds      int                `json:"timeout_seconds"`
	MaxConcurrent       int                `json:"max_concurrent"`
	JudgeModel          string             `json:"judge_model"`
	Seed                *int               `json:"seed"`
	CompositeWeights    map[string]float64 `json:"composite_weights"`
	Variants            []VariantRequest   `json:"variants"`
}

type VariantRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsControl   bool   `json:"is_control"`
	Ordering    int    `json:"ordering"`
}

type ArtifactRequest struct {
	ArtifactType string `json:"artifact_type"`
	SourceKind   string `json:"source_kind"`
	DisplayName  string `json:"display_name"`
	SourceRef    string `json:"source_ref"`
	FilePath     string `json:"file_path"`
	Content      string `json:"content"`
}

type ArtifactDiff struct {
	From ArtifactVersion `json:"from"`
	To   ArtifactVersion `json:"to"`
	Diff string          `json:"diff"`
}
