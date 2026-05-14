package support

import (
	"context"
	"net"
	"testing"

	graderpb "github.com/mustafaselman/frameval/engine/proto"
	"google.golang.org/grpc"
)

// FakeGraderConfig controls the canned responses the FakeGrader sends.
// Zero values produce a healthy grader that returns an empty grade with
// CompositeScore=0 — sufficient for tests that only need the grader to
// be reachable.
type FakeGraderConfig struct {
	// HealthCheck
	Healthy bool

	// GradeRun
	CompositeScore float64
	TestPassRate   float64

	// ClassifyFailure
	FailurePrimary    string
	FailureRationale  string
	FailureConfidence float64
}

// StartFakeGrader brings up an in-process gRPC server bound to an ephemeral
// localhost port and returns its address. The server is torn down via
// t.Cleanup so callers don't have to manage lifecycle.
//
// Use the returned address with grpc.NewClient(addr, ...) — the production
// grader_client follows the same wire pattern, so wiring the engine to point
// at the fake is a one-line swap.
func StartFakeGrader(t *testing.T, cfg FakeGraderConfig) string {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen ephemeral port: %v", err)
	}

	srv := grpc.NewServer()
	graderpb.RegisterGraderServiceServer(srv, &fakeGraderServer{cfg: applyFakeGraderDefaults(cfg)})

	done := make(chan struct{})
	go func() {
		_ = srv.Serve(lis)
		close(done)
	}()

	t.Cleanup(func() {
		srv.GracefulStop()
		<-done
	})

	return lis.Addr().String()
}

func applyFakeGraderDefaults(cfg FakeGraderConfig) FakeGraderConfig {
	// Healthy defaults to true — most tests want a working grader.
	if !cfg.Healthy {
		cfg.Healthy = true
	}
	return cfg
}

type fakeGraderServer struct {
	graderpb.UnimplementedGraderServiceServer
	cfg FakeGraderConfig
}

func (s *fakeGraderServer) HealthCheck(_ context.Context, _ *graderpb.Empty) (*graderpb.HealthResponse, error) {
	return &graderpb.HealthResponse{Healthy: s.cfg.Healthy, Version: "fake"}, nil
}

func (s *fakeGraderServer) GradeRun(_ context.Context, _ *graderpb.GradeRunRequest) (*graderpb.GradeRunResponse, error) {
	return &graderpb.GradeRunResponse{
		Code: &graderpb.CodeGradeResult{
			TestPassRate:   float32(s.cfg.TestPassRate),
			TypeCheckPass:  true,
			FileStateValid: true,
		},
		Process:        &graderpb.ProcessGradeResult{},
		Judge:          &graderpb.JudgeGradeResult{},
		Adherence:      &graderpb.SpecAdherenceResult{},
		CompositeScore: float32(s.cfg.CompositeScore),
	}, nil
}

func (s *fakeGraderServer) ClassifyFailure(_ context.Context, _ *graderpb.ClassifyFailureRequest) (*graderpb.ClassifyFailureResponse, error) {
	primary := s.cfg.FailurePrimary
	if primary == "" {
		primary = "NONE"
	}
	return &graderpb.ClassifyFailureResponse{
		Classification: &graderpb.FailureClassificationProto{
			Primary:    primary,
			Confidence: s.cfg.FailureConfidence,
			Rationale:  s.cfg.FailureRationale,
		},
	}, nil
}

func (s *fakeGraderServer) ComputeStats(_ context.Context, _ *graderpb.ComputeStatsRequest) (*graderpb.ComputeStatsResponse, error) {
	return &graderpb.ComputeStatsResponse{}, nil
}

func (s *fakeGraderServer) ClassifyDimensions(_ context.Context, _ *graderpb.ClassifyDimensionsRequest) (*graderpb.ClassifyDimensionsResponse, error) {
	return &graderpb.ClassifyDimensionsResponse{}, nil
}
