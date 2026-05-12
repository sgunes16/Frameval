package benchmark

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mustafaselman/frameval/engine/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestImportTerminalBenchTasks_directoryTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	taskDir := filepath.Join(root, "hello-task")
	require.NoError(t, os.MkdirAll(filepath.Join(taskDir, "tests"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(`
descriptions:
  - key: base
    description: |
      Create hello.txt in the working directory.
difficulty: easy
max_test_timeout_sec: 45
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(taskDir, "seed.txt"), []byte("starter"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(taskDir, "tests", "test_outputs.py"), []byte("def test_placeholder():\n    assert True\n"), 0o644))

	store := openTestStore(t)
	defer store.Close()

	count, err := ImportTerminalBenchTasks(ctx, store, root)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	task, err := store.GetTask(ctx, "terminal-bench-hello-task")
	require.NoError(t, err)
	require.Equal(t, "terminal-bench", task.ExternalSource)
	require.Equal(t, "brownfield", task.Category)
	require.Len(t, task.TestCases, 1)
	require.Equal(t, "hidden", task.TestCases[0].Visibility)
	require.Equal(t, 45, task.TestCases[0].TimeoutSeconds)

	workspaceFiles, ok := task.Metadata["workspace_files"].([]any)
	require.True(t, ok)
	require.Len(t, workspaceFiles, 1)

	hiddenFiles, ok := task.Metadata["hidden_files"].([]any)
	require.True(t, ok)
	require.Len(t, hiddenFiles, 2)
}

func TestImportSWEBenchTasks_jsonlDataset(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	datasetPath := filepath.Join(root, "dataset.jsonl")
	require.NoError(t, os.WriteFile(datasetPath, []byte(`{"instance_id":"pallets__flask-1","repo":"pallets/flask","base_commit":"abc123","problem_statement":"Fix the regression.\nMore context.","version":"1.0","issue_url":"https://example.com/issue","pr_url":"https://example.com/pr","test_patch":"diff --git a/tests/test_app.py b/tests/test_app.py","FAIL_TO_PASS":"[\"tests/test_app.py::test_fix\"]","PASS_TO_PASS":"[\"tests/test_app.py::test_existing\"]"}`+"\n"), 0o644))

	store := openTestStore(t)
	defer store.Close()

	count, err := ImportSWEBenchTasks(ctx, store, datasetPath)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	task, err := store.GetTask(ctx, "swe-bench-pallets--flask-1")
	require.NoError(t, err)
	require.Equal(t, "swe-bench", task.ExternalSource)
	require.Equal(t, "git", task.WorkspaceMode)
	require.Equal(t, "https://github.com/pallets/flask.git", task.WorkspaceGitURL)
	require.Equal(t, "abc123", task.WorkspaceGitRef)
	require.Len(t, task.TestCases, 1)
	require.Equal(t, "hidden", task.TestCases[0].Visibility)
	require.Contains(t, task.TestCases[0].SetupScript, "git apply")
	require.Contains(t, task.TestCases[0].SetupScript, "git apply --3way")
	require.Contains(t, task.TestCases[0].TestCommand, "tests/test_app.py::test_fix")
}

func TestImportSWEBenchTasks_djangoUsesRuntestsLabels(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	datasetPath := filepath.Join(root, "dataset.jsonl")
	require.NoError(t, os.WriteFile(datasetPath, []byte(`{"instance_id":"django__django-11001","repo":"django/django","base_commit":"abc123","problem_statement":"Fix Django ordering.","version":"3.0","FAIL_TO_PASS":"[\"test_order_by_multiline_sql (expressions.tests.BasicExpressionsTests)\"]","PASS_TO_PASS":"[\"test_deconstruct (expressions.tests.FTests)\"]"}`+"\n"), 0o644))

	store := openTestStore(t)
	defer store.Close()

	count, err := ImportSWEBenchTasks(ctx, store, datasetPath)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	task, err := store.GetTask(ctx, "swe-bench-django--django-11001")
	require.NoError(t, err)
	require.Equal(t, "PYTHON=.frameval-venv/bin/python; [ -x \"$PYTHON\" ] || PYTHON=python3; \"$PYTHON\" tests/runtests.py --verbosity 2 'expressions.tests.BasicExpressionsTests.test_order_by_multiline_sql' 'expressions.tests.FTests.test_deconstruct'", task.TestCases[0].TestCommand)
	require.Contains(t, task.TestCases[0].SetupScript, "pytz sqlparse asgiref")
	require.NotContains(t, task.TestCases[0].SetupScript, "tests/requirements/*.txt")
	require.NotContains(t, task.TestCases[0].SetupScript, "npm install")
}

func openTestStore(t *testing.T) *storage.Store {
	t.Helper()
	dbDir := t.TempDir()
	store, err := storage.Open(context.Background(), filepath.Join(dbDir, "frameval.db"))
	require.NoError(t, err)
	return store
}
