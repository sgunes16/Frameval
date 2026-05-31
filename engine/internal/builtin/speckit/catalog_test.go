package speckit

import (
	"strings"
	"testing"
)

func TestListReturnsAllEntries(t *testing.T) {
	got := List()
	if len(got) != 6 {
		t.Fatalf("entry count: got %d want 6", len(got))
	}
	// Canonical must come first; rest alphabetical by ID.
	if got[0].ID != "canonical" {
		t.Errorf("first entry: got %q want %q", got[0].ID, "canonical")
	}
	wantIDs := []string{"canonical", "dual-role", "lite", "research-first", "rigorous", "tdd-first"}
	for i, w := range wantIDs {
		if got[i].ID != w {
			t.Errorf("entry %d: got %q want %q", i, got[i].ID, w)
		}
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
}

func TestCanonicalEntryPreservesOldStagePrompts(t *testing.T) {
	ext, ok := Lookup("canonical")
	if !ok {
		t.Fatal("canonical missing")
	}
	if len(ext.Stages) != 4 {
		t.Fatalf("stage count: got %d want 4", len(ext.Stages))
	}
	expect := []struct {
		name         string
		slashCommand string
		promptStart  string
	}{
		{"specify", "/speckit.specify", "/speckit.specify\n\n{{TASK}}"},
		{"plan", "/speckit.plan", "/speckit.plan\n\n{{TECHNICAL_DETAILS}}"},
		{"tasks", "/speckit.tasks", "/speckit.tasks"},
		{"implement", "/speckit.implement", "/speckit.implement"},
	}
	for i, want := range expect {
		st := ext.Stages[i]
		if st.Name != want.name {
			t.Errorf("stage %d name: got %q want %q", i, st.Name, want.name)
		}
		if st.SlashCommand != want.slashCommand {
			t.Errorf("stage %d slash: got %q want %q", i, st.SlashCommand, want.slashCommand)
		}
		if !strings.Contains(st.PromptTemplate, want.promptStart) {
			t.Errorf("stage %d prompt should contain %q; got %q", i, want.promptStart, st.PromptTemplate)
		}
		if st.Role != "" {
			t.Errorf("canonical stage %d should have empty role; got %q", i, st.Role)
		}
	}
}

func TestDualRoleEntryHasRoleTags(t *testing.T) {
	ext, ok := Lookup("dual-role")
	if !ok {
		t.Fatal("dual-role missing")
	}
	if !ext.MultiAgent {
		t.Error("dual-role should set MultiAgent=true")
	}
	wantRoles := []string{"architect", "architect", "coder", "coder"}
	if len(ext.Stages) != len(wantRoles) {
		t.Fatalf("stage count: got %d want %d", len(ext.Stages), len(wantRoles))
	}
	for i, want := range wantRoles {
		if ext.Stages[i].Role != want {
			t.Errorf("stage %d role: got %q want %q", i, ext.Stages[i].Role, want)
		}
	}
}
