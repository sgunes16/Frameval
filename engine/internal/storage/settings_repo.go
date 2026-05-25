package storage

import (
	"context"
	"database/sql"
	"fmt"
)

// GetSetting returns the value for key. Returns sql.ErrNoRows when the
// key is absent — callers decide whether to fall back to env defaults
// or surface the error.
func (s *Store) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := s.DB.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

// SetSetting upserts the value for key, bumping updated_at to now.
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO app_settings (key, value)
		VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value      = excluded.value,
			updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
	`, key, value)
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}

// GetSettingsByPrefix returns every key starting with prefix. Empty
// map when no match (not an error).
func (s *Store) GetSettingsByPrefix(ctx context.Context, prefix string) (map[string]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT key, value FROM app_settings WHERE key LIKE ?`, prefix+"%")
	if err != nil {
		return nil, fmt.Errorf("query settings by prefix %q: %w", prefix, err)
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		out[k] = v
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// Silence unused import warning when sql.ErrNoRows isn't directly referenced
// here — the docstring claims callers receive it from GetSetting.
var _ = sql.ErrNoRows
