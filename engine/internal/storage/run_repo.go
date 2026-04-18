package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/mustafaselman/frameval/engine/internal/models"
)

func (s *Store) EnsureRunsForExperiment(ctx context.Context, experimentID string) error {
	experiment, err := s.GetExperiment(ctx, experimentID)
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin runs tx: %w", err)
	}
	defer tx.Rollback()
	for _, variant := range experiment.Variants {
		for runNumber := 1; runNumber <= experiment.RunsPerVariant; runNumber++ {
			_, execErr := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO runs (id, experiment_id, variant_id, run_number, status)
				VALUES (?, ?, ?, ?, 'pending')
			`, uuid.NewString(), experiment.ID, variant.ID, runNumber)
			if execErr != nil {
				return fmt.Errorf("ensure run: %w", execErr)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit runs tx: %w", err)
	}
	return nil
}

func (s *Store) ListRunsByExperiment(ctx context.Context, experimentID string) ([]models.Run, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, experiment_id, variant_id, run_number, status, container_id, environment_fingerprint_json,
		       started_at, completed_at, duration_seconds, error_message
		FROM runs WHERE experiment_id = ? ORDER BY run_number ASC
	`, experimentID)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()
	runs := make([]models.Run, 0)
	for rows.Next() {
		run, scanErr := scanRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for idx := range runs {
		if grade, gradeErr := s.GetGradeByRun(ctx, runs[idx].ID); gradeErr == nil {
			runs[idx].Grade = grade
		}
	}
	return runs, nil
}

func (s *Store) ListRunnableRuns(ctx context.Context, experimentID string) ([]models.Run, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, experiment_id, variant_id, run_number, status, container_id, environment_fingerprint_json,
		       started_at, completed_at, duration_seconds, error_message
		FROM runs WHERE experiment_id = ? AND status IN ('pending', 'failed') ORDER BY run_number ASC
	`, experimentID)
	if err != nil {
		return nil, fmt.Errorf("list runnable runs: %w", err)
	}
	defer rows.Close()
	runs := make([]models.Run, 0)
	for rows.Next() {
		run, scanErr := scanRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (s *Store) GetRun(ctx context.Context, runID string) (*models.Run, error) {
	row := s.DB.QueryRowContext(ctx, `
		SELECT id, experiment_id, variant_id, run_number, status, container_id, environment_fingerprint_json,
		       started_at, completed_at, duration_seconds, error_message
		FROM runs WHERE id = ?
	`, runID)
	run, err := scanRun(row)
	if err != nil {
		return nil, err
	}
	if transcript, transcriptErr := s.GetTranscriptByRun(ctx, runID); transcriptErr == nil {
		run.Transcript = transcript
	}
	if grade, gradeErr := s.GetGradeByRun(ctx, runID); gradeErr == nil {
		run.Grade = grade
	}
	return &run, nil
}

func (s *Store) UpdateRunStatus(ctx context.Context, runID string, status string, errorMessage string) error {
	_, err := s.DB.ExecContext(ctx, `
		UPDATE runs SET status = ?, error_message = ?,
		    started_at = CASE WHEN ? = 'running' AND started_at IS NULL THEN strftime('%Y-%m-%dT%H:%M:%SZ', 'now') ELSE started_at END,
		    completed_at = CASE WHEN ? IN ('completed', 'failed', 'cancelled', 'timeout') THEN strftime('%Y-%m-%dT%H:%M:%SZ', 'now') ELSE completed_at END,
		    duration_seconds = CASE
		        WHEN ? IN ('completed', 'failed', 'cancelled', 'timeout') AND started_at IS NOT NULL
		        THEN ROUND((julianday('now') - julianday(started_at)) * 86400.0, 3)
		        ELSE duration_seconds
		    END
		WHERE id = ?
	`, status, errorMessage, status, status, status, runID)
	if err != nil {
		return fmt.Errorf("update run status: %w", err)
	}
	return nil
}

func scanRun(scanner interface{ Scan(dest ...any) error }) (models.Run, error) {
	var run models.Run
	var containerID, fingerprint, startedAt, completedAt, errorMessage sql.NullString
	var duration sql.NullFloat64
	if err := scanner.Scan(&run.ID, &run.ExperimentID, &run.VariantID, &run.RunNumber, &run.Status, &containerID, &fingerprint, &startedAt, &completedAt, &duration, &errorMessage); err != nil {
		return run, fmt.Errorf("scan run: %w", err)
	}
	run.ContainerID = containerID.String
	run.EnvironmentFingerprintJSON = fingerprint.String
	run.StartedAt = startedAt.String
	run.CompletedAt = completedAt.String
	run.ErrorMessage = errorMessage.String
	if duration.Valid {
		run.DurationSeconds = duration.Float64
	}
	return run, nil
}
