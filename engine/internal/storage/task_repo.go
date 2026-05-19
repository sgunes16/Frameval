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
	ExternalSource  string                 `json:"external_source" yaml:"external_source"`
	ExternalID      string                 `json:"external_id" yaml:"external_id"`
	ExternalURL     string                 `json:"external_url" yaml:"external_url"`
	Metadata        map[string]any         `json:"metadata" yaml:"metadata"`
	Workspace       *taskManifestWorkspace `json:"workspace,omitempty" yaml:"workspace,omitempty"`
	ComplexityScore float64                `json:"complexity_score" yaml:"complexity_score"`
	CodebaseType    string                 `json:"codebase_type" yaml:"codebase_type"`
	// Prompt accepts both the legacy `prompt` key and the AgentDx-era
	// `task_prompt` key. The new per-directory task.yaml format uses
	// task_prompt (matches models.Task.TaskPrompt); the legacy monolithic
	// tasks.yaml used prompt. Either populates the same Go field via
	// the UnmarshalYAML hook below.
	Prompt          string            `json:"prompt" yaml:"prompt"`
	TaskPrompt      string            `json:"task_prompt" yaml:"task_prompt"`
	TechnicalDetail string            `json:"technical_details" yaml:"technical_details"`
	SetupScript     string            `json:"setup_script" yaml:"setup_script"`
	CodebasePath    string            `json:"codebase_path" yaml:"codebase_path"`
	TestCases       []models.TestCase `json:"test_cases" yaml:"test_cases"`
	// TaskRootPath is NOT a YAML field — the seeder fills it in with the
	// task's on-disk directory so harnesses can resolve sibling
	// `harness_context/` bundles (ClaudeMd, SpecKit). Without this the
	// ClaudeMd harness Setup returns ErrClaudemdSourceMissing for every
	// per-directory task even when the file exists on disk.
	TaskRootPath string `json:"-" yaml:"-"`
}

// promptText returns the agent prompt regardless of which YAML key the
// task.yaml author used. Per-directory AgentDx tasks use `task_prompt`;
// the legacy monolithic manifest used `prompt`.
func (m *taskManifest) promptText() string {
	if strings.TrimSpace(m.TaskPrompt) != "" {
		return m.TaskPrompt
	}
	return m.Prompt
}

type taskManifestFile struct {
	Tasks []taskManifest `json:"tasks" yaml:"tasks"`
}

// SeedBuiltinTasks loads task definitions from tasksRoot.
//
// Two discovery formats are supported:
//
//  1. **Per-directory (AgentDx-era):** `tasksRoot/<id>/task.yaml`. One
//     task per directory; the surrounding workspace/, tests/, setup.sh,
//     and eval.sh live as siblings. This is the format documented in
//     docs/task-authoring.md and used by the 3 stress tasks.
//
//  2. **Monolithic (legacy):** `tasksRoot/tasks.yaml` (or .yml/.json) with a
//     top-level `tasks: [...]` array. Kept for back-compat.
//
// Both formats can coexist — per-directory tasks take precedence on ID
// collision. If neither is found the seeder is a no-op (engine still
// starts; the UI shows zero tasks).
func (s *Store) SeedBuiltinTasks(ctx context.Context, tasksRoot string) error {
	manifestPath := tasksRoot
	info, err := os.Stat(tasksRoot)
	if err != nil {
		return fmt.Errorf("stat tasks root: %w", err)
	}

	manifests := []taskManifest{}
	seenIDs := map[string]struct{}{}

	if info.IsDir() {
		// Format 1: walk subdirectories for per-task task.yaml files.
		entries, err := os.ReadDir(tasksRoot)
		if err != nil {
			return fmt.Errorf("read tasks root: %w", err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			subPath := filepath.Join(tasksRoot, entry.Name(), "task.yaml")
			raw, err := os.ReadFile(subPath)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return fmt.Errorf("read task manifest %s: %w", subPath, err)
			}
			single, err := parseSingleTaskManifest(subPath, raw)
			if err != nil {
				return fmt.Errorf("parse task manifest %s: %w", subPath, err)
			}
			if strings.TrimSpace(single.ID) == "" {
				single.ID = entry.Name()
			}
			// Resolve the workspace/setup/eval paths relative to the task dir
			// so the orchestrator knows where to mount from at run time.
			taskDir := filepath.Join(tasksRoot, entry.Name())
			if single.CodebasePath == "" {
				wsPath := filepath.Join(taskDir, "workspace")
				if _, err := os.Stat(wsPath); err == nil {
					single.CodebasePath = wsPath
				}
			}
			// Record the on-disk task directory so harnesses (ClaudeMd,
			// SpecKit) can locate sibling harness_context/ bundles at
			// runtime. Legacy monolithic manifests don't have a per-task
			// directory, so this stays empty for them.
			single.TaskRootPath = taskDir
			manifests = append(manifests, single)
			seenIDs[single.ID] = struct{}{}
		}

		// Format 2: legacy monolithic manifest (only if a file is present).
		if resolved, resolveErr := resolveTasksManifestPath(tasksRoot); resolveErr == nil {
			manifestPath = resolved
		} else {
			// No legacy manifest; if we already found per-dir tasks we're done.
			if len(manifests) > 0 {
				return s.upsertManifests(ctx, manifests)
			}
			return nil
		}
	}

	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read tasks manifest %s: %w", manifestPath, err)
	}
	legacy, err := parseTaskManifests(manifestPath, raw)
	if err != nil {
		return fmt.Errorf("parse tasks manifest %s: %w", manifestPath, err)
	}
	for _, m := range legacy {
		if _, dup := seenIDs[m.ID]; dup {
			continue // per-dir task wins
		}
		manifests = append(manifests, m)
	}

	return s.upsertManifests(ctx, manifests)
}

// upsertManifests sorts by ID for stable upsert order, writes each task to
// the database, then prunes any built-in tasks that are no longer present in
// the manifest. The prune step keeps the DB in sync after tasks are renamed
// or deleted from disk — without it, removing a task.yaml leaves the row
// stranded forever. User-created tasks (is_builtin = 0) are never touched.
func (s *Store) upsertManifests(ctx context.Context, manifests []taskManifest) error {
	sort.SliceStable(manifests, func(i, j int) bool { return manifests[i].ID < manifests[j].ID })
	keepIDs := make([]string, 0, len(manifests))
	for _, manifest := range manifests {
		if id := strings.TrimSpace(manifest.ID); id != "" {
			keepIDs = append(keepIDs, id)
		}
	}
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

		// Setup script precedence: an inline `setup_script:` field in
		// task.yaml wins; otherwise fall back to a sibling `setup.sh`
		// file (older brownfield tasks store their setup that way).
		// Returning the file contents — not a `bash setup.sh` shim —
		// because PrepareWorkspace pipes SetupScript straight to
		// `sh -c`, and setup.sh isn't on the sandbox container's
		// filesystem during that stage.
		setupScript := manifest.SetupScript
		if strings.TrimSpace(setupScript) == "" && manifest.TaskRootPath != "" {
			candidate := filepath.Join(manifest.TaskRootPath, "setup.sh")
			if data, err := os.ReadFile(candidate); err == nil {
				setupScript = string(data)
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
			ExternalSource:  manifest.ExternalSource,
			ExternalID:      manifest.ExternalID,
			ExternalURL:     manifest.ExternalURL,
			Metadata:        manifest.Metadata,
			ComplexityScore: manifest.ComplexityScore,
			CodebaseType:    manifest.CodebaseType,
			TaskPrompt:      manifest.promptText(),
			TechnicalDetail: manifest.TechnicalDetail,
			SetupScript:     setupScript,
			CodebasePath:    manifest.CodebasePath,
			TaskRootPath:    manifest.TaskRootPath,
			IsBuiltin:       true,
			TestCases:       manifest.TestCases,
		}
		if _, upsertErr := s.UpsertTask(ctx, task); upsertErr != nil {
			return upsertErr
		}
	}
	return s.pruneOrphanBuiltinTasks(ctx, keepIDs)
}

// pruneOrphanBuiltinTasks deletes any rows marked is_builtin = 1 whose ID is
// not in keepIDs. Called after the seeder finishes upserting so that
// tasks removed from disk also vanish from /api/tasks on the next boot.
func (s *Store) pruneOrphanBuiltinTasks(ctx context.Context, keepIDs []string) error {
	if len(keepIDs) == 0 {
		// Refuse to wipe the entire built-in table when the manifest is empty
		// (e.g. misconfigured FRAMEVAL_TASKS_ROOT). Better to leave stale rows
		// than to delete everything by accident.
		return nil
	}
	placeholders := strings.Repeat("?,", len(keepIDs))
	placeholders = placeholders[:len(placeholders)-1]
	query := fmt.Sprintf("DELETE FROM tasks WHERE is_builtin = 1 AND id NOT IN (%s)", placeholders)
	args := make([]any, len(keepIDs))
	for i, id := range keepIDs {
		args[i] = id
	}
	if _, err := s.DB.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("prune orphan built-in tasks: %w", err)
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

// parseSingleTaskManifest parses a per-directory task.yaml (or .json) that
// contains exactly one task — no `tasks:` wrapper. Used by the per-directory
// discovery branch in SeedBuiltinTasks.
func parseSingleTaskManifest(manifestPath string, raw []byte) (taskManifest, error) {
	var manifest taskManifest
	switch strings.ToLower(filepath.Ext(manifestPath)) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(raw, &manifest); err != nil {
			return manifest, err
		}
	default:
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return manifest, err
		}
	}
	return manifest, nil
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
		INSERT INTO tasks (id, name, description, category, template_kind, workspace_mode, workspace_git_url, workspace_git_ref, external_source, external_id, external_url, metadata_json, complexity_score, codebase_type, task_prompt, technical_details, setup_script, codebase_path, task_root_path, is_builtin)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			category = excluded.category,
			template_kind = excluded.template_kind,
			workspace_mode = excluded.workspace_mode,
			workspace_git_url = excluded.workspace_git_url,
			workspace_git_ref = excluded.workspace_git_ref,
			external_source = excluded.external_source,
			external_id = excluded.external_id,
			external_url = excluded.external_url,
			metadata_json = excluded.metadata_json,
			complexity_score = excluded.complexity_score,
			codebase_type = excluded.codebase_type,
			task_prompt = excluded.task_prompt,
			technical_details = excluded.technical_details,
			setup_script = excluded.setup_script,
			codebase_path = excluded.codebase_path,
			task_root_path = excluded.task_root_path,
			is_builtin = excluded.is_builtin
	`, task.ID, task.Name, task.Description, task.Category, fallbackString(task.TemplateKind, "builtin"), fallbackString(task.WorkspaceMode, "empty"), nullableString(task.WorkspaceGitURL), nullableString(task.WorkspaceGitRef), nullableString(task.ExternalSource), nullableString(task.ExternalID), nullableString(task.ExternalURL), marshalJSON(task.Metadata), task.ComplexityScore, task.CodebaseType, task.TaskPrompt, task.TechnicalDetail, task.SetupScript, task.CodebasePath, task.TaskRootPath, boolToInt(task.IsBuiltin))
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
			INSERT INTO test_cases (id, task_id, name, test_command, expected_result, visibility, timeout_seconds, setup_script, ordering)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, testCase.ID, task.ID, testCase.Name, testCase.TestCommand, testCase.ExpectedResult, nullableString(fallbackString(testCase.Visibility, "public")), maxInt(testCase.TimeoutSeconds, 120), nullableString(testCase.SetupScript), maxInt(testCase.Ordering, idx))
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
		SELECT id, name, description, category, template_kind, workspace_mode, workspace_git_url, workspace_git_ref, external_source, external_id, external_url, '{}', complexity_score, codebase_type, task_prompt, technical_details, setup_script, codebase_path, task_root_path, is_builtin, created_at
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
	return tasks, nil
}

func (s *Store) GetTask(ctx context.Context, taskID string) (*models.Task, error) {
	row := s.DB.QueryRowContext(ctx, `
		SELECT id, name, description, category, template_kind, workspace_mode, workspace_git_url, workspace_git_ref, external_source, external_id, external_url, metadata_json, complexity_score, codebase_type, task_prompt, technical_details, setup_script, codebase_path, task_root_path, is_builtin, created_at
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
	rows, err := s.DB.QueryContext(ctx, `SELECT id, task_id, name, test_command, expected_result, visibility, timeout_seconds, setup_script, ordering FROM test_cases WHERE task_id = ? ORDER BY ordering ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list test cases: %w", err)
	}
	defer rows.Close()
	cases := make([]models.TestCase, 0)
	for rows.Next() {
		var testCase models.TestCase
		var expected, visibility, setupScript sql.NullString
		var timeout sql.NullInt64
		if err := rows.Scan(&testCase.ID, &testCase.TaskID, &testCase.Name, &testCase.TestCommand, &expected, &visibility, &timeout, &setupScript, &testCase.Ordering); err != nil {
			return nil, fmt.Errorf("scan test case: %w", err)
		}
		testCase.ExpectedResult = expected.String
		testCase.Visibility = visibility.String
		if timeout.Valid {
			testCase.TimeoutSeconds = int(timeout.Int64)
		}
		testCase.SetupScript = setupScript.String
		cases = append(cases, testCase)
	}
	return cases, rows.Err()
}

func scanTask(scanner interface{ Scan(dest ...any) error }) (models.Task, error) {
	var task models.Task
	var technicalDetail, setupScript, codebasePath, taskRootPath, gitURL, gitRef sql.NullString
	var externalSource, externalID, externalURL, metadata sql.NullString
	var templateKind, workspaceMode string
	var isBuiltin int
	if err := scanner.Scan(&task.ID, &task.Name, &task.Description, &task.Category, &templateKind, &workspaceMode, &gitURL, &gitRef, &externalSource, &externalID, &externalURL, &metadata, &task.ComplexityScore, &task.CodebaseType, &task.TaskPrompt, &technicalDetail, &setupScript, &codebasePath, &taskRootPath, &isBuiltin, &task.CreatedAt); err != nil {
		return task, fmt.Errorf("scan task: %w", err)
	}
	task.TemplateKind = templateKind
	task.WorkspaceMode = workspaceMode
	task.WorkspaceGitURL = gitURL.String
	task.WorkspaceGitRef = gitRef.String
	task.ExternalSource = externalSource.String
	task.ExternalID = externalID.String
	task.ExternalURL = externalURL.String
	task.Metadata = unmarshalJSON(metadata.String, map[string]any{})
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
