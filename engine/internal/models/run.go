package models

type Run struct {
	ID                         string      `json:"id"`
	ExperimentID               string      `json:"experiment_id"`
	VariantID                  string      `json:"variant_id"`
	RunNumber                  int         `json:"run_number"`
	Status                     string      `json:"status"`
	ContainerID                string      `json:"container_id,omitempty"`
	EnvironmentFingerprintJSON string      `json:"environment_fingerprint_json,omitempty"`
	StartedAt                  string      `json:"started_at,omitempty"`
	CompletedAt                string      `json:"completed_at,omitempty"`
	DurationSeconds            float64     `json:"duration_seconds,omitempty"`
	ErrorMessage               string      `json:"error_message,omitempty"`
	Transcript                 *Transcript `json:"transcript,omitempty"`
	Grade                      *Grade      `json:"grade,omitempty"`
}

type Transcript struct {
	ID             string       `json:"id"`
	RunID          string       `json:"run_id"`
	RawOutput      string       `json:"raw_output"`
	ParsedTurns    []ParsedTurn `json:"parsed_turns,omitempty"`
	FilesystemDiff string       `json:"filesystem_diff,omitempty"`
	TotalTurns     int          `json:"total_turns"`
	TotalTokens    int          `json:"total_tokens"`
	CostUSD        float64      `json:"cost_usd"`
	OutputFiles    []OutputFile `json:"output_files,omitempty"`
}

type ParsedTurn struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

type OutputFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}
