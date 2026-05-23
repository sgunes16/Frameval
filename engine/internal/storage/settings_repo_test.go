package storage

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestSettings_GetSetRoundtrip(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.SetSetting(ctx, "judge.provider", "zai"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	got, err := store.GetSetting(ctx, "judge.provider")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if got != "zai" {
		t.Errorf("got %q, want %q", got, "zai")
	}
}

func TestSettings_GetMissingReturnsErrNoRows(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetSetting(ctx, "nonexistent.key")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestSettings_GetAllPrefixFilter(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Seeded values from migration 014: judge.provider, judge.model, judge.enabled
	got, err := store.GetSettingsByPrefix(ctx, "judge.")
	if err != nil {
		t.Fatalf("GetSettingsByPrefix: %v", err)
	}
	if got["judge.provider"] != "openrouter" {
		t.Errorf("judge.provider = %q, want openrouter", got["judge.provider"])
	}
	if got["judge.model"] != "deepseek/deepseek-chat-v3-0324:free" {
		t.Errorf("judge.model = %q, want deepseek/...", got["judge.model"])
	}
	if got["judge.enabled"] != "true" {
		t.Errorf("judge.enabled = %q, want true", got["judge.enabled"])
	}
}

func TestSettings_SetUpsertsExisting(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.SetSetting(ctx, "judge.provider", "openai"); err != nil {
		t.Fatalf("first SetSetting: %v", err)
	}
	if err := store.SetSetting(ctx, "judge.provider", "ollama"); err != nil {
		t.Fatalf("second SetSetting: %v", err)
	}
	got, err := store.GetSetting(ctx, "judge.provider")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if got != "ollama" {
		t.Errorf("got %q, want %q (upsert should replace)", got, "ollama")
	}
}
