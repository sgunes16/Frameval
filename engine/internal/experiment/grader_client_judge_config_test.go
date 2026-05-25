package experiment

import (
	"context"
	"testing"

	"github.com/mustafaselman/frameval/engine/test/support"
)

func TestBuildJudgeConfig_Disabled(t *testing.T) {
	store := support.TmpStore(t)
	ctx := context.Background()
	_ = store.SetSetting(ctx, "judge.enabled", "false")

	cfg := BuildJudgeConfigForTest(ctx, store)
	if cfg != nil {
		t.Errorf("expected nil config when disabled, got %+v", cfg)
	}
}

func TestBuildJudgeConfig_EnabledWithKey(t *testing.T) {
	store := support.TmpStore(t)
	ctx := context.Background()
	_ = store.SetSetting(ctx, "judge.enabled", "true")
	_ = store.SetSetting(ctx, "judge.provider", "openrouter")
	_ = store.SetSetting(ctx, "judge.model", "deepseek/test-model")
	_ = store.UpsertAPIKey(ctx, "openrouter", "sk-or-test")

	cfg := BuildJudgeConfigForTest(ctx, store)
	if cfg == nil {
		t.Fatal("expected non-nil config when enabled")
	}
	if cfg.Provider != "openrouter" {
		t.Errorf("Provider = %q, want openrouter", cfg.Provider)
	}
	if cfg.Model != "deepseek/test-model" {
		t.Errorf("Model = %q, want deepseek/test-model", cfg.Model)
	}
	if cfg.ApiKey != "sk-or-test" {
		t.Errorf("ApiKey = %q, want sk-or-test", cfg.ApiKey)
	}
}

func TestBuildJudgeConfig_EnabledMissingKey(t *testing.T) {
	store := support.TmpStore(t)
	ctx := context.Background()
	_ = store.SetSetting(ctx, "judge.enabled", "true")
	_ = store.SetSetting(ctx, "judge.provider", "ollama")
	_ = store.SetSetting(ctx, "judge.model", "qwen2.5-coder:32b")
	// no api_keys row for ollama — that's fine, ollama doesn't need a key

	cfg := BuildJudgeConfigForTest(ctx, store)
	if cfg == nil {
		t.Fatal("expected non-nil config when enabled even without key")
	}
	if cfg.ApiKey != "" {
		t.Errorf("ApiKey = %q, want empty string (grader falls back to env)", cfg.ApiKey)
	}
}

func TestBuildJudgeConfig_PopulatesRubrics(t *testing.T) {
	store := support.TmpStore(t)
	ctx := context.Background()
	_ = store.SetSetting(ctx, "judge.enabled", "true")
	_ = store.SetSetting(ctx, "judge.provider", "openrouter")
	_ = store.SetSetting(ctx, "judge.model", "x")

	cfg := BuildJudgeConfigForTest(ctx, store)
	if cfg == nil {
		t.Fatal("expected non-nil cfg")
	}
	if len(cfg.Rubrics) != 5 {
		t.Errorf("want 5 seeded rubrics, got %d", len(cfg.Rubrics))
	}
	found := false
	for _, r := range cfg.Rubrics {
		if r.Key == "correctness" && len(r.Prompt) > 50 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("correctness rubric missing or empty prompt")
	}
}
