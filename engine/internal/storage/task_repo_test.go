package storage_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mustafaselman/frameval/engine/test/support"
)

// truthSetTestRefRE matches references to evaluation-harness test files
// (the truth set) — e.g. test_scope.sh, test_imports_v2_api.py. These
// names must never appear in a task's agent-facing technical_details:
// the field is injected into the agent prompt by harnesses such as
// spec-kit, and naming the grading files leads thorough planning
// workflows to recreate them at the workspace root (false scope drift)
// and leaks trap answers. See the brownfield-* task.yaml rewrites.
var truthSetTestRefRE = regexp.MustCompile(`\btest_[A-Za-z0-9_]*\.(py|sh)\b`)

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

// TestTechnicalDetailDoesNotLeakTruthSet guards against the
// task-authoring bug where technical_details named the evaluation
// harness's own test files (test_scope.sh, test_*.py) and grading
// assertions. technical_details is agent-facing — spec-kit's plan
// stage injects it into the prompt — so a thorough planner reads
// "test_validator_works.py should exist", finds it missing during the
// run (truth-set tests are materialized after the agent), and writes
// it at the workspace root, tripping the scope check and leaking the
// trap answer. Every built-in task's technical_details must stay free
// of truth-set test file references.
func TestTechnicalDetailDoesNotLeakTruthSet(t *testing.T) {
	// Self-check: the detector must actually fire on a known leak,
	// otherwise a passing run would be vacuous.
	if !truthSetTestRefRE.MatchString("- test_scope.sh: only app/models.py may change") {
		t.Fatal("detector failed to match a known leak string")
	}

	tasksRoot, err := filepath.Abs(filepath.Join("..", "..", "..", "tasks"))
	if err != nil {
		t.Fatalf("resolve tasks root: %v", err)
	}
	if _, err := os.Stat(tasksRoot); err != nil {
		t.Fatalf("tasks root %s not found: %v", tasksRoot, err)
	}

	store := support.TmpStore(t)
	if err := store.SeedBuiltinTasks(context.Background(), tasksRoot); err != nil {
		t.Fatalf("SeedBuiltinTasks: %v", err)
	}
	tasks, err := store.ListTasks(context.Background())
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) == 0 {
		t.Fatal("no built-in tasks seeded")
	}

	for _, task := range tasks {
		if hits := truthSetTestRefRE.FindAllString(task.TechnicalDetail, -1); len(hits) > 0 {
			t.Errorf("task %q technical_details leaks truth-set test file(s): %s",
				task.ID, strings.Join(hits, ", "))
		}
	}
}
