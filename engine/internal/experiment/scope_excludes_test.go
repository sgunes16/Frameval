package experiment

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/mustafaselman/frameval/engine/test/support"
)

func TestHarnessExcludePathspecs(t *testing.T) {
	cases := []struct {
		harnessID string
		want      []string
	}{
		{"bare", nil},
		{"ralph", nil},
		{"multiagent", nil},
		{"agent_instructions", []string{":!CLAUDE.md"}},
		{"speckit", []string{
			":!.opencode", ":!.opencode/**",
			":!.specify", ":!.specify/**",
			":!specs", ":!specs/**",
			":!memory", ":!memory/**",
			":!AGENTS.md",
		}},
		{"unknown-future", nil},
		{"", nil},
	}
	for _, tc := range cases {
		t.Run(tc.harnessID, func(t *testing.T) {
			got := harnessExcludePathspecs(tc.harnessID)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestVerificationEnvironmentSetsHarnessExcludesEnv(t *testing.T) {
	env := verificationEnvironment("task-x", "speckit")
	got := env["FRAMEVAL_HARNESS_EXCLUDES"]
	if got == "" {
		t.Fatal("FRAMEVAL_HARNESS_EXCLUDES should be set for speckit")
	}
	for _, want := range []string{":!.specify", ":!specs"} {
		if !strings.Contains(got, want) {
			t.Errorf("env should contain %q; got %q", want, got)
		}
	}
}

func TestVerificationEnvironmentOmitsHarnessExcludesForBareHarness(t *testing.T) {
	env := verificationEnvironment("task-x", "bare")
	if v, ok := env["FRAMEVAL_HARNESS_EXCLUDES"]; ok && v != "" {
		t.Errorf("bare harness should not set FRAMEVAL_HARNESS_EXCLUDES; got %q", v)
	}
}

func TestSpeckitVersionOrDefault_FallsBackToHardcoded(t *testing.T) {
	store := support.TmpStore(t)
	ctx := context.Background()

	v := speckitVersionOrDefault(ctx, store)
	if v == "" {
		t.Fatal("expected a non-empty version")
	}
	// When nothing is configured the hard-coded default must be returned.
	if v != "v0.9.1" {
		t.Errorf("expected hard-coded default v0.9.1, got %q", v)
	}
}

func TestSpeckitVersionOrDefault_ReadsFromAppSettings(t *testing.T) {
	store := support.TmpStore(t)
	ctx := context.Background()

	if err := store.SetSetting(ctx, "speckit.version", "v1.2.3"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	v := speckitVersionOrDefault(ctx, store)
	if v != "v1.2.3" {
		t.Errorf("expected v1.2.3 from app_settings, got %q", v)
	}
}
