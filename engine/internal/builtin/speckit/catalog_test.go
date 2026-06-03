package speckit

import (
	"strings"
	"testing"
)

func TestListReturnsOnlyCanonical(t *testing.T) {
	got := List()
	// Only the canonical core flow is exposed; the community-extension
	// variants were removed (network installs + brownfield misfit).
	if len(got) != 1 || got[0].ID != "canonical" {
		first := ""
		if len(got) > 0 {
			first = got[0].ID
		}
		t.Fatalf("want exactly [canonical], got %d entries (first=%q)", len(got), first)
	}
}

func TestLookupKnownAndUnknown(t *testing.T) {
	ext, ok := Lookup("canonical")
	if !ok || ext.ID != "canonical" {
		t.Errorf("known: ok=%v id=%q", ok, ext.ID)
	}
	if _, ok := Lookup("nope"); ok {
		t.Error("unknown should return ok=false")
	}
	if _, ok := Lookup(""); ok {
		t.Error("empty should return ok=false")
	}
	// Removed variants must no longer resolve.
	if _, ok := Lookup("dual-role"); ok {
		t.Error("dual-role should be gone")
	}
}

func TestCatalogUsesOnlyRealCommands(t *testing.T) {
	known := map[string]bool{"specify": true, "clarify": true, "plan": true, "tasks": true,
		"analyze": true, "checklist": true, "constitution": true, "implement": true, "taskstoissues": true}
	for _, ext := range List() {
		for _, s := range ext.Stages {
			cmd := strings.TrimPrefix(s.SlashCommand, "/speckit.")
			if !known[cmd] {
				t.Errorf("extension %q stage %q uses unknown command %q", ext.ID, s.Name, s.SlashCommand)
			}
		}
	}
}
