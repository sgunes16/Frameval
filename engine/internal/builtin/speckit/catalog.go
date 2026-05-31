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
}

var entries = []SpecKitExtension{
	{
		ID:          "canonical",
		Name:        "Canonical (4-stage)",
		Description: "specify → plan → tasks → implement; the upstream spec-kit baseline.",
		Stages: []Stage{
			{Name: "specify", SlashCommand: "/speckit.specify", PromptTemplate: "/speckit.specify\n\n{{TASK}}"},
			{Name: "plan", SlashCommand: "/speckit.plan", PromptTemplate: "/speckit.plan\n\n{{TECHNICAL_DETAILS}}"},
			{Name: "tasks", SlashCommand: "/speckit.tasks", PromptTemplate: "/speckit.tasks"},
			{Name: "implement", SlashCommand: "/speckit.implement", PromptTemplate: "/speckit.implement"},
		},
		SourceURL: "https://github.github.io/spec-kit/",
	},
	{
		ID:          "lite",
		Name:        "Lite (2-stage)",
		Description: "specify → implement; the minimal-ceremony baseline.",
		Stages: []Stage{
			{Name: "specify", SlashCommand: "/speckit.specify", PromptTemplate: "/speckit.specify\n\n{{TASK}}"},
			{Name: "implement", SlashCommand: "/speckit.implement", PromptTemplate: "/speckit.implement"},
		},
	},
	{
		ID:          "tdd-first",
		Name:        "TDD-first",
		Description: "specify → tests → plan → implement → verify; write tests before the plan.",
		Stages: []Stage{
			{Name: "specify", SlashCommand: "/speckit.specify", PromptTemplate: "/speckit.specify\n\n{{TASK}}"},
			{Name: "tests", SlashCommand: "/speckit.tests", PromptTemplate: "/speckit.tests\n\nWrite failing tests that pin every requirement from the specify stage."},
			{Name: "plan", SlashCommand: "/speckit.plan", PromptTemplate: "/speckit.plan\n\n{{TECHNICAL_DETAILS}}"},
			{Name: "implement", SlashCommand: "/speckit.implement", PromptTemplate: "/speckit.implement"},
			{Name: "verify", SlashCommand: "/speckit.verify", PromptTemplate: "/speckit.verify\n\nRun the test suite and report any failures."},
		},
	},
	{
		ID:          "research-first",
		Name:        "Research-first",
		Description: "research → specify → plan → tasks → implement; gather context before specifying.",
		Stages: []Stage{
			{Name: "research", SlashCommand: "/speckit.research", PromptTemplate: "/speckit.research\n\nSurvey the codebase and external context relevant to:\n{{TASK}}"},
			{Name: "specify", SlashCommand: "/speckit.specify", PromptTemplate: "/speckit.specify\n\n{{TASK}}"},
			{Name: "plan", SlashCommand: "/speckit.plan", PromptTemplate: "/speckit.plan\n\n{{TECHNICAL_DETAILS}}"},
			{Name: "tasks", SlashCommand: "/speckit.tasks", PromptTemplate: "/speckit.tasks"},
			{Name: "implement", SlashCommand: "/speckit.implement", PromptTemplate: "/speckit.implement"},
		},
	},
	{
		ID:          "rigorous",
		Name:        "Rigorous review",
		Description: "specify → plan → tasks → implement → review; post-implement review pass.",
		Stages: []Stage{
			{Name: "specify", SlashCommand: "/speckit.specify", PromptTemplate: "/speckit.specify\n\n{{TASK}}"},
			{Name: "plan", SlashCommand: "/speckit.plan", PromptTemplate: "/speckit.plan\n\n{{TECHNICAL_DETAILS}}"},
			{Name: "tasks", SlashCommand: "/speckit.tasks", PromptTemplate: "/speckit.tasks"},
			{Name: "implement", SlashCommand: "/speckit.implement", PromptTemplate: "/speckit.implement"},
			{Name: "review", SlashCommand: "/speckit.review", PromptTemplate: "/speckit.review\n\nReview the implementation against the spec; flag deviations and unhandled cases."},
		},
	},
	{
		ID:          "dual-role",
		Name:        "Dual-role (multi-agent)",
		Description: "Architect (specify, plan) hands off to coder (tasks, implement); role-tagged transcript.",
		MultiAgent:  true,
		Stages: []Stage{
			{Name: "specify", SlashCommand: "/speckit.specify", PromptTemplate: "/speckit.specify\n\n{{TASK}}", Role: "architect"},
			{Name: "plan", SlashCommand: "/speckit.plan", PromptTemplate: "/speckit.plan\n\n{{TECHNICAL_DETAILS}}", Role: "architect"},
			{Name: "tasks", SlashCommand: "/speckit.tasks", PromptTemplate: "/speckit.tasks", Role: "coder"},
			{Name: "implement", SlashCommand: "/speckit.implement", PromptTemplate: "/speckit.implement", Role: "coder"},
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
