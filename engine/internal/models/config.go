package models

type APIKey struct {
	ID           string `json:"id"`
	Provider     string `json:"provider"`
	EncryptedKey string `json:"-"`
	RedactedKey  string `json:"redacted_key,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}

type ModelConfig struct {
	ID                       string  `json:"id"`
	Provider                 string  `json:"provider"`
	ModelID                  string  `json:"model_id"`
	DisplayName              string  `json:"display_name"`
	InputPricePer1K          float64 `json:"input_price_per_1k"`
	OutputPricePer1K         float64 `json:"output_price_per_1k"`
	MaxContextTokens         int     `json:"max_context_tokens"`
	SupportsStructuredOutput bool    `json:"supports_structured_output"`
	SupportsSeed             bool    `json:"supports_seed"`
}

type AgentInfo struct {
	Name      string   `json:"name"`
	Modes     []string `json:"modes"`
	Available bool     `json:"available"`
}

type QueueStatus struct {
	Depth         int `json:"depth"`
	ActiveWorkers int `json:"active_workers"`
	MaxWorkers    int `json:"max_workers"`
}

type HealthStatus struct {
	Healthy bool   `json:"healthy"`
	Version string `json:"version"`
	Message string `json:"message,omitempty"`
}
