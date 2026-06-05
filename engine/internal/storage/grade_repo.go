package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/mustafaselman/frameval/engine/internal/models"
)

func (s *Store) SaveGrade(ctx context.Context, grade models.Grade) error {
	if grade.ID == "" {
		grade.ID = uuid.NewString()
	}
	judgeScoresJSON, _ := json.Marshal(grade.JudgeScores)
	judgeRationalesJSON, _ := json.Marshal(grade.JudgeRationales)
	// Wrap DELETE+INSERT in a transaction so a concurrent GetGradeByRun
	// (e.g. the progressive-grading-view poller) never sees the empty
	// window between the two statements. Without this, the two-phase save
	// in orchestrator.executeRun (partial row → full row after the judge
	// returns) hands out a brief 404 every time it transitions.
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin save grade tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after successful Commit
	if grade.RunID != "" {
		if _, err := tx.ExecContext(ctx, `DELETE FROM grades WHERE run_id = ?`, grade.RunID); err != nil {
			return fmt.Errorf("delete existing run grade: %w", err)
		}
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO grades (
			id, run_id, test_pass_rate, test_pass_count, test_fail_count, lint_score, type_check_pass,
			file_state_valid, turn_count, total_tokens, cost_usd, token_efficiency, backtrack_count,
			self_validation_rate, premature_completion, idle_turns, error_recovery_count, tool_call_accuracy,
			context_utilization, judge_scores, judge_rationales, judge_irr_alpha, raw_judge_responses_json,
			spec_instruction_compliance, spec_constraint_violations, spec_convention_adherence,
			spec_per_instruction_json, composite_score, test_results_json, judge_user_prompt,
			tool_call_count, tool_error_rate, ran_validation, harness_adherence_score, harness_adherence_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, grade.ID, nullableString(grade.RunID), grade.TestPassRate, grade.TestPassCount, grade.TestFailCount, grade.LintScore,
		boolToInt(grade.TypeCheckPass), boolToInt(grade.FileStateValid), grade.TurnCount, grade.TotalTokens, grade.CostUSD, grade.TokenEfficiency,
		grade.BacktrackCount, grade.SelfValidationRate, boolToInt(grade.PrematureCompletion), grade.IdleTurns, grade.ErrorRecoveryCount,
		grade.ToolCallAccuracy, grade.ContextUtilization, string(judgeScoresJSON), string(judgeRationalesJSON),
		grade.JudgeIRRAlpha, marshalJSON(grade.RawJudgeResponses), grade.SpecInstructionCompliance,
		grade.SpecConstraintViolations, grade.SpecConventionAdherence, marshalJSON(grade.SpecPerInstruction),
		grade.CompositeScore, marshalJSON(grade.TestResults), grade.JudgeUserPrompt,
		grade.ToolCallCount, grade.ToolErrorRate, boolToInt(grade.RanValidation),
		grade.HarnessAdherenceScore, nullableString(grade.HarnessAdherenceJSON))
	if err != nil {
		return fmt.Errorf("save grade: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit save grade: %w", err)
	}
	return nil
}

func (s *Store) GetGradeByRun(ctx context.Context, runID string) (*models.Grade, error) {
	row := s.DB.QueryRowContext(ctx, `
		SELECT id, run_id, test_pass_rate, test_pass_count, test_fail_count, lint_score, type_check_pass,
		       file_state_valid, turn_count, total_tokens, cost_usd, token_efficiency, backtrack_count,
		       self_validation_rate, premature_completion, idle_turns, error_recovery_count, tool_call_accuracy,
		       context_utilization, judge_scores, judge_rationales, judge_irr_alpha, raw_judge_responses_json,
		       spec_instruction_compliance, spec_constraint_violations, spec_convention_adherence,
		       spec_per_instruction_json, composite_score, graded_at, test_results_json,
		       COALESCE(judge_user_prompt, ''),
		       tool_call_count, tool_error_rate, ran_validation, harness_adherence_score,
		       harness_adherence_json
		FROM grades WHERE run_id = ?
	`, runID)
	var grade models.Grade
	var typeCheckPass, fileStateValid, prematureCompletion, ranValidation int
	var judgeScores, judgeRationales, rawJudge, perInstruction, testResults string
	var harnessAdherenceJSON sql.NullString
	if err := row.Scan(&grade.ID, &grade.RunID, &grade.TestPassRate, &grade.TestPassCount, &grade.TestFailCount, &grade.LintScore,
		&typeCheckPass, &fileStateValid, &grade.TurnCount, &grade.TotalTokens, &grade.CostUSD, &grade.TokenEfficiency,
		&grade.BacktrackCount, &grade.SelfValidationRate, &prematureCompletion, &grade.IdleTurns, &grade.ErrorRecoveryCount,
		&grade.ToolCallAccuracy, &grade.ContextUtilization, &judgeScores, &judgeRationales, &grade.JudgeIRRAlpha, &rawJudge,
		&grade.SpecInstructionCompliance, &grade.SpecConstraintViolations, &grade.SpecConventionAdherence, &perInstruction,
		&grade.CompositeScore, &grade.GradedAt, &testResults, &grade.JudgeUserPrompt,
		&grade.ToolCallCount, &grade.ToolErrorRate, &ranValidation, &grade.HarnessAdherenceScore,
		&harnessAdherenceJSON); err != nil {
		return nil, fmt.Errorf("get grade: %w", err)
	}
	grade.TypeCheckPass = typeCheckPass == 1
	grade.FileStateValid = fileStateValid == 1
	grade.PrematureCompletion = prematureCompletion == 1
	grade.RanValidation = ranValidation == 1
	if harnessAdherenceJSON.Valid {
		grade.HarnessAdherenceJSON = harnessAdherenceJSON.String
	}
	if judgeScores != "" && judgeScores != "null" {
		_ = json.Unmarshal([]byte(judgeScores), &grade.JudgeScores)
	}
	if judgeRationales != "" && judgeRationales != "null" {
		_ = json.Unmarshal([]byte(judgeRationales), &grade.JudgeRationales)
	}
	grade.RawJudgeResponses = unmarshalJSON(rawJudge, []string{})
	grade.SpecPerInstruction = unmarshalJSON(perInstruction, []models.InstructionResult{})
	grade.TestResults = unmarshalJSON(testResults, []models.TestResult{})
	// Wall-clock time lives on the run, not the grade row; pull it in so the
	// compare view can offer "Time" as a process metric. Best-effort — a
	// missing/NULL duration just leaves it at 0 (rendered as no data point).
	var dur sql.NullFloat64
	if err := s.DB.QueryRowContext(ctx, `SELECT duration_seconds FROM runs WHERE id = ?`, runID).Scan(&dur); err == nil && dur.Valid {
		grade.DurationSeconds = dur.Float64
	}
	return &grade, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
