package storage_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mustafaselman/frameval/engine/test/support"
)

// TestSeedBuiltinTasksRecordsTaskRootPath pins the fix for the
// ClaudeMd harness: per-directory tasks loaded from disk must
// expose `task_root_path` so the harness can locate sibling
// harness_context/ bundles at runtime. Before this fix the column
// was always empty and every ClaudeMd run failed Setup with
// ErrClaudemdSourceMissing.
func TestSeedBuiltinTasksRecordsTaskRootPath(t *testing.T) {
	store := support.TmpStore(t)
	root := t.TempDir()

	taskDir := filepath.Join(root, "demo-task")
	if err := os.MkdirAll(filepath.Join(taskDir, "harness_context"), 0o755); err != nil {
		t.Fatalf("mkdir harness_context: %v", err)
	}
	manifest := []byte(`id: demo-task
name: Demo task
description: tiny manifest used for testing the seeder
category: greenfield
template_kind: builtin
workspace_mode: empty
complexity_score: 1.0
codebase_type: python
task_prompt: |
  do something
test_cases: []
`)
	if err := os.WriteFile(filepath.Join(taskDir, "task.yaml"), manifest, 0o644); err != nil {
		t.Fatalf("write task.yaml: %v", err)
	}

	if err := store.SeedBuiltinTasks(context.Background(), root); err != nil {
		t.Fatalf("SeedBuiltinTasks: %v", err)
	}

	got, err := store.GetTask(context.Background(), "demo-task")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.TaskRootPath == "" {
		t.Fatalf("expected TaskRootPath to be set, got empty")
	}
	want := taskDir
	if got.TaskRootPath != want {
		t.Errorf("TaskRootPath = %q, want %q", got.TaskRootPath, want)
	}
}
