package experiment

import (
	"reflect"
	"strings"
	"testing"
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
			":!.specify", ":!.specify/**",
			":!specs", ":!specs/**",
			":!memory", ":!memory/**",
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
