package storage

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Store struct {
	DB *sql.DB
}

func Open(ctx context.Context, dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	store := &Store{DB: db}
	if err := store.runMigrations(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.DB.Close()
}

func (s *Store) runMigrations(ctx context.Context) error {
	if _, err := s.DB.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename   TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, entry := range entries {
		var existing string
		row := s.DB.QueryRowContext(ctx, `SELECT filename FROM schema_migrations WHERE filename = ?`, entry.Name())
		if scanErr := row.Scan(&existing); scanErr == nil {
			continue // already applied
		}
		contents, readErr := migrationFiles.ReadFile(filepath.Join("migrations", entry.Name()))
		if readErr != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), readErr)
		}
		sqlText := strings.TrimSpace(string(contents))
		if sqlText == "" {
			continue
		}
		for _, statement := range splitSQLStatements(sqlText) {
			if _, execErr := s.DB.ExecContext(ctx, statement); execErr != nil && !isIgnorableMigrationError(execErr) {
				return fmt.Errorf("apply migration %s: %w", entry.Name(), execErr)
			}
		}
		if _, err := s.DB.ExecContext(ctx, `INSERT INTO schema_migrations(filename) VALUES (?)`, entry.Name()); err != nil {
			return fmt.Errorf("record migration %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func marshalJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func unmarshalJSON[T any](raw string, fallback T) T {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	decoded := fallback
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return fallback
	}
	return decoded
}

func splitSQLStatements(contents string) []string {
	// Strip line comments first so semicolons inside comments don't confuse the splitter.
	var stripped strings.Builder
	for _, line := range strings.Split(contents, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		stripped.WriteString(line)
		stripped.WriteString("\n")
	}
	parts := strings.Split(stripped.String(), ";")
	statements := make([]string, 0, len(parts))
	for _, part := range parts {
		statement := strings.TrimSpace(part)
		if statement == "" {
			continue
		}
		statements = append(statements, statement)
	}
	return statements
}

func isIgnorableMigrationError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate column name") || strings.Contains(message, "already exists")
}
