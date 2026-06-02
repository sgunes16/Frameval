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
	{
		ID:            "lite",
		Name:          "Lite (2-stage)",
		Description:   "tinyspec → implement; minimal-ceremony via the tinyspec extension.",
		ExtensionName: "tinyspec",
		InstallURL:    "https://github.com/Quratulain-bilal/spec-kit-tinyspec/archive/0d0e84cc2586.zip",
		Stages: []Stage{
			{Name: "tinyspec", SlashCommand: "/speckit.tinyspec", PromptTemplate: "/speckit.tinyspec\n\n{{TASK}}"},
			{Name: "tinyspec-implement", SlashCommand: "/speckit.tinyspec.implement", PromptTemplate: "/speckit.tinyspec.implement"},
		},
	},
	{
		ID:            "tdd-first",
		Name:          "TDD-first",
		Description:   "v-model requirements → acceptance → plan → tasks → implement; write tests before the plan.",
		ExtensionName: "v-model",
		InstallURL:    "https://github.com/leocamello/spec-kit-v-model/archive/refs/tags/v0.7.2.zip",
		Stages: []Stage{
			{Name: "v-model-requirements", SlashCommand: "/speckit.v-model.requirements", PromptTemplate: "/speckit.v-model.requirements\n\n{{TASK}}"},
			{Name: "v-model-acceptance", SlashCommand: "/speckit.v-model.acceptance", PromptTemplate: "/speckit.v-model.acceptance"},
			{Name: "plan", SlashCommand: "/speckit.plan", PromptTemplate: "/speckit.plan\n\n{{TECHNICAL_DETAILS}}"},
			{Name: "tasks", SlashCommand: "/speckit.tasks", PromptTemplate: "/speckit.tasks"},
			{Name: "implement", SlashCommand: "/speckit.implement", PromptTemplate: "/speckit.implement"},
		},
	},
	{
		ID:            "research-first",
		Name:          "Research-first",
		Description:   "brownfield scan → specify → plan → tasks → implement; gather codebase context before specifying.",
		ExtensionName: "brownfield",
		InstallURL:    "https://github.com/Quratulain-bilal/spec-kit-brownfield/archive/7479c5206a57.zip",
		Stages: []Stage{
			{Name: "brownfield-scan", SlashCommand: "/speckit.brownfield.scan", PromptTemplate: "/speckit.brownfield.scan"},
			{Name: "specify", SlashCommand: "/speckit.specify", PromptTemplate: "/speckit.specify\n\n{{TASK}}"},
			{Name: "plan", SlashCommand: "/speckit.plan", PromptTemplate: "/speckit.plan\n\n{{TECHNICAL_DETAILS}}"},
			{Name: "tasks", SlashCommand: "/speckit.tasks", PromptTemplate: "/speckit.tasks"},
			{Name: "implement", SlashCommand: "/speckit.implement", PromptTemplate: "/speckit.implement"},
		},
	},
	{
		ID:            "rigorous",
		Name:          "Rigorous review",
		Description:   "specify → clarify → red-team → plan → tasks → analyze → implement; adversarial review before implementation.",
		ExtensionName: "red-team",
		InstallURL:    "https://github.com/ashbrener/spec-kit-red-team/archive/refs/tags/v1.0.2.zip",
		Stages: []Stage{
			{Name: "specify", SlashCommand: "/speckit.specify", PromptTemplate: "/speckit.specify\n\n{{TASK}}"},
			{Name: "clarify", SlashCommand: "/speckit.clarify", PromptTemplate: "/speckit.clarify"},
			{Name: "red-team-run", SlashCommand: "/speckit.red-team.run", PromptTemplate: "/speckit.red-team.run"},
			{Name: "plan", SlashCommand: "/speckit.plan", PromptTemplate: "/speckit.plan\n\n{{TECHNICAL_DETAILS}}"},
			{Name: "tasks", SlashCommand: "/speckit.tasks", PromptTemplate: "/speckit.tasks"},
			{Name: "analyze", SlashCommand: "/speckit.analyze", PromptTemplate: "/speckit.analyze"},
			{Name: "implement", SlashCommand: "/speckit.implement", PromptTemplate: "/speckit.implement"},
		},
	},
	{
		ID:            "dual-role",
		Name:          "Dual-role (multi-agent)",
		Description:   "Architect (specify, plan) hands off to coder (tasks, implement); role-tagged transcript via the conduct extension.",
		MultiAgent:    true,
		ExtensionName: "conduct",
		InstallURL:    "https://github.com/twbrandon7/spec-kit-conduct-ext/archive/refs/tags/v1.0.1.zip",
		Stages: []Stage{
			{Name: "conduct-specify", SlashCommand: "/speckit.conduct.run", PromptTemplate: "/speckit.conduct.run specify\n\n{{TASK}}", Role: "architect"},
			{Name: "conduct-plan", SlashCommand: "/speckit.conduct.run", PromptTemplate: "/speckit.conduct.run plan\n\n{{TECHNICAL_DETAILS}}", Role: "architect"},
			{Name: "conduct-tasks", SlashCommand: "/speckit.conduct.run", PromptTemplate: "/speckit.conduct.run tasks", Role: "coder"},
			{Name: "conduct-implement", SlashCommand: "/speckit.conduct.run", PromptTemplate: "/speckit.conduct.run implement", Role: "coder"},
		},
	},
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
