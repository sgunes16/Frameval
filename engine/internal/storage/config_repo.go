package storage

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"
	"github.com/mustafaselman/frameval/engine/internal/models"
)

func (s *Store) SeedModelConfigs(ctx context.Context) error {
	defaults := []models.ModelConfig{
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

func (s *Store) ListAgents() []models.AgentInfo {
	return []models.AgentInfo{
		{Name: "cursor", Modes: []string{"cli"}, Available: true},
		{Name: "gemini", Modes: []string{"cli"}, Available: true},
		{Name: "api", Modes: []string{"api"}, Available: true},
	}
}
