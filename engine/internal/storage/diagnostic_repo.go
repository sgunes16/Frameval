package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/mustafaselman/frameval/engine/pkg/diagnostic"
)

// DiagnosticRecord persists the four parts of an AgentDx Diagnostic Profile
// alongside the run that produced them. `FailureLabel` is nullable — when
// the classifier was unavailable (no API key, network error, timeout) it
// stays nil rather than carrying a meaningless NONE row.
type DiagnosticRecord struct {
	ID                  string
	RunID               string
	Fingerprint         diagnostic.Fingerprint
	Symptoms            diagnostic.Symptoms
	Recovery            diagnostic.RecoveryProfile
	FailureLabel        *diagnostic.FailureClassification
	ClassifierModel     string
	ClassifierLatencyMs int32
}

// SaveDiagnostic writes a Diagnostic row. Existing rows for the same run
// are replaced (delete-then-insert) so re-grading is idempotent.
func (s *Store) SaveDiagnostic(ctx context.Context, rec DiagnosticRecord) error {
	if rec.ID == "" {
		rec.ID = uuid.NewString()
	}
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM diagnostic WHERE run_id = ?`, rec.RunID); err != nil {
		return fmt.Errorf("clear diagnostic for run %s: %w", rec.RunID, err)
	}
	failureLabelJSON := sql.NullString{}
	if rec.FailureLabel != nil {
		failureLabelJSON.String = marshalJSON(rec.FailureLabel)
		failureLabelJSON.Valid = true
	}
	classifierModel := sql.NullString{}
	if rec.ClassifierModel != "" {
		classifierModel.String = rec.ClassifierModel
		classifierModel.Valid = true
	}
	classifierLatency := sql.NullInt64{}
	if rec.ClassifierLatencyMs > 0 {
		classifierLatency.Int64 = int64(rec.ClassifierLatencyMs)
		classifierLatency.Valid = true
	}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO diagnostic (
			id, run_id, fingerprint, symptoms, recovery,
			failure_label, classifier_model, classifier_latency_ms, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	`,
		rec.ID,
		rec.RunID,
		marshalJSON(rec.Fingerprint),
		marshalJSON(rec.Symptoms),
		marshalJSON(rec.Recovery),
		failureLabelJSON,
		classifierModel,
		classifierLatency,
	)
	if err != nil {
		return fmt.Errorf("insert diagnostic for run %s: %w", rec.RunID, err)
	}
	return nil
}

// GetDiagnosticByRun loads the diagnostic record for a run, or returns
// sql.ErrNoRows if none exists.
func (s *Store) GetDiagnosticByRun(ctx context.Context, runID string) (*DiagnosticRecord, error) {
	row := s.DB.QueryRowContext(ctx, `
		SELECT id, run_id, fingerprint, symptoms, recovery,
		       failure_label, classifier_model, classifier_latency_ms
		FROM diagnostic WHERE run_id = ?
	`, runID)
	var rec DiagnosticRecord
	var fp, sym, rec3 string
	var failureLabel sql.NullString
	var classifierModel sql.NullString
	var classifierLatency sql.NullInt64
	if err := row.Scan(&rec.ID, &rec.RunID, &fp, &sym, &rec3, &failureLabel, &classifierModel, &classifierLatency); err != nil {
		return nil, err
	}
	rec.Fingerprint = unmarshalJSON(fp, diagnostic.Fingerprint{})
	rec.Symptoms = unmarshalJSON(sym, diagnostic.Symptoms{})
	rec.Recovery = unmarshalJSON(rec3, diagnostic.RecoveryProfile{})
	if failureLabel.Valid {
		fl := unmarshalJSON(failureLabel.String, diagnostic.FailureClassification{})
		rec.FailureLabel = &fl
	}
	if classifierModel.Valid {
		rec.ClassifierModel = classifierModel.String
	}
	if classifierLatency.Valid {
		rec.ClassifierLatencyMs = int32(classifierLatency.Int64)
	}
	return &rec, nil
}
