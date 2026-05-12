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

func (s *Store) GetTranscriptByRun(ctx context.Context, runID string) (*models.Transcript, error) {
	row := s.DB.QueryRowContext(ctx, `
		SELECT id, run_id, raw_output, parsed_turns_json, filesystem_diff, patch, total_turns, total_tokens, cost_usd, output_files_path
		FROM transcripts WHERE run_id = ?
	`, runID)
	var transcript models.Transcript
	var parsedTurns, files sql.NullString
	if err := row.Scan(&transcript.ID, &transcript.RunID, &transcript.RawOutput, &parsedTurns, &transcript.FilesystemDiff, &transcript.Patch, &transcript.TotalTurns, &transcript.TotalTokens, &transcript.CostUSD, &files); err != nil {
		return nil, fmt.Errorf("get transcript: %w", err)
	}
	transcript.ParsedTurns = unmarshalJSON(parsedTurns.String, []models.ParsedTurn{})
	transcript.OutputFiles = unmarshalJSON(files.String, []models.OutputFile{})
	return &transcript, nil
}
