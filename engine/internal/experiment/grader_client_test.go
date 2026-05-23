package experiment

import (
	"context"
	"testing"

	"github.com/mustafaselman/frameval/engine/internal/models"
)

func TestNewGraderClient_EmptyAddrReturnsClientWithNilStub(t *testing.T) {
	c := NewGraderClient("", nil, nil)
	if c == nil {
		t.Fatal("NewGraderClient returned nil")
	}
	if c.client != nil {
		t.Error("empty addr should produce nil client stub (fallback path)")
	}
	if c.conn != nil {
		t.Error("empty addr should not allocate a *grpc.ClientConn")
	}
}

func TestGraderClient_CloseIsIdempotent(t *testing.T) {
	c := NewGraderClient("", nil, nil)
	if err := c.Close(); err != nil {
		t.Errorf("first Close on no-grader client: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("second Close on already-closed client: %v", err)
	}
}

func TestGraderClient_CloseOnNilReceiverIsSafe(t *testing.T) {
	var c *GraderClient
	if err := c.Close(); err != nil {
		t.Errorf("Close on nil *GraderClient: %v", err)
	}
}

func TestGraderClient_GradeRunNoGraderConfiguredReturnsFallbackSource(t *testing.T) {
	// Empty addr → c.client is nil → GradeRun must take the fallback
	// path and Source must record that fact so the regrade handler can
	// surface 503 instead of silently persisting placeholder data.
	c := NewGraderClient("", nil, nil)

	grade, err := c.GradeRun(context.Background(), models.Task{ID: "t1"}, nil, models.Transcript{RunID: "r1"})
	if err != nil {
		t.Fatalf("GradeRun unexpected error: %v", err)
	}
	if grade.Source != models.GradeSourceFallback {
		t.Errorf("expected fallback source, got %q", grade.Source)
	}
}
