package storage

import (
	"context"
	"database/sql"
	"fmt"
)

type Rubric struct {
	Key         string `json:"key"`
	DisplayName string `json:"display_name"`
	Prompt      string `json:"prompt"`
	SortOrder   int    `json:"sort_order"`
	IsBuiltin   bool   `json:"is_builtin"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

func (s *Store) ListRubrics(ctx context.Context) ([]Rubric, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT key, display_name, prompt, sort_order, is_builtin, created_at, updated_at
		FROM rubrics ORDER BY sort_order, key`)
	if err != nil {
		return nil, fmt.Errorf("list rubrics: %w", err)
	}
	defer rows.Close()
	out := make([]Rubric, 0)
	for rows.Next() {
		var r Rubric
		var isB int
		if err := rows.Scan(&r.Key, &r.DisplayName, &r.Prompt, &r.SortOrder, &isB, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan rubric: %w", err)
		}
		r.IsBuiltin = isB == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetRubric(ctx context.Context, key string) (Rubric, error) {
	var r Rubric
	var isB int
	err := s.DB.QueryRowContext(ctx, `
		SELECT key, display_name, prompt, sort_order, is_builtin, created_at, updated_at
		FROM rubrics WHERE key = ?`, key).
		Scan(&r.Key, &r.DisplayName, &r.Prompt, &r.SortOrder, &isB, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return Rubric{}, err
	}
	r.IsBuiltin = isB == 1
	return r, nil
}

// UpsertRubric inserts or updates by key. is_builtin from the input is
// IGNORED on update — once a row is builtin it stays builtin (only a
// new INSERT can set is_builtin=true, and the API never does that).
func (s *Store) UpsertRubric(ctx context.Context, r Rubric) error {
	isB := 0
	if r.IsBuiltin {
		isB = 1
	}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO rubrics (key, display_name, prompt, sort_order, is_builtin)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			display_name = excluded.display_name,
			prompt = excluded.prompt,
			sort_order = excluded.sort_order,
			updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
	`, r.Key, r.DisplayName, r.Prompt, r.SortOrder, isB)
	if err != nil {
		return fmt.Errorf("upsert rubric %q: %w", r.Key, err)
	}
	return nil
}

func (s *Store) DeleteRubric(ctx context.Context, key string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM rubrics WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("delete rubric %q: %w", key, err)
	}
	return nil
}

var _ = sql.ErrNoRows
