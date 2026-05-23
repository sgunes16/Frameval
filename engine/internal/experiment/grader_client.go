package experiment

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"strings"
	"sync/atomic"
	"time"

	"github.com/sony/gobreaker/v2"

	"github.com/mustafaselman/frameval/engine/internal/models"
	"github.com/mustafaselman/frameval/engine/pkg/diagnostic"
	graderpb "github.com/mustafaselman/frameval/engine/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GraderClient holds a single gRPC connection to the Python grader and reuses
// it across all RPC calls. Constructed once at engine startup, closed on
// graceful shutdown. Opening a fresh *grpc.ClientConn per RPC (the prior
// design) wastes TCP handshakes and leaks per-connection balance-watcher
// goroutines under sustained load.
//
// A nil client field is the "no grader configured" sentinel — every RPC
// short-circuits to its fallback result. This preserves the original API
// contract that callers can construct a GraderClient unconditionally,
// whether the addr is empty, unreachable, or wrong-formatted.
type GraderClient struct {
	conn     *grpc.ClientConn
	client   graderpb.GraderServiceClient
	breaker  *gobreaker.CircuitBreaker[any]
	logger   *slog.Logger
	settings SettingsStore
	// Logged-once latch for the grader-unavailable warning. Without
	// it the orchestrator floods the log with one WARN line per run
	// while the grader is offline (the common dev case: nobody
	// started the Python sidecar). Once tripped, subsequent calls
	// fall back silently and rely on the original WARN being
	// visible above in the log.
	graderDownLogged atomic.Bool
}

// SettingsStore is the minimal store interface buildJudgeConfig needs.
// Defined here so tests can pass a fake. *storage.Store satisfies it.
type SettingsStore interface {
	GetSettingsByPrefix(ctx context.Context, prefix string) (map[string]string, error)
	GetDecryptedAPIKey(ctx context.Context, provider string) (string, error)
}

// NewGraderClient dials the grader at addr and stores the resulting
// connection + client stub. Errors during dial are logged on the supplied
// logger (or slog.Default() when nil) and the resulting client falls back
// to "no grader configured" behavior, so the engine still starts even if
// the grader is unreachable at startup time. grpc.NewClient is lazy — the
// underlying TCP connection is not established until the first RPC call.
func NewGraderClient(addr string, logger *slog.Logger, settings SettingsStore) *GraderClient {
	if logger == nil {
		logger = slog.Default()
	}
	if addr == "" {
		return &GraderClient{logger: logger, breaker: newGraderBreaker(), settings: settings}
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Warn("grader dial failed (will fall back to local grading)", "err", err, "addr", addr)
		return &GraderClient{logger: logger, breaker: newGraderBreaker(), settings: settings}
	}
	return &GraderClient{
		conn:     conn,
		client:   graderpb.NewGraderServiceClient(conn),
		breaker:  newGraderBreaker(),
		logger:   logger,
		settings: settings,
	}
}

// Close releases the gRPC connection. Safe on a client that never dialed
// (no-op) and idempotent — calling it twice does not double-close. Should
// be invoked from the engine's graceful-shutdown path.
func (c *GraderClient) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	conn := c.conn
	c.conn = nil
	c.client = nil
	return conn.Close()
}

// classifyFailureTimeout caps how long we wait for the LLM classifier to
// return before logging a warning and persisting the run without a failure
// label. The classifier is expected to return in ~3-5s on Haiku; the cap
// is generous to absorb Anthropic API hiccups without losing diagnostic.
const classifyFailureTimeout = 30 * time.Second

// ClassifyFailureResult bundles the verdict + transport latency the grader
// reports back. The orchestrator persists both into the `diagnostic` row.
type ClassifyFailureResult struct {
	Classification  diagnostic.FailureClassification
	ClassifierModel string
	LatencyMs       int32
}

// ClassifyFailure calls the Python grader's failure classifier RPC.
//
// Never returns an error to callers — on any RPC failure (no grader configured,
// network error, timeout, marshalling issue) it returns the `unclassified`
// sentinel (primary=NONE, confidence=0) so the orchestrator can still persist
// a diagnostic row without a failure_label. Callers detect the
// "classifier-was-unavailable" case by checking
// `result.Classification.Confidence == 0`.
func (c *GraderClient) ClassifyFailure(
	ctx context.Context,
	runID string,
	symptoms diagnostic.Symptoms,
	taskDescription string,
	transcriptTail string,
	classifierModel string,
) ClassifyFailureResult {
	fallback := ClassifyFailureResult{
		Classification:  diagnostic.FailureClassification{Primary: diagnostic.FailureNone},
		ClassifierModel: classifierModel,
	}
	if c.client == nil {
		return fallback
	}
	symptomsJSON, err := json.Marshal(symptoms)
	if err != nil {
		return fallback
	}
	ctx, cancel := context.WithTimeout(ctx, classifyFailureTimeout)
	defer cancel()
	req := &graderpb.ClassifyFailureRequest{
		RunId:           runID,
		SymptomsJson:    symptomsJSON,
		TaskDescription: taskDescription,
		TranscriptTail:  transcriptTail,
		ClassifierModel: classifierModel,
	}
	response, err := breakerExec(c.breaker, func() (*graderpb.ClassifyFailureResponse, error) {
		return retryGrader(ctx, defaultGraderRetry, func(ctx context.Context) (*graderpb.ClassifyFailureResponse, error) {
			return c.client.ClassifyFailure(ctx, req)
		})
	})
	if err != nil || response == nil || response.Classification == nil {
		if errors.Is(err, ErrGraderUnavailable) {
			c.logger.Warn("classify_failure: breaker open, returning unclassified", "run_id", runID, "err", err)
		}
		return fallback
	}
	return ClassifyFailureResult{
		Classification:  failureClassificationFromProto(response.Classification),
		ClassifierModel: classifierModel,
		LatencyMs:       response.LatencyMs,
	}
}

func failureClassificationFromProto(p *graderpb.FailureClassificationProto) diagnostic.FailureClassification {
	if p == nil {
		return diagnostic.FailureClassification{Primary: diagnostic.FailureNone}
	}
	secondary := make([]diagnostic.FailureCode, 0, len(p.Secondary))
	for _, s := range p.Secondary {
		secondary = append(secondary, diagnostic.FailureCode(s))
	}
	evidence := make([]diagnostic.EvidenceSpan, 0, len(p.Evidence))
	for _, e := range p.Evidence {
		if e == nil {
			continue
		}
		evidence = append(evidence, diagnostic.EvidenceSpan{
			Code:      diagnostic.FailureCode(e.Code),
			Quote:     e.Quote,
			TurnIndex: int(e.TurnIndex),
		})
	}
	return diagnostic.FailureClassification{
		Primary:    diagnostic.FailureCode(p.Primary),
		Secondary:  secondary,
		Evidence:   evidence,
		Confidence: p.Confidence,
		Rationale:  p.Rationale,
	}
}

// GradeRun calls the Python grader's GradeRun RPC. JudgeConfig is
// populated from SQLite settings via buildJudgeConfig; sending nil
// (proto-absent) tells the grader to use its own env defaults or return
// a disabled_judge_result. The returned grade always has Source set:
// GradeSourceGrader on a real round-trip, GradeSourceFallback on any
// error (no grader configured, breaker open, RPC failure, retry
// exhaustion). Callers that need to surface "grading actually happened"
// — e.g. the regrade HTTP handler — should branch on grade.Source
// rather than assuming a non-empty grade is real.
func (c *GraderClient) GradeRun(ctx context.Context, task models.Task, artifact *models.ArtifactVersion, transcript models.Transcript) (models.Grade, error) {
	if c.client == nil {
		return fallbackGrade(transcript), nil
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	request := &graderpb.GradeRunRequest{
		RunId:          transcript.RunID,
		TranscriptJson: transcript.RawOutput,
		FilesystemDiff: transcript.FilesystemDiff,
		Task:           &graderpb.TaskSpec{Id: task.ID, Prompt: task.TaskPrompt, CodebaseType: task.CodebaseType, SetupScript: task.SetupScript},
		JudgeConfig:    c.buildJudgeConfig(ctx),
	}
	for _, testCase := range task.TestCases {
		if strings.EqualFold(strings.TrimSpace(testCase.Visibility), "hidden") || strings.TrimSpace(testCase.SetupScript) != "" {
			continue
		}
		request.Task.TestCases = append(request.Task.TestCases, &graderpb.TestCase{Name: testCase.Name, Command: testCase.TestCommand, ExpectedResult: testCase.ExpectedResult})
	}
	for _, outputFile := range transcript.OutputFiles {
		request.OutputFiles = append(request.OutputFiles, &graderpb.OutputFile{Path: outputFile.Path, Content: []byte(outputFile.Content)})
	}
	if artifact != nil {
		request.Artifact = &graderpb.ArtifactSpec{Content: artifact.Content, Type: artifact.ArtifactType}
	}
	response, err := breakerExec(c.breaker, func() (*graderpb.GradeRunResponse, error) {
		return retryGrader(ctx, defaultGraderRetry, func(ctx context.Context) (*graderpb.GradeRunResponse, error) {
			return c.client.GradeRun(ctx, request)
		})
	})
	if err != nil {
		if errors.Is(err, ErrGraderUnavailable) {
			// Log loudly ONCE so operators see the grader is down;
			// subsequent runs fall back silently. The earlier per-
			// run WARN flooded the log when the grader sidecar was
			// off (the common dev case).
			if c.graderDownLogged.CompareAndSwap(false, true) {
				c.logger.Warn("grade_run: grader unavailable, returning fallback grade for this and subsequent runs until the grader returns", "run_id", transcript.RunID, "err", err)
			}
		}
		return fallbackGrade(transcript), nil
	}
	// Reset the "down logged" latch on the first successful grade
	// after an outage so a NEW outage produces a fresh WARN.
	c.graderDownLogged.Store(false)
	grade := gradeFromProto(response)
	grade.Source = models.GradeSourceGrader
	return grade, nil
}

func fallbackGrade(transcript models.Transcript) models.Grade {
	tokenEfficiency := 0.0
	if transcript.TotalTokens > 0 {
		tokenEfficiency = math.Min(1, 1000/float64(transcript.TotalTokens))
	}
	contextUtilization := 0.4
	if transcript.TotalTokens > 20 {
		contextUtilization = 0.8
	}
	processScore := ((tokenEfficiency * 0.3) + (contextUtilization * 0.3)) * 10
	compositeScore := processScore * 0.4
	return models.Grade{
		TestPassRate:              0,
		LintScore:                 5,
		TypeCheckPass:             false,
		FileStateValid:            true,
		TurnCount:                 transcript.TotalTurns,
		TotalTokens:               transcript.TotalTokens,
		CostUSD:                   transcript.CostUSD,
		TokenEfficiency:           tokenEfficiency,
		ContextUtilization:        contextUtilization,
		JudgeCorrectness:          0,
		JudgeMaintainability:      0,
		JudgeCompleteness:         0,
		JudgeBestPractices:        0,
		JudgeErrorHandling:        0,
		SpecInstructionCompliance: 0,
		SpecConventionAdherence:   0,
		CompositeScore:            compositeScore,
		Source:                    models.GradeSourceFallback,
	}
}

func gradeFromProto(response *graderpb.GradeRunResponse) models.Grade {
	grade := models.Grade{}
	if response.Code != nil {
		grade.TestPassRate = float64(response.Code.TestPassRate)
		grade.TestPassCount = int(response.Code.TestPassCount)
		grade.TestFailCount = int(response.Code.TestFailCount)
		grade.LintScore = float64(response.Code.LintScore)
		grade.TypeCheckPass = response.Code.TypeCheckPass
		grade.FileStateValid = response.Code.FileStateValid
		for _, test := range response.Code.TestResults {
			grade.TestResults = append(grade.TestResults, models.TestResult{Name: test.Name, Passed: test.Passed, Output: test.Output})
		}
	}
	if response.Process != nil {
		grade.TurnCount = int(response.Process.TurnCount)
		grade.TotalTokens = int(response.Process.TotalTokens)
		grade.CostUSD = float64(response.Process.CostUsd)
		grade.TokenEfficiency = float64(response.Process.TokenEfficiency)
		grade.BacktrackCount = int(response.Process.BacktrackCount)
		grade.SelfValidationRate = float64(response.Process.SelfValidationRate)
		grade.PrematureCompletion = response.Process.PrematureCompletion
		grade.IdleTurns = int(response.Process.IdleTurns)
		grade.ErrorRecoveryCount = int(response.Process.ErrorRecoveryCount)
		grade.ToolCallAccuracy = float64(response.Process.ToolCallAccuracy)
		grade.ContextUtilization = float64(response.Process.ContextUtilization)
	}
	if response.Judge != nil {
		grade.JudgeCorrectness = float64(response.Judge.Correctness)
		grade.JudgeMaintainability = float64(response.Judge.Maintainability)
		grade.JudgeCompleteness = float64(response.Judge.Completeness)
		grade.JudgeBestPractices = float64(response.Judge.BestPractices)
		grade.JudgeErrorHandling = float64(response.Judge.ErrorHandling)
		grade.JudgeIRRAlpha = float64(response.Judge.IrrAlpha)
		grade.RawJudgeResponses = response.Judge.RawResponses
	}
	if response.Adherence != nil {
		grade.SpecInstructionCompliance = float64(response.Adherence.InstructionCompliance)
		grade.SpecConstraintViolations = int(response.Adherence.ConstraintViolations)
		grade.SpecConventionAdherence = float64(response.Adherence.ConventionAdherence)
		for _, item := range response.Adherence.PerInstruction {
			grade.SpecPerInstruction = append(grade.SpecPerInstruction, models.InstructionResult{Instruction: item.Instruction, Status: item.Status, Reasoning: item.Reasoning})
		}
	}
	grade.CompositeScore = float64(response.CompositeScore)
	return grade
}

// AgentDx pivot retired the legacy `protoGrade` / `fallbackStats` machinery
// that fed the old ComputeStats / pairwise variant statistics view. The
// per-run Diagnostic Profile (Story #19-#23) replaces it. Stats helpers
// (mean/median/stddev) and the model.ExperimentStat type were dropped
// together; if AgentDx ever needs aggregate stats again, they belong in
// engine/pkg/diagnostic where they can compose with the fingerprint vector.

// buildJudgeConfig returns a JudgeConfig proto reflecting current SQLite
// state. Returns nil when judge is disabled or the settings store is
// missing — the grader treats a nil JudgeConfig as "use grader-side env
// defaults / disabled_judge_result", preserving the legacy headless path.
func (c *GraderClient) buildJudgeConfig(ctx context.Context) *graderpb.JudgeConfig {
	if c.settings == nil {
		return nil
	}
	settings, err := c.settings.GetSettingsByPrefix(ctx, "judge.")
	if err != nil {
		c.logger.Warn("buildJudgeConfig: settings lookup failed", "err", err)
		return nil
	}
	if settings["judge.enabled"] != "true" {
		return nil
	}
	provider := settings["judge.provider"]
	apiKey, _ := c.settings.GetDecryptedAPIKey(ctx, provider) // empty when missing — that's OK
	return &graderpb.JudgeConfig{
		Provider:    provider,
		Model:       settings["judge.model"],
		ApiKey:      apiKey,
		JudgeRounds: 1,
	}
}

// BuildJudgeConfigForTest exposes buildJudgeConfig for unit tests
// without exporting the method publicly. Production callers go through GradeRun.
func BuildJudgeConfigForTest(ctx context.Context, store SettingsStore) *graderpb.JudgeConfig {
	c := &GraderClient{settings: store, logger: slog.Default()}
	return c.buildJudgeConfig(ctx)
}
