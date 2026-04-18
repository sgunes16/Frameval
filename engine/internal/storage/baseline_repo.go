package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/mustafaselman/frameval/engine/internal/models"
)

func (s *Store) UpsertBaseline(ctx context.Context, baseline models.Baseline) (*models.Baseline, error) {
	if baseline.ID == "" {
		baseline.ID = uuid.NewString()
	}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO baselines (id, name, description, source, artifact_type, artifact_content, task_id, model, agent_cli, total_runs, evaluated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			source = excluded.source,
			artifact_type = excluded.artifact_type,
			artifact_content = excluded.artifact_content,
			task_id = excluded.task_id,
			model = excluded.model,
			agent_cli = excluded.agent_cli,
			total_runs = excluded.total_runs,
			evaluated_at = excluded.evaluated_at
	`, baseline.ID, baseline.Name, baseline.Description, baseline.Source, baseline.ArtifactType, baseline.ArtifactContent, baseline.TaskID, baseline.Model, baseline.AgentCLI, baseline.TotalRuns, baseline.EvaluatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert baseline: %w", err)
	}
	return s.GetBaseline(ctx, baseline.ID)
}

func (s *Store) ListBaselines(ctx context.Context) ([]models.Baseline, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, description, source, artifact_type, artifact_content, task_id, model, agent_cli, total_runs, evaluated_at FROM baselines ORDER BY evaluated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list baselines: %w", err)
	}
	defer rows.Close()
	baselines := make([]models.Baseline, 0)
	for rows.Next() {
		baseline, scanErr := scanBaseline(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		baselines = append(baselines, baseline)
	}
	return baselines, rows.Err()
}

func (s *Store) GetBaseline(ctx context.Context, baselineID string) (*models.Baseline, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT id, name, description, source, artifact_type, artifact_content, task_id, model, agent_cli, total_runs, evaluated_at FROM baselines WHERE id = ?`, baselineID)
	baseline, err := scanBaseline(row)
	if err != nil {
		return nil, err
	}
	return &baseline, nil
}

func scanBaseline(scanner interface{ Scan(dest ...any) error }) (models.Baseline, error) {
	var baseline models.Baseline
	var description, artifactType, artifactContent sql.NullString
	if err := scanner.Scan(&baseline.ID, &baseline.Name, &description, &baseline.Source, &artifactType, &artifactContent, &baseline.TaskID, &baseline.Model, &baseline.AgentCLI, &baseline.TotalRuns, &baseline.EvaluatedAt); err != nil {
		return baseline, fmt.Errorf("scan baseline: %w", err)
	}
	baseline.Description = description.String
	baseline.ArtifactType = artifactType.String
	baseline.ArtifactContent = artifactContent.String
	return baseline, nil
}
