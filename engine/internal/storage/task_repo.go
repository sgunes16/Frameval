package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/mustafaselman/frameval/engine/internal/models"
	"gopkg.in/yaml.v3"
)

type taskManifestWorkspace struct {
	Mode   string `json:"mode" yaml:"mode"`
	GitURL string `json:"git_url" yaml:"git_url"`
	GitRef string `json:"git_ref" yaml:"git_ref"`
}

type taskManifest struct {
	ID              string                 `json:"id" yaml:"id"`
	Name            string                 `json:"name" yaml:"name"`
	Description     string                 `json:"description" yaml:"description"`
	Category        string                 `json:"category" yaml:"category"`
	TemplateKind    string                 `json:"template_kind" yaml:"template_kind"`
	WorkspaceMode   string                 `json:"workspace_mode" yaml:"workspace_mode"`
	WorkspaceGitURL string                 `json:"workspace_git_url" yaml:"workspace_git_url"`
	WorkspaceGitRef string                 `json:"workspace_git_ref" yaml:"workspace_git_ref"`
	Workspace       *taskManifestWorkspace `json:"workspace,omitempty" yaml:"workspace,omitempty"`
	ComplexityScore float64                `json:"complexity_score" yaml:"complexity_score"`
	CodebaseType    string                 `json:"codebase_type" yaml:"codebase_type"`
	Prompt          string                 `json:"prompt" yaml:"prompt"`
	TechnicalDetail string                 `json:"technical_details" yaml:"technical_details"`
	SetupScript     string                 `json:"setup_script" yaml:"setup_script"`
	CodebasePath    string                 `json:"codebase_path" yaml:"codebase_path"`
	TestCases       []models.TestCase      `json:"test_cases" yaml:"test_cases"`
}

type taskManifestFile struct {
	Tasks []taskManifest `json:"tasks" yaml:"tasks"`
}

// SeedBuiltinTasks reads a tasks.yaml/tasks.yml/tasks.json manifest from tasksRoot
// (or, as a convenience, tasksRoot itself if it points directly at a file) and
// upserts every entry into the database. No per-task directories or tests/
// folders are required any more — each task declares its own verification
// command (typically a compile/syntax check) under `test_cases`.
func (s *Store) SeedBuiltinTasks(ctx context.Context, tasksRoot string) error {
	manifestPath := tasksRoot
	info, err := os.Stat(tasksRoot)
	if err != nil {
		return fmt.Errorf("stat tasks root: %w", err)
	}
	if info.IsDir() {
		resolved, resolveErr := resolveTasksManifestPath(tasksRoot)
		if resolveErr != nil {
			return resolveErr
		}
		manifestPath = resolved
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read tasks manifest %s: %w", manifestPath, err)
	}
	manifests, err := parseTaskManifests(manifestPath, raw)
	if err != nil {
		return fmt.Errorf("parse tasks manifest %s: %w", manifestPath, err)
	}
	sort.SliceStable(manifests, func(i, j int) bool { return manifests[i].ID < manifests[j].ID })
	for _, manifest := range manifests {
		if strings.TrimSpace(manifest.ID) == "" {
			return fmt.Errorf("task manifest missing id: %+v", manifest)
		}
		workspaceMode := manifest.WorkspaceMode
		gitURL := manifest.WorkspaceGitURL
		gitRef := manifest.WorkspaceGitRef
		if manifest.Workspace != nil {
			if manifest.Workspace.Mode != "" {
				workspaceMode = manifest.Workspace.Mode
			}
			if manifest.Workspace.GitURL != "" {
				gitURL = manifest.Workspace.GitURL
			}
			if manifest.Workspace.GitRef != "" {
				gitRef = manifest.Workspace.GitRef
			}
		}
		if workspaceMode == "" {
			if gitURL != "" {
				workspaceMode = "git"
			} else {
				workspaceMode = "empty"
			}
		}

		task := models.Task{
			ID:              manifest.ID,
			Name:            manifest.Name,
			Description:     manifest.Description,
			Category:        manifest.Category,
			TemplateKind:    fallbackString(manifest.TemplateKind, "builtin"),
			WorkspaceMode:   workspaceMode,
			WorkspaceGitURL: gitURL,
			WorkspaceGitRef: gitRef,
			ComplexityScore: manifest.ComplexityScore,
			CodebaseType:    manifest.CodebaseType,
			TaskPrompt:      manifest.Prompt,
			TechnicalDetail: manifest.TechnicalDetail,
			SetupScript:     manifest.SetupScript,
			CodebasePath:    manifest.CodebasePath,
			IsBuiltin:       true,
			TestCases:       manifest.TestCases,
		}
		if _, upsertErr := s.UpsertTask(ctx, task); upsertErr != nil {
			return upsertErr
		}
	}
	return nil
}

func resolveTasksManifestPath(tasksRoot string) (string, error) {
	for _, name := range []string{"tasks.yaml", "tasks.yml", "tasks.json"} {
		candidate := filepath.Join(tasksRoot, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("tasks manifest not found under %s; expected tasks.yaml, tasks.yml, or tasks.json", tasksRoot)
}

func parseTaskManifests(manifestPath string, raw []byte) ([]taskManifest, error) {
	switch strings.ToLower(filepath.Ext(manifestPath)) {
	case ".yaml", ".yml":
		var wrapped taskManifestFile
		if err := yaml.Unmarshal(raw, &wrapped); err == nil && len(wrapped.Tasks) > 0 {
			return wrapped.Tasks, nil
		}
		var manifests []taskManifest
		if err := yaml.Unmarshal(raw, &manifests); err != nil {
			return nil, err
		}
		return manifests, nil
	default:
		var wrapped taskManifestFile
		if err := json.Unmarshal(raw, &wrapped); err == nil && len(wrapped.Tasks) > 0 {
			return wrapped.Tasks, nil
		}
		var manifests []taskManifest
		if err := json.Unmarshal(raw, &manifests); err != nil {
			return nil, err
		}
		return manifests, nil
	}
}

func (s *Store) UpsertTask(ctx context.Context, task models.Task) (*models.Task, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin task tx: %w", err)
	}
	defer tx.Rollback()
	if task.ID == "" {
		task.ID = uuid.NewString()
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tasks (id, name, description, category, template_kind, workspace_mode, workspace_git_url, workspace_git_ref, complexity_score, codebase_type, task_prompt, technical_details, setup_script, codebase_path, task_root_path, is_builtin)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			category = excluded.category,
			template_kind = excluded.template_kind,
			workspace_mode = excluded.workspace_mode,
			workspace_git_url = excluded.workspace_git_url,
			workspace_git_ref = excluded.workspace_git_ref,
			complexity_score = excluded.complexity_score,
			codebase_type = excluded.codebase_type,
			task_prompt = excluded.task_prompt,
			technical_details = excluded.technical_details,
			setup_script = excluded.setup_script,
			codebase_path = excluded.codebase_path,
			task_root_path = excluded.task_root_path,
			is_builtin = excluded.is_builtin
	`, task.ID, task.Name, task.Description, task.Category, fallbackString(task.TemplateKind, "builtin"), fallbackString(task.WorkspaceMode, "empty"), nullableString(task.WorkspaceGitURL), nullableString(task.WorkspaceGitRef), task.ComplexityScore, task.CodebaseType, task.TaskPrompt, task.TechnicalDetail, task.SetupScript, task.CodebasePath, task.TaskRootPath, boolToInt(task.IsBuiltin))
	if err != nil {
		return nil, fmt.Errorf("upsert task: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM test_cases WHERE task_id = ?`, task.ID); err != nil {
		return nil, fmt.Errorf("delete existing test cases: %w", err)
	}
	for idx, testCase := range task.TestCases {
		if testCase.ID == "" {
			testCase.ID = uuid.NewString()
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO test_cases (id, task_id, name, test_command, expected_result, ordering)
			VALUES (?, ?, ?, ?, ?, ?)
		`, testCase.ID, task.ID, testCase.Name, testCase.TestCommand, testCase.ExpectedResult, maxInt(testCase.Ordering, idx))
		if err != nil {
			return nil, fmt.Errorf("insert test case: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit task tx: %w", err)
	}
	return s.GetTask(ctx, task.ID)
}

func (s *Store) ListTasks(ctx context.Context) ([]models.Task, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, name, description, category, template_kind, workspace_mode, workspace_git_url, workspace_git_ref, complexity_score, codebase_type, task_prompt, technical_details, setup_script, codebase_path, task_root_path, is_builtin, created_at
		FROM tasks ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()
	tasks := make([]models.Task, 0)
	for rows.Next() {
		task, scanErr := scanTask(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for idx := range tasks {
		testCases, casesErr := s.listTestCases(ctx, tasks[idx].ID)
		if casesErr != nil {
			return nil, casesErr
		}
		tasks[idx].TestCases = testCases
	}
	return tasks, nil
}

func (s *Store) GetTask(ctx context.Context, taskID string) (*models.Task, error) {
	row := s.DB.QueryRowContext(ctx, `
		SELECT id, name, description, category, template_kind, workspace_mode, workspace_git_url, workspace_git_ref, complexity_score, codebase_type, task_prompt, technical_details, setup_script, codebase_path, task_root_path, is_builtin, created_at
		FROM tasks WHERE id = ?
	`, taskID)
	task, err := scanTask(row)
	if err != nil {
		return nil, err
	}
	testCases, err := s.listTestCases(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	task.TestCases = testCases
	return &task, nil
}

func (s *Store) listTestCases(ctx context.Context, taskID string) ([]models.TestCase, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, task_id, name, test_command, expected_result, ordering FROM test_cases WHERE task_id = ? ORDER BY ordering ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list test cases: %w", err)
	}
	defer rows.Close()
	cases := make([]models.TestCase, 0)
	for rows.Next() {
		var testCase models.TestCase
		var expected sql.NullString
		if err := rows.Scan(&testCase.ID, &testCase.TaskID, &testCase.Name, &testCase.TestCommand, &expected, &testCase.Ordering); err != nil {
			return nil, fmt.Errorf("scan test case: %w", err)
		}
		testCase.ExpectedResult = expected.String
		cases = append(cases, testCase)
	}
	return cases, rows.Err()
}

func scanTask(scanner interface{ Scan(dest ...any) error }) (models.Task, error) {
	var task models.Task
	var technicalDetail, setupScript, codebasePath, taskRootPath, gitURL, gitRef sql.NullString
	var templateKind, workspaceMode string
	var isBuiltin int
	if err := scanner.Scan(&task.ID, &task.Name, &task.Description, &task.Category, &templateKind, &workspaceMode, &gitURL, &gitRef, &task.ComplexityScore, &task.CodebaseType, &task.TaskPrompt, &technicalDetail, &setupScript, &codebasePath, &taskRootPath, &isBuiltin, &task.CreatedAt); err != nil {
		return task, fmt.Errorf("scan task: %w", err)
	}
	task.TemplateKind = templateKind
	task.WorkspaceMode = workspaceMode
	task.WorkspaceGitURL = gitURL.String
	task.WorkspaceGitRef = gitRef.String
	task.TechnicalDetail = technicalDetail.String
	task.SetupScript = setupScript.String
	task.CodebasePath = codebasePath.String
	task.TaskRootPath = taskRootPath.String
	task.IsBuiltin = isBuiltin == 1
	return task, nil
}

func fallbackString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
