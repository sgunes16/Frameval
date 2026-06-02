package storage

import (
	"context"
	"testing"

	"github.com/mustafaselman/frameval/engine/internal/models"
)

func TestSaveGradeAndGetGradeByRun_NewMetricsRoundTrip(t *testing.T) {
	store := newTestStore(t)
	seedRun(t, store, "run-grade-metrics-1")

	want := models.Grade{
		RunID:                 "run-grade-metrics-1",
		CompositeScore:        0.80,
		ToolCallCount:         3,
		ToolErrorRate:         0.33,
		RanValidation:         true,
		HarnessAdherenceScore: 0.75,
		HarnessAdherenceJSON:  `[{"name":"spec_before_impl","passed":false}]`,
	}

	if err := store.SaveGrade(context.Background(), want); err != nil {
		t.Fatalf("SaveGrade: %v", err)
	}

	got, err := store.GetGradeByRun(context.Background(), "run-grade-metrics-1")
	if err != nil {
		t.Fatalf("GetGradeByRun: %v", err)
	}

	if got.ToolCallCount != want.ToolCallCount {
		t.Errorf("ToolCallCount: got %d want %d", got.ToolCallCount, want.ToolCallCount)
	}
	if got.ToolErrorRate != want.ToolErrorRate {
		t.Errorf("ToolErrorRate: got %f want %f", got.ToolErrorRate, want.ToolErrorRate)
	}
	if got.RanValidation != want.RanValidation {
		t.Errorf("RanValidation: got %v want %v", got.RanValidation, want.RanValidation)
	}
	if got.HarnessAdherenceScore != want.HarnessAdherenceScore {
		t.Errorf("HarnessAdherenceScore: got %f want %f", got.HarnessAdherenceScore, want.HarnessAdherenceScore)
	}
	if got.HarnessAdherenceJSON != want.HarnessAdherenceJSON {
		t.Errorf("HarnessAdherenceJSON: got %q want %q", got.HarnessAdherenceJSON, want.HarnessAdherenceJSON)
	}
}

func TestSaveGradeAndGetGradeByRun_NullHarnessAdherenceJSON(t *testing.T) {
	store := newTestStore(t)
	seedRun(t, store, "run-grade-metrics-2")

	want := models.Grade{
		RunID:          "run-grade-metrics-2",
		CompositeScore: 0.50,
		// HarnessAdherenceJSON intentionally empty — should round-trip as empty string.
	}

	if err := store.SaveGrade(context.Background(), want); err != nil {
		t.Fatalf("SaveGrade: %v", err)
	}

	got, err := store.GetGradeByRun(context.Background(), "run-grade-metrics-2")
	if err != nil {
		t.Fatalf("GetGradeByRun: %v", err)
	}

	if got.HarnessAdherenceJSON != "" {
		t.Errorf("HarnessAdherenceJSON: expected empty string, got %q", got.HarnessAdherenceJSON)
	}
	if got.RanValidation {
		t.Errorf("RanValidation: expected false for zero-value grade")
	}
}
