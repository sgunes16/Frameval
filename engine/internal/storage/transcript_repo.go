package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/mustafaselman/frameval/engine/internal/models"
)

func (s *Store) SaveTranscript(ctx context.Context, transcript models.Transcript) error {
	if transcript.ID == "" {
		transcript.ID = uuid.NewString()
	}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO transcripts (id, run_id, raw_output, parsed_turns_json, filesystem_diff, patch, total_turns, total_tokens, cost_usd, output_files_path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id) DO UPDATE SET
			raw_output = excluded.raw_output,
			parsed_turns_json = excluded.parsed_turns_json,
			filesystem_diff = excluded.filesystem_diff,
			patch = excluded.patch,
			total_turns = excluded.total_turns,
			total_tokens = excluded.total_tokens,
			cost_usd = excluded.cost_usd,
			output_files_path = excluded.output_files_path
	`, transcript.ID, transcript.RunID, transcript.RawOutput, marshalJSON(transcript.ParsedTurns), transcript.FilesystemDiff, transcript.Patch, transcript.TotalTurns, transcript.TotalTokens, transcript.CostUSD, marshalJSON(transcript.OutputFiles))
	if err != nil {
		return fmt.Errorf("save transcript: %w", err)
	}
	return nil
}

// UpdateTranscriptParsedTurns replaces ONLY the parsed_turns_json
// column for an existing transcript. Used by the reparse endpoint so
// transcripts written before a parser improvement can be refreshed
// without touching raw_output or any of the grade/diagnostic data
// downstream consumers cache on transcript identity.
func (s *Store) UpdateTranscriptParsedTurns(ctx context.Context, runID string, turns []models.ParsedTurn) error {
	res, err := s.DB.ExecContext(ctx,
		`UPDATE transcripts SET parsed_turns_json = ?, total_turns = ? WHERE run_id = ?`,
		marshalJSON(turns), len(turns), runID,
	)
	if err != nil {
		return fmt.Errorf("update parsed turns: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update parsed turns rows: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ListTurns returns the ParsedTurns for a single run. Returns an empty
// slice (not an error) when the run has no transcript yet — callers can
// poll this endpoint while a run is still streaming.
func (s *Store) ListTurns(ctx context.Context, runID string) ([]models.ParsedTurn, error) {
	var parsedTurns sql.NullString
	err := s.DB.QueryRowContext(ctx,
		`SELECT parsed_turns_json FROM transcripts WHERE run_id = ?`, runID,
	).Scan(&parsedTurns)
	if err == sql.ErrNoRows {
		return []models.ParsedTurn{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list turns: %w", err)
	}
	return unmarshalJSON(parsedTurns.String, []models.ParsedTurn{}), nil
}

// ListTurnsByExperiment returns turns grouped by run ID for every run
// in the experiment. Empty map for experiments with no transcripts yet.
//
// Single DB round-trip via a JOIN — N+1 anti-pattern avoided since
// Compare V2 calls this for every page load.
func (s *Store) ListTurnsByExperiment(ctx context.Context, experimentID string) (map[string][]models.ParsedTurn, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT t.run_id, t.parsed_turns_json
		FROM transcripts t
		JOIN runs r ON r.id = t.run_id
		WHERE r.experiment_id = ?
	`, experimentID)
	if err != nil {
		return nil, fmt.Errorf("list turns by experiment: %w", err)
	}
	defer rows.Close()

	out := make(map[string][]models.ParsedTurn)
	for rows.Next() {
		var runID string
		var parsedTurns sql.NullString
		if err := rows.Scan(&runID, &parsedTurns); err != nil {
			return nil, fmt.Errorf("list turns by experiment scan: %w", err)
		}
		out[runID] = unmarshalJSON(parsedTurns.String, []models.ParsedTurn{})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list turns by experiment iter: %w", err)
	}
	return out, nil
}

func (s *Store) GetTranscriptByRun(ctx context.Context, runID string) (*models.Transcript, error) {
	row := s.DB.QueryRowContext(ctx, `
		SELECT id, run_id, raw_output, parsed_turns_json, filesystem_diff, patch, total_turns, total_tokens, cost_usd, output_files_path
		FROM transcripts WHERE run_id = ?
	`, runID)
	var transcript models.Transcript
	var parsedTurns, files, patch sql.NullString
	if err := row.Scan(&transcript.ID, &transcript.RunID, &transcript.RawOutput, &parsedTurns, &transcript.FilesystemDiff, &patch, &transcript.TotalTurns, &transcript.TotalTokens, &transcript.CostUSD, &files); err != nil {
		return nil, fmt.Errorf("get transcript: %w", err)
	}
	transcript.Patch = patch.String
	transcript.ParsedTurns = unmarshalJSON(parsedTurns.String, []models.ParsedTurn{})
	transcript.OutputFiles = unmarshalJSON(files.String, []models.OutputFile{})
	return &transcript, nil
}
