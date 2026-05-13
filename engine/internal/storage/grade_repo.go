package storage

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mustafaselman/frameval/engine/internal/models"
)

func (s *Store) SaveGrade(ctx context.Context, grade models.Grade) error {
	if grade.ID == "" {
		grade.ID = uuid.NewString()
	}
	if grade.RunID != "" {
		if _, err := s.DB.ExecContext(ctx, `DELETE FROM grades WHERE run_id = ?`, grade.RunID); err != nil {
			return fmt.Errorf("delete existing run grade: %w", err)
		}
	}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO grades (
			id, run_id, test_pass_rate, test_pass_count, test_fail_count, lint_score, type_check_pass,
			file_state_valid, turn_count, total_tokens, cost_usd, token_efficiency, backtrack_count,
			self_validation_rate, premature_completion, idle_turns, error_recovery_count, tool_call_accuracy,
			context_utilization, judge_correctness, judge_maintainability, judge_completeness, judge_best_practices,
			judge_error_handling, judge_irr_alpha, raw_judge_responses_json, spec_instruction_compliance,
			spec_constraint_violations, spec_convention_adherence, spec_per_instruction_json, composite_score, test_results_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, grade.ID, nullableString(grade.RunID), grade.TestPassRate, grade.TestPassCount, grade.TestFailCount, grade.LintScore,
		boolToInt(grade.TypeCheckPass), boolToInt(grade.FileStateValid), grade.TurnCount, grade.TotalTokens, grade.CostUSD, grade.TokenEfficiency,
		grade.BacktrackCount, grade.SelfValidationRate, boolToInt(grade.PrematureCompletion), grade.IdleTurns, grade.ErrorRecoveryCount,
		grade.ToolCallAccuracy, grade.ContextUtilization, grade.JudgeCorrectness, grade.JudgeMaintainability, grade.JudgeCompleteness,
		grade.JudgeBestPractices, grade.JudgeErrorHandling, grade.JudgeIRRAlpha, marshalJSON(grade.RawJudgeResponses), grade.SpecInstructionCompliance,
		grade.SpecConstraintViolations, grade.SpecConventionAdherence, marshalJSON(grade.SpecPerInstruction), grade.CompositeScore, marshalJSON(grade.TestResults))
	if err != nil {
		return fmt.Errorf("save grade: %w", err)
	}
	return nil
}

func (s *Store) GetGradeByRun(ctx context.Context, runID string) (*models.Grade, error) {
	row := s.DB.QueryRowContext(ctx, `
		SELECT id, run_id, test_pass_rate, test_pass_count, test_fail_count, lint_score, type_check_pass,
		       file_state_valid, turn_count, total_tokens, cost_usd, token_efficiency, backtrack_count,
		       self_validation_rate, premature_completion, idle_turns, error_recovery_count, tool_call_accuracy,
		       context_utilization, judge_correctness, judge_maintainability, judge_completeness, judge_best_practices,
		       judge_error_handling, judge_irr_alpha, raw_judge_responses_json, spec_instruction_compliance,
		       spec_constraint_violations, spec_convention_adherence, spec_per_instruction_json, composite_score, graded_at, test_results_json
		FROM grades WHERE run_id = ?
	`, runID)
	var grade models.Grade
	var typeCheckPass, fileStateValid, prematureCompletion int
	var rawJudge, perInstruction, testResults string
	if err := row.Scan(&grade.ID, &grade.RunID, &grade.TestPassRate, &grade.TestPassCount, &grade.TestFailCount, &grade.LintScore,
		&typeCheckPass, &fileStateValid, &grade.TurnCount, &grade.TotalTokens, &grade.CostUSD, &grade.TokenEfficiency,
		&grade.BacktrackCount, &grade.SelfValidationRate, &prematureCompletion, &grade.IdleTurns, &grade.ErrorRecoveryCount,
		&grade.ToolCallAccuracy, &grade.ContextUtilization, &grade.JudgeCorrectness, &grade.JudgeMaintainability,
		&grade.JudgeCompleteness, &grade.JudgeBestPractices, &grade.JudgeErrorHandling, &grade.JudgeIRRAlpha, &rawJudge,
		&grade.SpecInstructionCompliance, &grade.SpecConstraintViolations, &grade.SpecConventionAdherence, &perInstruction,
		&grade.CompositeScore, &grade.GradedAt, &testResults); err != nil {
		return nil, fmt.Errorf("get grade: %w", err)
	}
	grade.TypeCheckPass = typeCheckPass == 1
	grade.FileStateValid = fileStateValid == 1
	grade.PrematureCompletion = prematureCompletion == 1
	grade.RawJudgeResponses = unmarshalJSON(rawJudge, []string{})
	grade.SpecPerInstruction = unmarshalJSON(perInstruction, []models.InstructionResult{})
	grade.TestResults = unmarshalJSON(testResults, []models.TestResult{})
	return &grade, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
