package experiment

import (
	"context"
	"testing"

	"github.com/mustafaselman/frameval/engine/test/support"
)

// TestJudgeEnabled covers the four required precedence cases for judgeEnabled.
//
// TmpStore runs all migrations, which seed judge.enabled=true and
// judge.model=deepseek/… by default. Tests that need a clean slate use a nil
// store (skips SQLite entirely) or explicitly override the seeded value.
func TestJudgeEnabled(t *testing.T) {
	ctx := context.Background()

	t.Run("default is false (nil store, no env)", func(t *testing.T) {
		t.Setenv("FRAMEVAL_ENABLE_LLM_JUDGE", "")
		// nil store → no SQLite row → env check → env empty → false
		if judgeEnabled(ctx, nil) {
			t.Error("expected false when nil store and env unset")
		}
	})

	t.Run("app_settings judge.enabled=true enables", func(t *testing.T) {
		t.Setenv("FRAMEVAL_ENABLE_LLM_JUDGE", "")
		store := support.TmpStore(t)
		// Migration seeds judge.enabled=true; just confirm the helper returns true.
		if !judgeEnabled(ctx, store) {
			t.Error("expected true when judge.enabled=true in app_settings (seeded by migration)")
		}
	})

	t.Run("env FRAMEVAL_ENABLE_LLM_JUDGE=true enables when no SQLite row", func(t *testing.T) {
		t.Setenv("FRAMEVAL_ENABLE_LLM_JUDGE", "true")
		// nil store → no SQLite row → env=true → true
		if !judgeEnabled(ctx, nil) {
			t.Error("expected true when env var set and no SQLite store")
		}
	})

	t.Run("app_setting=false overrides env=true (SQLite is authoritative)", func(t *testing.T) {
		t.Setenv("FRAMEVAL_ENABLE_LLM_JUDGE", "true")
		store := support.TmpStore(t)
		if err := store.SetSetting(ctx, "judge.enabled", "false"); err != nil {
			t.Fatalf("SetSetting: %v", err)
		}
		if judgeEnabled(ctx, store) {
			t.Error("expected false: app_settings=false must override env=true")
		}
	})
}

// TestResolveClassifierModel covers the three-tier fallback for the classifier model.
func TestResolveClassifierModel(t *testing.T) {
	ctx := context.Background()

	t.Run("hardcoded fallback when nil store and no env", func(t *testing.T) {
		t.Setenv("FRAMEVAL_LLM_MODEL", "")
		// nil store → no SQLite row → env empty → hardcoded default
		if m := resolveClassifierModel(ctx, nil); m != "claude-haiku-4-5" {
			t.Errorf("want claude-haiku-4-5, got %q", m)
		}
	})

	t.Run("env FRAMEVAL_LLM_MODEL used when nil store", func(t *testing.T) {
		t.Setenv("FRAMEVAL_LLM_MODEL", "gpt-4o-mini")
		// nil store → no SQLite row → env → gpt-4o-mini
		if m := resolveClassifierModel(ctx, nil); m != "gpt-4o-mini" {
			t.Errorf("want gpt-4o-mini, got %q", m)
		}
	})

	t.Run("app_settings judge.model overrides env", func(t *testing.T) {
		t.Setenv("FRAMEVAL_LLM_MODEL", "gpt-4o-mini")
		store := support.TmpStore(t)
		if err := store.SetSetting(ctx, "judge.model", "anthropic/claude-3-5-haiku"); err != nil {
			t.Fatalf("SetSetting: %v", err)
		}
		if m := resolveClassifierModel(ctx, store); m != "anthropic/claude-3-5-haiku" {
			t.Errorf("want anthropic/claude-3-5-haiku, got %q", m)
		}
	})
}
