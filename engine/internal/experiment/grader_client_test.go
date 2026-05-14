package experiment

import (
	"testing"
)

func TestNewGraderClient_EmptyAddrReturnsClientWithNilStub(t *testing.T) {
	c := NewGraderClient("", nil)
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
	c := NewGraderClient("", nil)
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
