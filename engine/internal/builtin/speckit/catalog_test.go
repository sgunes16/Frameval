package speckit

import ("strings"; "testing")

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

func TestCatalogUsesOnlyRealCommands(t *testing.T) {
	known := map[string]bool{"specify": true, "clarify": true, "plan": true, "tasks": true,
		"analyze": true, "checklist": true, "constitution": true, "implement": true, "taskstoissues": true}
	prefixes := []string{"tinyspec", "brownfield", "red-team", "conduct", "v-model"}
	for _, ext := range List() {
		for _, s := range ext.Stages {
			cmd := strings.TrimPrefix(s.SlashCommand, "/speckit.")
			ok := known[cmd]
			for _, p := range prefixes {
				if strings.HasPrefix(cmd, p) { ok = true }
			}
			if !ok {
				t.Errorf("extension %q stage %q uses unknown command %q", ext.ID, s.Name, s.SlashCommand)
			}
		}
		if ext.ID != "canonical" && (ext.ExtensionName == "" || ext.InstallURL == "") {
			t.Errorf("extension %q must set ExtensionName and InstallURL", ext.ID)
		}
	}
}
