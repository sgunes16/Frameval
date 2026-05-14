package support_test

import (
	"context"
	"testing"
	"time"

	graderpb "github.com/mustafaselman/frameval/engine/proto"
	"github.com/mustafaselman/frameval/engine/test/support"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// dialFake is a thin shim that mirrors how production code dials the grader:
// `grpc.NewClient(addr, insecure)`. Tests use it to assert the FakeGrader
// is reachable via the public address it advertises.
func dialFake(t *testing.T, addr string) graderpb.GraderServiceClient {
	t.Helper()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial fake grader: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return graderpb.NewGraderServiceClient(conn)
}

func TestFakeGrader_HealthCheckReturnsHealthy(t *testing.T) {
	addr := support.StartFakeGrader(t, support.FakeGraderConfig{})

	client := dialFake(t, addr)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := client.HealthCheck(ctx, &graderpb.Empty{})
	if err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
	if !resp.Healthy {
		t.Error("default fake grader should report healthy")
	}
}

func TestFakeGrader_GradeRunReturnsConfiguredComposite(t *testing.T) {
	addr := support.StartFakeGrader(t, support.FakeGraderConfig{
		CompositeScore: 7.5,
		TestPassRate:   0.8,
	})

	client := dialFake(t, addr)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := client.GradeRun(ctx, &graderpb.GradeRunRequest{RunId: "r1"})
	if err != nil {
		t.Fatalf("GradeRun: %v", err)
	}
	if resp.CompositeScore != 7.5 {
		t.Errorf("CompositeScore: want 7.5, got %v", resp.CompositeScore)
	}
	if resp.Code == nil || resp.Code.TestPassRate != 0.8 {
		t.Errorf("Code.TestPassRate: want 0.8, got %+v", resp.Code)
	}
}

func TestFakeGrader_ClassifyFailureReturnsConfiguredVerdict(t *testing.T) {
	addr := support.StartFakeGrader(t, support.FakeGraderConfig{
		FailurePrimary:    "HAL_API",
		FailureRationale:  "agent hallucinated an API method",
		FailureConfidence: 0.9,
	})

	client := dialFake(t, addr)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := client.ClassifyFailure(ctx, &graderpb.ClassifyFailureRequest{RunId: "r1"})
	if err != nil {
		t.Fatalf("ClassifyFailure: %v", err)
	}
	if resp.Classification == nil {
		t.Fatal("nil Classification")
	}
	if resp.Classification.Primary != "HAL_API" {
		t.Errorf("Primary: want HAL_API, got %q", resp.Classification.Primary)
	}
	if resp.Classification.Confidence != 0.9 {
		t.Errorf("Confidence: want 0.9, got %v", resp.Classification.Confidence)
	}
}

func TestFakeGrader_StartReturnsAddressableLoopback(t *testing.T) {
	addr := support.StartFakeGrader(t, support.FakeGraderConfig{})
	if addr == "" {
		t.Fatal("addr empty")
	}
	// Ephemeral port — must include a port separator.
	if addr[len(addr)-1] == ':' || len(addr) < 5 {
		t.Errorf("addr looks malformed: %q", addr)
	}
}
