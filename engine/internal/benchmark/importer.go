package benchmark

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mustafaselman/frameval/engine/internal/models"
	"github.com/mustafaselman/frameval/engine/internal/storage"
	"gopkg.in/yaml.v3"
)

type benchmarkFile struct {
	Path       string `json:"path"`
	Content    string `json:"content"`
	Executable bool   `json:"executable,omitempty"`
}

type terminalBenchManifest struct {
	Instruction         string                     `json:"instruction" yaml:"instruction"`
	Descriptions        []terminalBenchDescription `json:"descriptions" yaml:"descriptions"`
	Difficulty          string                     `json:"difficulty" yaml:"difficulty"`
	Category            string                     `json:"category" yaml:"category"`
	Tags                []string                   `json:"tags" yaml:"tags"`
	AuthorEmail         string                     `json:"author_email" yaml:"author_email"`
	MaxAgentTimeoutSec  float64                    `json:"max_agent_timeout_sec" yaml:"max_agent_timeout_sec"`
	MaxTestTimeoutSec   float64                    `json:"max_test_timeout_sec" yaml:"max_test_timeout_sec"`
	TestScripts         []string                   `json:"test_scripts" yaml:"test_scripts"`
	RunTestsInSameShell bool                       `json:"run_tests_in_same_shell" yaml:"run_tests_in_same_shell"`
}

type terminalBenchDescription struct {
	Key         string `json:"key" yaml:"key"`
	Description string `json:"description" yaml:"description"`
}

type sweBenchInstance struct {
	InstanceID       string `json:"instance_id"`
	Repo             string `json:"repo"`
	BaseCommit       string `json:"base_commit"`
	ProblemStatement string `json:"problem_statement"`
	Version          string `json:"version"`
	IssueURL         string `json:"issue_url"`
	PRURL            string `json:"pr_url"`
	TestPatch        string `json:"test_patch"`
	FailToPass       string `json:"FAIL_TO_PASS"`
	PassToPass       string `json:"PASS_TO_PASS"`
}

func ImportTerminalBenchTasks(ctx context.Context, store *storage.Store, root string) (int, error) {
	manifestPaths, err := discoverTerminalBenchManifests(root)
	if err != nil {
		return 0, err
	}
	imported := 0
	for _, manifestPath := range manifestPaths {
		task, err := loadTerminalBenchTask(manifestPath)
		if err != nil {
			return imported, err
		}
		if _, err := store.UpsertTask(ctx, task); err != nil {
			return imported, fmt.Errorf("import terminal-bench task %s: %w", task.ID, err)
		}
		imported++
	}
	return imported, nil
}

func ImportSWEBenchTasks(ctx context.Context, store *storage.Store, datasetPath string) (int, error) {
	instances, err := loadSWEBenchInstances(datasetPath)
	if err != nil {
		return 0, err
	}
	imported := 0
	for _, instance := range instances {
		task := buildSWEBenchTask(instance)
		if _, err := store.UpsertTask(ctx, task); err != nil {
			return imported, fmt.Errorf("import swe-bench task %s: %w", task.ID, err)
		}
		imported++
	}
	return imported, nil
}

func discoverTerminalBenchManifests(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat terminal-bench root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("terminal-bench root must be a directory: %s", root)
	}
	var manifests []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "task.yaml" || d.Name() == "task.yml" {
			manifests = append(manifests, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk terminal-bench root: %w", err)
	}
	return manifests, nil
}

func loadTerminalBenchTask(manifestPath string) (models.Task, error) {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return models.Task{}, fmt.Errorf("read terminal-bench manifest: %w", err)
	}
	var manifest terminalBenchManifest
	if err := yaml.Unmarshal(raw, &manifest); err != nil {
		return models.Task{}, fmt.Errorf("parse terminal-bench manifest: %w", err)
	}
	taskDir := filepath.Dir(manifestPath)
	taskID := filepath.Base(taskDir)
	workspaceFiles, err := collectBenchmarkFiles(taskDir, func(rel string, d fs.DirEntry) bool {
		if d.IsDir() {
			return rel != "tests" && rel != ".git"
		}
		switch rel {
		case "task.yaml", "task.yml", "solution.sh", "solution.yaml", "run-tests.sh", "Dockerfile", "docker-compose.yaml", "docker-compose.yml":
			return false
		}
		return !strings.HasPrefix(rel, "tests/")
	})
	if err != nil {
		return models.Task{}, err
	}
	hiddenFiles, err := collectBenchmarkFiles(taskDir, func(rel string, d fs.DirEntry) bool {
		if d.IsDir() {
			return rel == "tests"
		}
		return rel == "run-tests.sh" || strings.HasPrefix(rel, "tests/")
	})
	if err != nil {
		return models.Task{}, err
	}
	for idx := range hiddenFiles {
		hiddenFiles[idx].Path = filepath.ToSlash(filepath.Join(".frameval-hidden", hiddenFiles[idx].Path))
	}
	hiddenFiles = append(hiddenFiles, benchmarkFile{
		Path:       ".frameval-hidden/frameval-run-tests.sh",
		Content:    terminalBenchRunTestsScript(taskDir, manifest, hiddenFiles),
		Executable: true,
	})
	prompt := terminalBenchPrompt(manifest)
	description := fmt.Sprintf("Imported from Terminal-Bench (%s).", fallbackString(manifest.Difficulty, "unspecified"))
	if prompt == "" {
		prompt = fmt.Sprintf("Complete the Terminal-Bench task %s.", taskID)
	}
	return models.Task{
		ID:              "terminal-bench-" + taskID,
		Name:            taskID,
		Description:     description,
		Category:        benchmarkCategory(len(workspaceFiles) == 0),
		TemplateKind:    "benchmark_import",
		WorkspaceMode:   "empty",
		ExternalSource:  "terminal-bench",
		ExternalID:      taskID,
		ExternalURL:     "",
		Metadata:        benchmarkMetadata(workspaceFiles, hiddenFiles, map[string]any{"difficulty": manifest.Difficulty, "tags": manifest.Tags, "author_email": manifest.AuthorEmail, "source_path": taskDir}),
		ComplexityScore: benchmarkComplexity(manifest.Difficulty),
		CodebaseType:    detectCodebaseType(workspaceFiles, hiddenFiles, "shell"),
		TaskPrompt:      prompt,
		TechnicalDetail: fmt.Sprintf("Imported from %s", taskDir),
		SetupScript:     benchmarkDependencySetupScript(true),
		TaskRootPath:    taskDir,
		IsBuiltin:       false,
		TestCases: []models.TestCase{{
			Name:           "terminal-bench oracle",
			TestCommand:    "bash .frameval-hidden/frameval-run-tests.sh",
			Visibility:     "hidden",
			TimeoutSeconds: maxInt(int(manifest.MaxTestTimeoutSec), 120),
			Ordering:       0,
		}},
	}, nil
}

func terminalBenchPrompt(manifest terminalBenchManifest) string {
	if strings.TrimSpace(manifest.Instruction) != "" {
		return strings.TrimSpace(manifest.Instruction)
	}
	for _, item := range manifest.Descriptions {
		if item.Key == "base" && strings.TrimSpace(item.Description) != "" {
			return strings.TrimSpace(item.Description)
		}
	}
	for _, item := range manifest.Descriptions {
		if strings.TrimSpace(item.Description) != "" {
			return strings.TrimSpace(item.Description)
		}
	}
	return ""
}

func terminalBenchRunTestsScript(taskDir string, manifest terminalBenchManifest, hiddenFiles []benchmarkFile) string {
	available := map[string]bool{}
	for _, file := range hiddenFiles {
		available[strings.TrimPrefix(file.Path, ".frameval-hidden/")] = true
	}
	var lines []string
	lines = append(lines, "#!/bin/sh", "set -eu")
	if available["run-tests.sh"] {
		lines = append(lines, "sh .frameval-hidden/run-tests.sh")
		return strings.Join(lines, "\n") + "\n"
	}
	customScripts := make([]string, 0, len(manifest.TestScripts))
	for _, script := range manifest.TestScripts {
		normalized := filepath.ToSlash(strings.TrimSpace(script))
		if normalized != "" && available[normalized] {
			customScripts = append(customScripts, normalized)
		}
	}
	if len(customScripts) > 0 {
		for _, script := range customScripts {
			lines = append(lines, "sh "+shellQuote(filepath.ToSlash(filepath.Join(".frameval-hidden", script))))
		}
		return strings.Join(lines, "\n") + "\n"
	}
	lines = append(lines,
		"python3 -m pip install --quiet pytest >/dev/null 2>&1 || true",
		"pytest .frameval-hidden/tests/test_outputs.py -q",
	)
	return strings.Join(lines, "\n") + "\n"
}

func loadSWEBenchInstances(datasetPath string) ([]sweBenchInstance, error) {
	raw, err := os.ReadFile(datasetPath)
	if err != nil {
		return nil, fmt.Errorf("read swe-bench dataset: %w", err)
	}
	if bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) {
		var items []sweBenchInstance
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, fmt.Errorf("parse swe-bench json: %w", err)
		}
		return items, nil
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	items := make([]sweBenchInstance, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item sweBenchInstance
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("parse swe-bench jsonl line: %w", err)
		}
		items = append(items, item)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan swe-bench dataset: %w", err)
	}
	return items, nil
}

func buildSWEBenchTask(instance sweBenchInstance) models.Task {
	hiddenFiles := []benchmarkFile{}
	if strings.TrimSpace(instance.TestPatch) != "" {
		hiddenFiles = append(hiddenFiles, benchmarkFile{Path: ".frameval-hidden/swe_test.patch", Content: instance.TestPatch})
	}
	tests := append(parseSWETestList(instance.FailToPass), parseSWETestList(instance.PassToPass)...)
	command := sweBenchTestCommand(instance.Repo, tests)
	metadata := benchmarkMetadata(nil, hiddenFiles, map[string]any{
		"repo":         instance.Repo,
		"base_commit":  instance.BaseCommit,
		"issue_url":    instance.IssueURL,
		"pr_url":       instance.PRURL,
		"version":      instance.Version,
		"fail_to_pass": parseSWETestList(instance.FailToPass),
		"pass_to_pass": parseSWETestList(instance.PassToPass),
	})
	return models.Task{
		ID:              "swe-bench-" + sanitizeID(instance.InstanceID),
		Name:            instance.InstanceID,
		Description:     firstNonEmptyLine(instance.ProblemStatement),
		Category:        "bug-fix",
		TemplateKind:    "benchmark_import",
		WorkspaceMode:   "git",
		WorkspaceGitURL: "https://github.com/" + instance.Repo + ".git",
		WorkspaceGitRef: instance.BaseCommit,
		ExternalSource:  "swe-bench",
		ExternalID:      instance.InstanceID,
		ExternalURL:     firstNonEmpty(instance.PRURL, instance.IssueURL),
		Metadata:        metadata,
		ComplexityScore: 8,
		CodebaseType:    "python",
		TaskPrompt:      strings.TrimSpace(instance.ProblemStatement),
		TechnicalDetail: fmt.Sprintf("Repo: %s\nBase commit: %s", instance.Repo, instance.BaseCommit),
		SetupScript:     sweBenchDependencySetupScript(instance.Repo, false),
		IsBuiltin:       false,
		TestCases: []models.TestCase{{
			Name:           "swe-bench oracle",
			TestCommand:    command,
			Visibility:     "hidden",
			TimeoutSeconds: 600,
			SetupScript:    sweBenchVerificationSetupScript(instance.Repo, false),
			Ordering:       0,
		}},
	}
}

func sweBenchVerificationSetupScript(repo string, includeNodeInstall bool) string {
	return strings.Join([]string{
		sweBenchDependencySetupScript(repo, includeNodeInstall),
		"if [ -s .frameval-hidden/swe_test.patch ]; then",
		"  if git apply --check --recount --whitespace=nowarn .frameval-hidden/swe_test.patch; then",
		"    git apply --recount --whitespace=nowarn .frameval-hidden/swe_test.patch",
		"  else",
		"    git apply --3way --recount --whitespace=nowarn .frameval-hidden/swe_test.patch",
		"  fi",
		"fi",
	}, "\n")
}

func sweBenchDependencySetupScript(repo string, includeNodeInstall bool) string {
	if repo == "django/django" {
		return djangoDependencySetupScript()
	}
	return benchmarkDependencySetupScript(includeNodeInstall)
}

func djangoDependencySetupScript() string {
	lines := []string{
		"set +e",
		"python3 -m venv .frameval-venv || true",
		"if [ -f .frameval-venv/bin/activate ]; then . .frameval-venv/bin/activate; fi",
		"PYTHON=.frameval-venv/bin/python; [ -x \"$PYTHON\" ] || PYTHON=python3",
		"\"$PYTHON\" -m pip install --quiet --upgrade pip setuptools wheel || true",
		"\"$PYTHON\" -m pip install --quiet pytest numpy pytz sqlparse asgiref || true",
		"\"$PYTHON\" -m pip install --quiet -e . || true",
		"\"$PYTHON\" setup.py build_ext --inplace || true",
		"set -e",
	}
	return strings.Join(lines, "\n")
}

func benchmarkDependencySetupScript(includeNodeInstall bool) string {
	lines := []string{
		"set +e",
		"if [ -f requirements.txt ] || [ -f requirements-dev.txt ] || [ -f test-requirements.txt ] || [ -f pyproject.toml ] || [ -f setup.py ]; then",
		"  python3 -m venv .frameval-venv || true",
		"  if [ -f .frameval-venv/bin/activate ]; then . .frameval-venv/bin/activate; fi",
		"  python -m pip install --quiet --upgrade pip setuptools wheel || true",
		"  python -m pip install --quiet pytest numpy || true",
		"  python -m pip install --quiet pytz sqlparse asgiref || true",
		"  if [ -f requirements.txt ]; then python -m pip install --quiet -r requirements.txt || true; fi",
		"  if [ -f requirements-dev.txt ]; then python -m pip install --quiet -r requirements-dev.txt || true; fi",
		"  if [ -f test-requirements.txt ]; then python -m pip install --quiet -r test-requirements.txt || true; fi",
		"  for req in requirements/*.txt tests/requirements/*.txt test_requirements/*.txt; do [ -f \"$req\" ] && python -m pip install --quiet -r \"$req\" || true; done",
		"  if [ -f pyproject.toml ] || [ -f setup.py ]; then python -m pip install --quiet -e '.[test]' || python -m pip install --quiet -e . || true; fi",
		"  if [ -f setup.py ]; then python setup.py build_ext --inplace || true; fi",
		"fi",
	}
	if includeNodeInstall {
		lines = append(lines, "if [ -f package-lock.json ]; then npm ci || true; elif [ -f package.json ]; then npm install || true; fi")
	}
	lines = append(lines, "set -e")
	return strings.Join(lines, "\n")
}

func sweBenchTestCommand(repo string, tests []string) string {
	unique := uniqueTests(tests)
	if repo == "django/django" {
		labels := make([]string, 0, len(unique))
		for _, test := range unique {
			if label := djangoTestLabel(test); label != "" {
				labels = append(labels, shellQuote(label))
			}
		}
		prefix := "PYTHON=.frameval-venv/bin/python; [ -x \"$PYTHON\" ] || PYTHON=python3; \"$PYTHON\" tests/runtests.py --verbosity 2"
		if len(labels) == 0 {
			return prefix
		}
		return prefix + " " + strings.Join(labels, " ")
	}
	command := "PYTHON=.frameval-venv/bin/python; [ -x \"$PYTHON\" ] || PYTHON=python3; \"$PYTHON\" -m pytest -q"
	if len(unique) > 0 {
		quoted := make([]string, 0, len(unique))
		for _, test := range unique {
			quoted = append(quoted, shellQuote(test))
		}
		command += " " + strings.Join(quoted, " ")
	}
	return command
}

func uniqueTests(tests []string) []string {
	unique := make([]string, 0, len(tests))
	seen := map[string]bool{}
	for _, test := range tests {
		if seen[test] {
			continue
		}
		seen[test] = true
		unique = append(unique, test)
	}
	return unique
}

func djangoTestLabel(test string) string {
	trimmed := strings.TrimSpace(test)
	if trimmed == "" {
		return ""
	}
	if openIdx := strings.LastIndex(trimmed, "("); openIdx >= 0 && strings.HasSuffix(trimmed, ")") {
		name := strings.TrimSpace(trimmed[:openIdx])
		target := strings.TrimSpace(strings.TrimSuffix(trimmed[openIdx+1:], ")"))
		if name != "" && target != "" {
			return target + "." + name
		}
	}
	return trimmed
}

func collectBenchmarkFiles(root string, include func(rel string, d fs.DirEntry) bool) ([]benchmarkFile, error) {
	files := make([]benchmarkFile, 0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if !include(rel, d) {
				return filepath.SkipDir
			}
			return nil
		}
		if !include(rel, d) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		files = append(files, benchmarkFile{
			Path:       rel,
			Content:    string(content),
			Executable: info.Mode()&0o111 != 0,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect benchmark files from %s: %w", root, err)
	}
	return files, nil
}

func benchmarkMetadata(workspaceFiles []benchmarkFile, hiddenFiles []benchmarkFile, extra map[string]any) map[string]any {
	metadata := map[string]any{}
	if len(workspaceFiles) > 0 {
		metadata["workspace_files"] = workspaceFiles
	}
	if len(hiddenFiles) > 0 {
		metadata["hidden_files"] = hiddenFiles
	}
	for key, value := range extra {
		metadata[key] = value
	}
	return metadata
}

func benchmarkCategory(greenfield bool) string {
	if greenfield {
		return "greenfield"
	}
	return "brownfield"
}

func benchmarkComplexity(difficulty string) float64 {
	switch strings.ToLower(strings.TrimSpace(difficulty)) {
	case "easy":
		return 4
	case "hard":
		return 8
	default:
		return 6
	}
}

func detectCodebaseType(workspaceFiles []benchmarkFile, hiddenFiles []benchmarkFile, fallback string) string {
	counts := map[string]int{}
	for _, file := range append(append([]benchmarkFile{}, workspaceFiles...), hiddenFiles...) {
		switch filepath.Ext(file.Path) {
		case ".py":
			counts["python"]++
		case ".ts", ".tsx", ".js", ".jsx":
			counts["typescript"]++
		case ".go":
			counts["go"]++
		case ".sh":
			counts["shell"]++
		}
	}
	bestType := fallback
	bestCount := 0
	for codebaseType, count := range counts {
		if count > bestCount {
			bestType = codebaseType
			bestCount = count
		}
	}
	return bestType
}

func parseSWETestList(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var items []string
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal([]byte(trimmed), &items); err == nil {
			return filterNonEmpty(items)
		}
	}
	lines := strings.FieldsFunc(trimmed, func(r rune) bool { return r == '\n' || r == ',' })
	return filterNonEmpty(lines)
}

func filterNonEmpty(items []string) []string {
	filtered := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(strings.Trim(item, `"`))
		if trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}
	return filtered
}

func sanitizeID(value string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", "_", "-", " ", "-")
	return replacer.Replace(strings.TrimSpace(value))
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonEmptyLine(value string) string {
	for _, line := range strings.Split(value, "\n") {
		if strings.TrimSpace(line) != "" {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func fallbackString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func maxInt(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
