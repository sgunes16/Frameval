// Package support contains test helpers for Frameval integration tests.
//
// The helpers here are stable building blocks for tests that exercise the
// orchestrator, sandbox, hub, and gRPC paths without external services.
// Production code never imports this package.
package support

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mustafaselman/frameval/engine/internal/storage"
)

// TmpStore opens a fresh on-disk SQLite store inside the test's temp directory
// and runs every embedded migration. The store is closed and the file removed
// in t.Cleanup, so callers do not have to manage lifecycle.
//
// On-disk (not :memory:) so that the migration runner's SetMaxOpenConns(1)
// constraint does not conflict with concurrent reads in the same test.
func TmpStore(t *testing.T) *storage.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "frameval_test.db")
	store, err := storage.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
		_ = os.Remove(dbPath)
	})
	return store
}
