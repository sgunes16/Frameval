// Package speckit holds the curated catalog of spec-kit extensions the
// launcher exposes to users. Each extension is a small ordered list of
// stages with prompt templates; the harness walks them in sequence at
// invocation time. The dual-role entry tags its stages with role names
// so Project 2's Inspector role accent fires for those runs.
package speckit

import "sort"

type Stage struct {
	Name           string // stable id used in transcripts ("specify", "plan", ...)
	SlashCommand   string // "/speckit.specify"
	PromptTemplate string // text with {{TASK}} / {{TECHNICAL_DETAILS}} substitutions
	Role           string // optional; non-empty only for dual-role
}

type SpecKitExtension struct {
	ID          string
	Name        string
	Description string
	Stages      []Stage
	MultiAgent  bool
	SourceURL   string
	// ExtensionName is the `specify extension add <name>` id; empty for core-only (canonical).
	ExtensionName string
	// InstallURL is the `--from` release/zip URL passed to `specify extension add`; empty for core-only.
	InstallURL string
}

var entries = []SpecKitExtension{
	{
		ID:          "canonical",
		Name:        "Canonical (4-stage)",
		Description: "specify → plan → tasks → implement; the upstream spec-kit baseline.",
		Stages: []Stage{
			{Name: "specify", SlashCommand: "/speckit.specify", PromptTemplate: "/speckit.specify\n\n{{TASK}}"},
			{Name: "clarify", SlashCommand: "/speckit.clarify", PromptTemplate: "/speckit.clarify"},
			{Name: "plan", SlashCommand: "/speckit.plan", PromptTemplate: "/speckit.plan\n\n{{TECHNICAL_DETAILS}}"},
			{Name: "tasks", SlashCommand: "/speckit.tasks", PromptTemplate: "/speckit.tasks"},
			{Name: "analyze", SlashCommand: "/speckit.analyze", PromptTemplate: "/speckit.analyze"},
			{Name: "implement", SlashCommand: "/speckit.implement", PromptTemplate: "/speckit.implement"},
		},
		SourceURL: "https://github.github.io/spec-kit/",
	},
	// Only the canonical core flow is exposed. The community-extension variants
	// (lite/tdd-first/research-first/rigorous/dual-role) were removed: they
	// require `specify extension add --from <url>` (network) and their
	// greenfield/script-heavy commands misfit Frameval's single-file brownfield
	// tasks (red-team treated as a URL, conduct stalls, v-model gates, etc.).
}

// List returns every catalog entry. Canonical is always first; remaining
// entries follow alphabetical id order so the picker UI is deterministic.
func List() []SpecKitExtension {
	out := make([]SpecKitExtension, len(entries))
	copy(out, entries)
	sort.SliceStable(out[1:], func(i, j int) bool {
		return out[1+i].ID < out[1+j].ID
	})
	return out
}

// Lookup returns the entry matching id, or (zero, false) if none.
// Empty id is treated as unknown.
func Lookup(id string) (SpecKitExtension, bool) {
	if id == "" {
		return SpecKitExtension{}, false
	}
	for _, e := range entries {
		if e.ID == id {
			return e, true
		}
	}
	return SpecKitExtension{}, false
}
