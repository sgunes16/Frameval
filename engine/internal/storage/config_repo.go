package storage

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/mustafaselman/frameval/engine/internal/models"
)

// OpenCodeModelsRunner is the minimal sandbox interface SeedOpenCodeModels
// depends on. Defined here (not pulled from the sandbox package) so the
// storage layer stays leaf-level — *sandbox.Manager satisfies it
// implicitly. Callers in tests can pass a fake runner instead.
type OpenCodeModelsRunner interface {
	RunShell(ctx context.Context, workspace string, env map[string]string, script string) (string, error)
}

// SeedOpenCodeModels queries `opencode models` inside the sandbox image
// and upserts every opencode/* id into model_configs as provider="opencode".
// Best-effort: a missing Docker daemon, a slow image pull, or an
// unauthenticated opencode CLI return nil rather than failing startup —
// the dropdown still works off the static seed list and the user can
// pick a local Ollama model.
//
// Idempotent thanks to UpsertModelConfig's ON CONFLICT clause, so running
// this on every boot is safe.
func (s *Store) SeedOpenCodeModels(ctx context.Context, runner OpenCodeModelsRunner) error {
	if runner == nil {
		return nil
	}
	// runInDocker tars and copies the workspace dir into the container,
	// which fails when workspace is "" (empty path can't be tarred).
	// `opencode models` doesn't actually need any files, but we still
	// have to hand it a real (empty) directory.
	tmpDir, err := os.MkdirTemp("", "frameval-opencode-models-")
	if err != nil {
		slog.Warn("seed opencode models: mktemp failed", "err", err)
		return nil
	}
	defer os.RemoveAll(tmpDir)

	// Stderr is dropped so opencode's first-run sqlite-migration banner
	// doesn't pollute the parse.
	output, err := runner.RunShell(ctx, tmpDir, nil, "opencode models 2>/dev/null")
	if err != nil {
		slog.Warn("seed opencode models: RunShell failed", "err", err, "output_prefix", truncate(output, 200))
		return nil
	}
	inserted := 0
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "opencode/") {
			continue
		}
		cfg := models.ModelConfig{
			Provider:    "opencode",
			ModelID:     line,
			DisplayName: opencodeDisplayName(line),
		}
		if err := s.UpsertModelConfig(ctx, cfg); err != nil {
			return err
		}
		inserted++
	}
	slog.Info("seed opencode models", "count", inserted)
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// opencodeDisplayName turns "opencode/deepseek-v4-flash-free" into
// "Deepseek V4 Flash (opencode free)" for the dropdown. Plain title-
// casing is enough — the CLI already uses friendly names with `-free`
// suffixes that we surface explicitly.
func opencodeDisplayName(modelID string) string {
	suffix := strings.TrimPrefix(modelID, "opencode/")
	free := strings.HasSuffix(suffix, "-free")
	suffix = strings.TrimSuffix(suffix, "-free")
	pretty := strings.ReplaceAll(suffix, "-", " ")
	pretty = strings.Title(pretty) //nolint:staticcheck // titles for display only
	if free {
		return pretty + " (opencode free)"
	}
	return pretty + " (opencode)"
}

func (s *Store) SeedModelConfigs(ctx context.Context) error {
	defaults := []models.ModelConfig{
		// Local — Ollama via Aider's OpenAI-compatible adapter. Model IDs
		// MUST be `openai/<ollama-tag>` (Aider routes that prefix to
		// OLLAMA_BASE_URL transparently). These are the typical "already
		// pulled" set on a fresh macOS install; for tags not in this list
		// pull them with `ollama pull <tag>` and add a row via the API.
		{Provider: "ollama", ModelID: "openai/qwen2.5-coder:7b", DisplayName: "Qwen2.5 Coder 7B (Ollama)", InputPricePer1K: 0, OutputPricePer1K: 0, MaxContextTokens: 32768, SupportsStructuredOutput: false, SupportsSeed: false},
		{Provider: "ollama", ModelID: "openai/qwen2.5-coder:1.5b", DisplayName: "Qwen2.5 Coder 1.5B (Ollama)", InputPricePer1K: 0, OutputPricePer1K: 0, MaxContextTokens: 32768, SupportsStructuredOutput: false, SupportsSeed: false},
		{Provider: "ollama", ModelID: "openai/llama3.1:8b", DisplayName: "Llama 3.1 8B (Ollama)", InputPricePer1K: 0, OutputPricePer1K: 0, MaxContextTokens: 8192, SupportsStructuredOutput: false, SupportsSeed: false},
		{Provider: "ollama", ModelID: "openai/llama3.2:3b", DisplayName: "Llama 3.2 3B (Ollama)", InputPricePer1K: 0, OutputPricePer1K: 0, MaxContextTokens: 131072, SupportsStructuredOutput: false, SupportsSeed: false},
		{Provider: "ollama", ModelID: "openai/llama3.2:1b", DisplayName: "Llama 3.2 1B (Ollama)", InputPricePer1K: 0, OutputPricePer1K: 0, MaxContextTokens: 131072, SupportsStructuredOutput: false, SupportsSeed: false},
		{Provider: "ollama", ModelID: "openai/phi3:mini", DisplayName: "Phi-3 Mini (Ollama)", InputPricePer1K: 0, OutputPricePer1K: 0, MaxContextTokens: 4096, SupportsStructuredOutput: false, SupportsSeed: false},
		// Cursor — premium runs on Cursor's cloud agent.
		{Provider: "cursor", ModelID: "auto", DisplayName: "Cursor Auto", InputPricePer1K: 0, OutputPricePer1K: 0, MaxContextTokens: 0, SupportsStructuredOutput: false, SupportsSeed: false},
		{Provider: "cursor", ModelID: "premium", DisplayName: "Cursor Premium", InputPricePer1K: 0, OutputPricePer1K: 0, MaxContextTokens: 0, SupportsStructuredOutput: false, SupportsSeed: false},
		// Cloud — OpenAI / Anthropic / Google. Pricing is approximate.
		{Provider: "openai", ModelID: "gpt-5.4", DisplayName: "GPT-5.4", InputPricePer1K: 0.01, OutputPricePer1K: 0.03, MaxContextTokens: 256000, SupportsStructuredOutput: true, SupportsSeed: true},
		{Provider: "anthropic", ModelID: "claude-opus-4.7", DisplayName: "Claude Opus 4.7", InputPricePer1K: 0.015, OutputPricePer1K: 0.075, MaxContextTokens: 200000, SupportsStructuredOutput: false, SupportsSeed: false},
		{Provider: "anthropic", ModelID: "claude-sonnet-4.6", DisplayName: "Claude Sonnet 4.6", InputPricePer1K: 0.003, OutputPricePer1K: 0.015, MaxContextTokens: 200000, SupportsStructuredOutput: false, SupportsSeed: false},
		{Provider: "google", ModelID: "gemini-3.1-pro", DisplayName: "Gemini 3.1 Pro", InputPricePer1K: 0.0035, OutputPricePer1K: 0.0105, MaxContextTokens: 1000000, SupportsStructuredOutput: true, SupportsSeed: true},
	}
	for _, cfg := range defaults {
		if err := s.UpsertModelConfig(ctx, cfg); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) UpsertModelConfig(ctx context.Context, cfg models.ModelConfig) error {
	if cfg.ID == "" {
		cfg.ID = uuid.NewString()
	}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO model_configs (id, provider, model_id, display_name, input_price_per_1k, output_price_per_1k, max_context_tokens, supports_structured_output, supports_seed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(model_id) DO UPDATE SET
			provider = excluded.provider,
			display_name = excluded.display_name,
			input_price_per_1k = excluded.input_price_per_1k,
			output_price_per_1k = excluded.output_price_per_1k,
			max_context_tokens = excluded.max_context_tokens,
			supports_structured_output = excluded.supports_structured_output,
			supports_seed = excluded.supports_seed
	`, cfg.ID, cfg.Provider, cfg.ModelID, cfg.DisplayName, cfg.InputPricePer1K, cfg.OutputPricePer1K, cfg.MaxContextTokens, boolToInt(cfg.SupportsStructuredOutput), boolToInt(cfg.SupportsSeed))
	if err != nil {
		return fmt.Errorf("upsert model config: %w", err)
	}
	return nil
}

func (s *Store) ListModelConfigs(ctx context.Context) ([]models.ModelConfig, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, provider, model_id, display_name, input_price_per_1k, output_price_per_1k, max_context_tokens, supports_structured_output, supports_seed FROM model_configs ORDER BY provider, display_name`)
	if err != nil {
		return nil, fmt.Errorf("list model configs: %w", err)
	}
	defer rows.Close()
	configs := make([]models.ModelConfig, 0)
	for rows.Next() {
		var config models.ModelConfig
		var supportsStructured, supportsSeed int
		if err := rows.Scan(&config.ID, &config.Provider, &config.ModelID, &config.DisplayName, &config.InputPricePer1K, &config.OutputPricePer1K, &config.MaxContextTokens, &supportsStructured, &supportsSeed); err != nil {
			return nil, fmt.Errorf("scan model config: %w", err)
		}
		config.SupportsStructuredOutput = supportsStructured == 1
		config.SupportsSeed = supportsSeed == 1
		configs = append(configs, config)
	}
	return configs, rows.Err()
}

func (s *Store) UpsertAPIKey(ctx context.Context, provider string, apiKey string) error {
	encrypted, err := encryptKey(apiKey)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `
		INSERT INTO api_keys (id, provider, encrypted_key)
		VALUES (?, ?, ?)
		ON CONFLICT(provider) DO UPDATE SET encrypted_key = excluded.encrypted_key, updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
	`, uuid.NewString(), provider, encrypted)
	if err != nil {
		return fmt.Errorf("upsert api key: %w", err)
	}
	return nil
}

func (s *Store) DeleteAPIKey(ctx context.Context, provider string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM api_keys WHERE provider = ?`, provider)
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	return nil
}

func (s *Store) ListAPIKeys(ctx context.Context) ([]models.APIKey, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, provider, encrypted_key, created_at, updated_at FROM api_keys ORDER BY provider ASC`)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()
	keys := make([]models.APIKey, 0)
	for rows.Next() {
		var key models.APIKey
		if err := rows.Scan(&key.ID, &key.Provider, &key.EncryptedKey, &key.CreatedAt, &key.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		key.RedactedKey = redactKey(key.EncryptedKey)
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func encryptKey(value string) (string, error) {
	key := sha256.Sum256([]byte("frameval-local-dev-key"))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}
	nonce := key[:gcm.NonceSize()]
	ciphertext := gcm.Seal(nil, nonce, []byte(value), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func redactKey(value string) string {
	if len(value) <= 8 {
		return "****"
	}
	return value[:4] + "..." + value[len(value)-4:]
}

// GetDecryptedAPIKey returns the plaintext API key for provider, or
// sql.ErrNoRows when no row exists. The encrypted column was written
// by UpsertAPIKey via encryptKey; this is its inverse.
func (s *Store) GetDecryptedAPIKey(ctx context.Context, provider string) (string, error) {
	var encrypted string
	err := s.DB.QueryRowContext(ctx, `SELECT encrypted_key FROM api_keys WHERE provider = ?`, provider).Scan(&encrypted)
	if err != nil {
		return "", err
	}
	return decryptKey(encrypted)
}

func decryptKey(encrypted string) (string, error) {
	key := sha256.Sum256([]byte("frameval-local-dev-key"))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}
	nonce := key[:gcm.NonceSize()]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("gcm open: %w", err)
	}
	return string(plain), nil
}

func (s *Store) ListAgents() []models.AgentInfo {
	return []models.AgentInfo{
		{Name: "opencode", Modes: []string{"cli"}, Available: true},
		{Name: "aider", Modes: []string{"cli"}, Available: true},
		{Name: "cursor", Modes: []string{"cli"}, Available: true},
	}
}
