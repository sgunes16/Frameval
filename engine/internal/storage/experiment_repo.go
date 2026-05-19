package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/mustafaselman/frameval/engine/internal/models"
)

func (s *Store) CreateExperiment(ctx context.Context, req models.ExperimentRequest) (*models.Experiment, error) {
	weights := req.CompositeWeights
	if len(weights) == 0 {
		weights = map[string]float64{"code": 0.3, "judge": 0.3, "process": 0.2, "adherence": 0.2}
	}
	if req.RunsPerVariant == 0 {
		req.RunsPerVariant = 5
	}
	if req.ExecutionMode == "" {
		req.ExecutionMode = "cli"
	}
	if req.MaxConcurrent == 0 {
		req.MaxConcurrent = 1
	}
	if req.TimeoutSeconds == 0 {
		req.TimeoutSeconds = 600
	}
	experimentID := uuid.NewString()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin experiment tx: %w", err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO experiments (
			id, name, description, status, task_id, workspace_source_type, local_path, git_url, git_ref, model, agent_cli, execution_mode,
			runs_per_variant, temperature, timeout_seconds, max_concurrent, judge_model, seed, composite_weights_json
		) VALUES (?, ?, ?, 'draft', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, experimentID, req.Name, req.Description, req.TaskID, fallbackExperimentWorkspace(req.WorkspaceSourceType), nullableString(req.LocalPath), nullableString(req.GitURL), nullableString(req.GitRef), req.Model, req.AgentCLI, req.ExecutionMode,
		req.RunsPerVariant, req.Temperature, req.TimeoutSeconds, req.MaxConcurrent, req.JudgeModel, req.Seed, marshalJSON(weights))
	if err != nil {
		return nil, fmt.Errorf("insert experiment: %w", err)
	}
	for idx, variantReq := range req.Variants {
		variantID := uuid.NewString()
		_, err = tx.ExecContext(ctx, `
			INSERT INTO variants (id, experiment_id, name, description, is_control, ordering, harness_id)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, variantID, experimentID, variantReq.Name, variantReq.Description, boolToInt(variantReq.IsControl), maxInt(variantReq.Ordering, idx), fallbackHarnessID(variantReq.HarnessID))
		if err != nil {
			return nil, fmt.Errorf("insert variant: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit experiment tx: %w", err)
	}
	return s.GetExperiment(ctx, experimentID)
}

func (s *Store) ListExperiments(ctx context.Context) ([]models.Experiment, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, name, description, status, task_id, workspace_source_type, local_path, git_url, git_ref, model, agent_cli, execution_mode,
		       runs_per_variant, temperature, timeout_seconds, max_concurrent, judge_model,
		       seed, estimated_cost_usd, actual_cost_usd, composite_weights_json,
		       created_at, started_at, completed_at
		FROM experiments ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list experiments: %w", err)
	}
	defer rows.Close()
	experiments := make([]models.Experiment, 0)
	for rows.Next() {
		experiment, scanErr := scanExperiment(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		experiments = append(experiments, experiment)
	}
	return experiments, rows.Err()
}

func (s *Store) GetExperiment(ctx context.Context, experimentID string) (*models.Experiment, error) {
	row := s.DB.QueryRowContext(ctx, `
		SELECT id, name, description, status, task_id, workspace_source_type, local_path, git_url, git_ref, model, agent_cli, execution_mode,
		       runs_per_variant, temperature, timeout_seconds, max_concurrent, judge_model,
		       seed, estimated_cost_usd, actual_cost_usd, composite_weights_json,
		       created_at, started_at, completed_at
		FROM experiments WHERE id = ?
	`, experimentID)
	experiment, err := scanExperiment(row)
	if err != nil {
		return nil, err
	}
	variants, err := s.ListVariantsByExperiment(ctx, experimentID)
	if err != nil {
		return nil, err
	}
	experiment.Variants = variants
	return &experiment, nil
}

func (s *Store) UpdateExperiment(ctx context.Context, experimentID string, req models.ExperimentRequest) (*models.Experiment, error) {
	_, err := s.DB.ExecContext(ctx, `
		UPDATE experiments
		SET name = ?, description = ?, task_id = ?, workspace_source_type = ?, local_path = ?, git_url = ?, git_ref = ?, model = ?, agent_cli = ?, execution_mode = ?,
		    runs_per_variant = ?, temperature = ?, timeout_seconds = ?, max_concurrent = ?, judge_model = ?, seed = ?, composite_weights_json = ?
		WHERE id = ?
	`, req.Name, req.Description, req.TaskID, fallbackExperimentWorkspace(req.WorkspaceSourceType), nullableString(req.LocalPath), nullableString(req.GitURL), nullableString(req.GitRef), req.Model, req.AgentCLI, req.ExecutionMode, maxInt(req.RunsPerVariant, 1),
		req.Temperature, req.TimeoutSeconds, req.MaxConcurrent, req.JudgeModel, req.Seed, marshalJSON(req.CompositeWeights), experimentID)
	if err != nil {
		return nil, fmt.Errorf("update experiment: %w", err)
	}
	return s.GetExperiment(ctx, experimentID)
}

func (s *Store) DeleteExperiment(ctx context.Context, experimentID string) error {
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM experiments WHERE id = ?`, experimentID); err != nil {
		return fmt.Errorf("delete experiment: %w", err)
	}
	return nil
}

func (s *Store) UpdateExperimentStatus(ctx context.Context, experimentID string, status string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE experiments SET status = ? WHERE id = ?`, status, experimentID)
	if err != nil {
		return fmt.Errorf("update experiment status: %w", err)
	}
	return nil
}

func (s *Store) ReconcileCompletedExperiments(ctx context.Context) (int64, error) {
	result, err := s.DB.ExecContext(ctx, `
		UPDATE experiments
		SET status = 'completed',
		    completed_at = COALESCE(completed_at, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		WHERE status = 'running'
		  AND EXISTS (
		      SELECT 1 FROM runs
		      WHERE runs.experiment_id = experiments.id
		  )
		  AND NOT EXISTS (
		      SELECT 1 FROM runs
		      WHERE runs.experiment_id = experiments.id
		        AND runs.status NOT IN ('completed', 'failed', 'cancelled', 'timeout')
		  )
	`)
	if err != nil {
		return 0, fmt.Errorf("reconcile completed experiments: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read reconciled experiment count: %w", err)
	}
	return count, nil
}

func (s *Store) SetExperimentEstimate(ctx context.Context, experimentID string, amount float64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE experiments SET estimated_cost_usd = ?, status = 'ready' WHERE id = ?`, amount, experimentID)
	if err != nil {
		return fmt.Errorf("set experiment estimate: %w", err)
	}
	return nil
}

// GetExperimentAnchors returns the cached anchor bundle as a raw JSON
// string. Empty / never-computed experiments return the canonical "{}"
// placeholder so callers don't have to distinguish missing-default
// from explicit-empty. Returns sql.ErrNoRows (wrapped) when the
// experiment row itself is missing — the handler maps that to 404
// and any other error to 500.
func (s *Store) GetExperimentAnchors(ctx context.Context, experimentID string) (string, error) {
	var raw sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT anchors_json FROM experiments WHERE id = ?`, experimentID).Scan(&raw)
	if err != nil {
		return "", fmt.Errorf("get experiment anchors: %w", err)
	}
	if !raw.Valid || raw.String == "" {
		return "{}", nil
	}
	return raw.String, nil
}

// SetExperimentAnchors persists a freshly built anchor bundle. The
// orchestrator calls this on run finalize, so it must be fast — a
// single UPDATE keyed by primary key, no triggers.
func (s *Store) SetExperimentAnchors(ctx context.Context, experimentID string, payload string) error {
	if _, err := s.DB.ExecContext(ctx, `UPDATE experiments SET anchors_json = ? WHERE id = ?`, payload, experimentID); err != nil {
		return fmt.Errorf("set experiment anchors: %w", err)
	}
	return nil
}

func scanExperiment(scanner interface{ Scan(dest ...any) error }) (models.Experiment, error) {
	var experiment models.Experiment
	var description, judgeModel, startedAt, completedAt, localPath, gitURL, gitRef sql.NullString
	var workspaceSourceType string
	var seed sql.NullInt64
	var estimated, actual sql.NullFloat64
	var weightsRaw string
	if err := scanner.Scan(
		&experiment.ID, &experiment.Name, &description, &experiment.Status, &experiment.TaskID, &workspaceSourceType, &localPath, &gitURL, &gitRef, &experiment.Model,
		&experiment.AgentCLI, &experiment.ExecutionMode, &experiment.RunsPerVariant, &experiment.Temperature,
		&experiment.TimeoutSeconds, &experiment.MaxConcurrent, &judgeModel, &seed, &estimated, &actual,
		&weightsRaw, &experiment.CreatedAt, &startedAt, &completedAt,
	); err != nil {
		return experiment, fmt.Errorf("scan experiment: %w", err)
	}
	experiment.Description = description.String
	experiment.WorkspaceSourceType = workspaceSourceType
	experiment.LocalPath = localPath.String
	experiment.GitURL = gitURL.String
	experiment.GitRef = gitRef.String
	experiment.JudgeModel = judgeModel.String
	experiment.StartedAt = startedAt.String
	experiment.CompletedAt = completedAt.String
	if seed.Valid {
		value := int(seed.Int64)
		experiment.Seed = &value
	}
	if estimated.Valid {
		value := estimated.Float64
		experiment.EstimatedCostUSD = &value
	}
	if actual.Valid {
		value := actual.Float64
		experiment.ActualCostUSD = &value
	}
	experiment.CompositeWeights = unmarshalJSON(weightsRaw, map[string]float64{"code": 0.3, "judge": 0.3, "process": 0.2, "adherence": 0.2})
	return experiment, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func maxInt(value int, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func fallbackExperimentWorkspace(value string) string {
	if value == "" {
		return "task_codebase"
	}
	return value
}
