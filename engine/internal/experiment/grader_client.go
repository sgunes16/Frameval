package experiment

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/mustafaselman/frameval/engine/internal/models"
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

func (c *GraderClient) ComputeStats(ctx context.Context, experimentID string, variantGrades map[string][]models.Grade) ([]models.ExperimentStat, error) {
	if c.addr == "" {
		return fallbackStats(experimentID, variantGrades), nil
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(c.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fallbackStats(experimentID, variantGrades), nil
	}
	defer conn.Close()
	client := graderpb.NewGraderServiceClient(conn)
	request := &graderpb.ComputeStatsRequest{ExperimentId: experimentID}
	for variantID, grades := range variantGrades {
		variant := &graderpb.VariantGrades{VariantId: variantID}
		for _, grade := range grades {
			variant.Grades = append(variant.Grades, protoGrade(grade))
		}
		request.VariantGrades = append(request.VariantGrades, variant)
	}
	response, err := client.ComputeStats(ctx, request)
	if err != nil {
		return fallbackStats(experimentID, variantGrades), nil
	}
	stats := make([]models.ExperimentStat, 0, len(response.Stats))
	for _, stat := range response.Stats {
		stats = append(stats, models.ExperimentStat{VariantAID: stat.VariantAId, VariantBID: stat.VariantBId, MetricName: stat.MetricName, MeanA: float64(stat.MeanA), MeanB: float64(stat.MeanB), MedianA: float64(stat.MedianA), MedianB: float64(stat.MedianB), StdA: float64(stat.StdA), StdB: float64(stat.StdB), MannWhitneyU: float64(stat.MannWhitneyU), PValue: float64(stat.PValue), CohensD: float64(stat.CohensD), CILower: float64(stat.CiLower), CIUpper: float64(stat.CiUpper), IsSignificant: stat.IsSignificant, ObservedPower: float64(stat.ObservedPower)})
	}
	if len(stats) == 0 {
		return fallbackStats(experimentID, variantGrades), nil
	}
	return stats, nil
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

func protoGrade(grade models.Grade) *graderpb.GradeRunResponse {
	response := &graderpb.GradeRunResponse{CompositeScore: float32(grade.CompositeScore)}
	response.Code = &graderpb.CodeGradeResult{TestPassRate: float32(grade.TestPassRate), TestPassCount: int32(grade.TestPassCount), TestFailCount: int32(grade.TestFailCount), LintScore: float32(grade.LintScore), TypeCheckPass: grade.TypeCheckPass, FileStateValid: grade.FileStateValid}
	response.Process = &graderpb.ProcessGradeResult{TurnCount: int32(grade.TurnCount), TotalTokens: int32(grade.TotalTokens), CostUsd: float32(grade.CostUSD), TokenEfficiency: float32(grade.TokenEfficiency), BacktrackCount: int32(grade.BacktrackCount), SelfValidationRate: float32(grade.SelfValidationRate), PrematureCompletion: grade.PrematureCompletion, IdleTurns: int32(grade.IdleTurns), ErrorRecoveryCount: int32(grade.ErrorRecoveryCount), ToolCallAccuracy: float32(grade.ToolCallAccuracy), ContextUtilization: float32(grade.ContextUtilization)}
	response.Judge = &graderpb.JudgeGradeResult{Correctness: float32(grade.JudgeCorrectness), Maintainability: float32(grade.JudgeMaintainability), Completeness: float32(grade.JudgeCompleteness), BestPractices: float32(grade.JudgeBestPractices), ErrorHandling: float32(grade.JudgeErrorHandling), IrrAlpha: float32(grade.JudgeIRRAlpha), RawResponses: grade.RawJudgeResponses}
	response.Adherence = &graderpb.SpecAdherenceResult{InstructionCompliance: float32(grade.SpecInstructionCompliance), ConstraintViolations: int32(grade.SpecConstraintViolations), ConventionAdherence: float32(grade.SpecConventionAdherence)}
	return response
}

func fallbackStats(experimentID string, variantGrades map[string][]models.Grade) []models.ExperimentStat {
	variantIDs := make([]string, 0, len(variantGrades))
	for variantID := range variantGrades {
		variantIDs = append(variantIDs, variantID)
	}
	sort.Strings(variantIDs)
	if len(variantIDs) < 2 {
		return []models.ExperimentStat{}
	}
	metrics := []struct {
		name    string
		extract func(models.Grade) float64
	}{
		{name: "composite_score", extract: func(grade models.Grade) float64 { return grade.CompositeScore }},
		{name: "test_pass_rate", extract: func(grade models.Grade) float64 { return grade.TestPassRate }},
		{name: "token_efficiency", extract: func(grade models.Grade) float64 { return grade.TokenEfficiency }},
		{name: "context_utilization", extract: func(grade models.Grade) float64 { return grade.ContextUtilization }},
	}
	stats := make([]models.ExperimentStat, 0, len(metrics))
	leftID, rightID := variantIDs[0], variantIDs[1]
	for _, metric := range metrics {
		valuesA := collectMetricValues(variantGrades[leftID], metric.extract)
		valuesB := collectMetricValues(variantGrades[rightID], metric.extract)
		if len(valuesA) == 0 || len(valuesB) == 0 {
			continue
		}
		stats = append(stats, models.ExperimentStat{
			ExperimentID:  experimentID,
			VariantAID:    leftID,
			VariantBID:    rightID,
			MetricName:    metric.name,
			MeanA:         mean(valuesA),
			MeanB:         mean(valuesB),
			MedianA:       median(valuesA),
			MedianB:       median(valuesB),
			StdA:          stddev(valuesA),
			StdB:          stddev(valuesB),
			PValue:        math.NaN(),
			CohensD:       mean(valuesA) - mean(valuesB),
			CILower:       math.Min(min(valuesA), min(valuesB)),
			CIUpper:       math.Max(max(valuesA), max(valuesB)),
			IsSignificant: false,
			ObservedPower: 0,
		})
	}
	return stats
}

func collectMetricValues(grades []models.Grade, extractor func(models.Grade) float64) []float64 {
	values := make([]float64, 0, len(grades))
	for _, grade := range grades {
		values = append(values, extractor(grade))
	}
	return values
}

func mean(values []float64) float64 {
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func median(values []float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	middle := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[middle-1] + sorted[middle]) / 2
	}
	return sorted[middle]
}

func stddev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	avg := mean(values)
	total := 0.0
	for _, value := range values {
		diff := value - avg
		total += diff * diff
	}
	return math.Sqrt(total / float64(len(values)))
}

func min(values []float64) float64 {
	current := values[0]
	for _, value := range values[1:] {
		if value < current {
			current = value
		}
	}
	return current
}

func max(values []float64) float64 {
	current := values[0]
	for _, value := range values[1:] {
		if value > current {
			current = value
		}
	}
	return current
}
