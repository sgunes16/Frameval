package experiment

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"time"

	"github.com/mustafaselman/frameval/engine/internal/models"
	"github.com/mustafaselman/frameval/engine/pkg/diagnostic"
	graderpb "github.com/mustafaselman/frameval/engine/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GraderClient struct {
	addr string
}

func NewGraderClient(addr string) *GraderClient {
	return &GraderClient{addr: addr}
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
	if c.addr == "" {
		return fallback
	}
	symptomsJSON, err := json.Marshal(symptoms)
	if err != nil {
		return fallback
	}
	ctx, cancel := context.WithTimeout(ctx, classifyFailureTimeout)
	defer cancel()
	conn, err := grpc.NewClient(c.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fallback
	}
	defer conn.Close()
	client := graderpb.NewGraderServiceClient(conn)
	response, err := client.ClassifyFailure(ctx, &graderpb.ClassifyFailureRequest{
		RunId:            runID,
		SymptomsJson:     symptomsJSON,
		TaskDescription:  taskDescription,
		TranscriptTail:   transcriptTail,
		ClassifierModel:  classifierModel,
	})
	if err != nil || response == nil || response.Classification == nil {
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

func (c *GraderClient) GradeRun(ctx context.Context, task models.Task, artifact *models.ArtifactVersion, transcript models.Transcript) (models.Grade, error) {
	if c.addr == "" {
		return fallbackGrade(transcript), nil
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(c.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fallbackGrade(transcript), nil
	}
	defer conn.Close()
	client := graderpb.NewGraderServiceClient(conn)
	request := &graderpb.GradeRunRequest{
		RunId:          transcript.RunID,
		TranscriptJson: transcript.RawOutput,
		FilesystemDiff: transcript.FilesystemDiff,
		Task:           &graderpb.TaskSpec{Id: task.ID, Prompt: task.TaskPrompt, CodebaseType: task.CodebaseType, SetupScript: task.SetupScript},
		JudgeConfig:    &graderpb.JudgeConfig{Model: "gpt-5.4", Provider: "openai", JudgeRounds: 1},
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
	response, err := client.GradeRun(ctx, request)
	if err != nil {
		return fallbackGrade(transcript), nil
	}
	return gradeFromProto(response), nil
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
