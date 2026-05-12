package experiment

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mustafaselman/frameval/engine/internal/executor"
	"github.com/mustafaselman/frameval/engine/internal/models"
	"github.com/mustafaselman/frameval/engine/internal/sandbox"
	"github.com/mustafaselman/frameval/engine/internal/storage"
)

type EventBroadcaster interface {
	Broadcast(eventType string, payload any)
}

type Orchestrator struct {
	store    *storage.Store
	queue    *Queue
	sandbox  *sandbox.Manager
	registry *executor.Registry
	grader   *GraderClient
	hub      EventBroadcaster
}

func NewOrchestrator(store *storage.Store, queue *Queue, manager *sandbox.Manager, registry *executor.Registry, grader *GraderClient, hub EventBroadcaster) *Orchestrator {
	return &Orchestrator{store: store, queue: queue, sandbox: manager, registry: registry, grader: grader, hub: hub}
}

func (o *Orchestrator) StartExperiment(ctx context.Context, experimentID string) error {
	if err := o.store.EnsureRunsForExperiment(ctx, experimentID); err != nil {
		return err
	}
	if err := o.store.UpdateExperimentStatus(ctx, experimentID, "running"); err != nil {
		return err
	}
	runs, err := o.store.ListRunnableRuns(ctx, experimentID)
	if err != nil {
		return err
	}
	for _, run := range runs {
		run := run
		o.queue.Enqueue(func(jobCtx context.Context) {
			_ = o.executeRun(jobCtx, run.ID)
		})
	}
	o.broadcast("experiment.status", map[string]any{"experiment_id": experimentID, "status": "running"})
	return nil
}

func (o *Orchestrator) RetryRun(ctx context.Context, runID string) error {
	if err := o.store.UpdateRunStatus(ctx, runID, "pending", ""); err != nil {
		return err
	}
	run, err := o.store.GetRun(ctx, runID)
	if err != nil {
		return err
	}
	o.queue.Enqueue(func(jobCtx context.Context) {
		_ = o.executeRun(jobCtx, run.ID)
	})
	return nil
}

func (o *Orchestrator) RegradeRun(ctx context.Context, runID string) error {
	run, err := o.store.GetRun(ctx, runID)
	if err != nil {
		return err
	}
	if run.Transcript == nil {
		return fmt.Errorf("run transcript not found")
	}
	experiment, err := o.store.GetExperiment(ctx, run.ExperimentID)
	if err != nil {
		return err
	}
	defer func() { _ = o.refreshExperimentState(ctx, experiment.ID) }()
	task, err := o.store.GetTask(ctx, experiment.TaskID)
	if err != nil {
		return err
	}
	artifact, _ := o.store.GetLatestArtifactByVariant(ctx, run.VariantID)
	grade, err := o.grader.GradeRun(ctx, *task, artifact, *run.Transcript)
	if err != nil {
		return err
	}
	grade.ID = uuid.NewString()
	grade.RunID = run.ID
	return o.store.SaveGrade(ctx, grade)
}

func (o *Orchestrator) RegradeRunPayload(ctx context.Context, runID string, task models.Task, artifact *models.ArtifactVersion, transcript models.Transcript) (*models.Grade, error) {
	grade, err := o.grader.GradeRun(ctx, task, artifact, transcript)
	if err != nil {
		return nil, err
	}
	grade.ID = uuid.NewString()
	grade.RunID = runID
	grade.GradedAt = time.Now().UTC().Format(time.RFC3339)
	if err := o.store.SaveGrade(ctx, grade); err != nil {
		return nil, err
	}
	return &grade, nil
}

func (o *Orchestrator) CancelExperiment(ctx context.Context, experimentID string) error {
	if err := o.store.UpdateExperimentStatus(ctx, experimentID, "cancelled"); err != nil {
		return err
	}
	o.broadcast("experiment.status", map[string]any{"experiment_id": experimentID, "status": "cancelled"})
	return nil
}

func (o *Orchestrator) QueueSnapshot() models.QueueStatus {
	return o.queue.Snapshot()
}

func (o *Orchestrator) SandboxHealth(ctx context.Context) map[string]any {
	return o.sandbox.Health(ctx)
}

func (o *Orchestrator) EstimateExperiment(ctx context.Context, experimentID string) (float64, error) {
	experiment, err := o.store.GetExperiment(ctx, experimentID)
	if err != nil {
		return 0, err
	}
	task, err := o.store.GetTask(ctx, experiment.TaskID)
	if err != nil {
		return 0, err
	}
	configs, err := o.store.ListModelConfigs(ctx)
	if err != nil {
		return 0, err
	}
	estimate := EstimateCost(*task, *experiment, configs)
	if err := o.store.SetExperimentEstimate(ctx, experimentID, estimate); err != nil {
		return 0, err
	}
	return estimate, nil
}

func (o *Orchestrator) executeRun(ctx context.Context, runID string) error {
	run, err := o.store.GetRun(ctx, runID)
	if err != nil {
		return err
	}
	experiment, err := o.store.GetExperiment(ctx, run.ExperimentID)
	if err != nil {
		return err
	}
	defer func() { _ = o.refreshExperimentState(ctx, experiment.ID) }()
	task, err := o.store.GetTask(ctx, experiment.TaskID)
	if err != nil {
		return err
	}
	artifacts, artifactErr := o.store.ListEffectiveArtifactsByVariant(ctx, run.VariantID)
	if artifactErr != nil {
		_ = o.store.UpdateRunStatus(ctx, run.ID, "failed", artifactErr.Error())
		return artifactErr
	}
	artifact, _ := o.store.GetLatestArtifactByVariant(ctx, run.VariantID)
	execImpl, err := o.registry.Get(experiment.AgentCLI)
	if err != nil {
		_ = o.store.UpdateRunStatus(ctx, run.ID, "failed", err.Error())
		return err
	}
	workspace, beforeSnapshot, err := o.sandbox.PrepareWorkspace(ctx, *experiment, *task, artifacts)
	if err != nil {
		_ = o.store.UpdateRunStatus(ctx, run.ID, "failed", err.Error())
		return err
	}
	defer func() { _ = os.RemoveAll(workspace) }()
	_ = o.store.UpdateRunStatus(ctx, run.ID, "running", "")
	o.broadcast("run.status", map[string]any{"experiment_id": experiment.ID, "run_id": run.ID, "status": "running", "variant_id": run.VariantID})
	result, execErr := execImpl.Execute(ctx, executor.RunConfig{
		WorkspacePath: workspace,
		Prompt:        task.TaskPrompt,
		Model:         experiment.Model,
		Environment:   runEnvironment(task.ID),
		OnOutput: func(line string) {
			o.broadcastRunLog(*experiment, *run, "executor", line)
		},
	})
	if result == nil {
		result = &executor.RunResult{}
	}
	if !result.StreamedOutput {
		o.broadcastLogBlock(*experiment, *run, "executor", result.RawOutput)
	}
	if execErr != nil && strings.TrimSpace(result.RawOutput) == "" {
		o.broadcastRunLog(*experiment, *run, "executor", execErr.Error())
	}
	if execErr != nil {
		transcript := models.Transcript{ID: uuid.NewString(), RunID: run.ID, RawOutput: result.RawOutput, ParsedTurns: result.ParsedTurns, TotalTurns: len(result.ParsedTurns), TotalTokens: len(strings.Fields(result.RawOutput)), CostUSD: 0}
		_ = o.store.SaveTranscript(ctx, transcript)
		_ = o.store.UpdateRunStatus(ctx, run.ID, "failed", execErr.Error())
		o.broadcast("run.status", map[string]any{"experiment_id": experiment.ID, "run_id": run.ID, "status": "failed", "variant_id": run.VariantID})
		return execErr
	}
	patch, patchErr := o.sandbox.CapturePatch(ctx, workspace)
	if patchErr != nil {
		o.broadcastRunLog(*experiment, *run, "executor", "patch capture skipped: "+patchErr.Error())
	}
	outputFiles, fileErr := o.sandbox.CollectOutputFiles(workspace)
	if fileErr != nil {
		_ = o.store.UpdateRunStatus(ctx, run.ID, "failed", fileErr.Error())
		return fileErr
	}
	transcript := models.Transcript{ID: uuid.NewString(), RunID: run.ID, RawOutput: result.RawOutput, ParsedTurns: result.ParsedTurns, TotalTurns: len(result.ParsedTurns), TotalTokens: len(strings.Fields(result.RawOutput)), CostUSD: 0, OutputFiles: outputFiles, FilesystemDiff: o.sandbox.DiffSnapshots(beforeSnapshot, outputFiles), Patch: patch}
	if err := o.store.SaveTranscript(ctx, transcript); err != nil {
		_ = o.store.UpdateRunStatus(ctx, run.ID, "failed", err.Error())
		return err
	}
	// Run the task's verification commands inside the sandbox (which has the
	// compilers/interpreters), not in the grader container.
	testResults, passed, failed := o.runTaskVerifications(ctx, *experiment, *run, workspace, *task)
	_ = o.store.UpdateRunStatus(ctx, run.ID, "grading", "")
	o.broadcast("run.status", map[string]any{"experiment_id": experiment.ID, "run_id": run.ID, "status": "grading", "variant_id": run.VariantID})
	o.broadcast("grading.progress", map[string]any{"run_id": run.ID, "grader": "composite", "status": "running"})
	grade, gradeErr := o.grader.GradeRun(ctx, *task, artifact, transcript)
	if gradeErr != nil {
		_ = o.store.UpdateRunStatus(ctx, run.ID, "failed", gradeErr.Error())
		return gradeErr
	}
	total := passed + failed
	if total > 0 {
		grade.TestResults = testResults
		grade.TestPassCount = passed
		grade.TestFailCount = failed
		grade.TestPassRate = float64(passed) / float64(total)
		grade.FileStateValid = len(outputFiles) > 0
		grade.CompositeScore = recomputeCompositeScore(grade)
	}
	grade.ID = uuid.NewString()
	grade.RunID = run.ID
	grade.GradedAt = time.Now().UTC().Format(time.RFC3339)
	if err := o.store.SaveGrade(ctx, grade); err != nil {
		_ = o.store.UpdateRunStatus(ctx, run.ID, "failed", err.Error())
		return err
	}
	status := "completed"
	errorMessage := ""
	if execErr != nil {
		status = "failed"
		errorMessage = execErr.Error()
	}
	if err := o.store.UpdateRunStatus(ctx, run.ID, status, errorMessage); err != nil {
		return err
	}
	o.broadcast("run.status", map[string]any{"experiment_id": experiment.ID, "run_id": run.ID, "status": status, "variant_id": run.VariantID})
	return nil
}

func (o *Orchestrator) refreshExperimentState(ctx context.Context, experimentID string) error {
	runs, err := o.store.ListRunsByExperiment(ctx, experimentID)
	if err != nil {
		return err
	}
	completed := 0
	active := 0
	variantGrades := map[string][]models.Grade{}
	for _, run := range runs {
		if run.Status == "completed" || run.Status == "failed" {
			completed++
		}
		if run.Status == "running" || run.Status == "grading" || run.Status == "queued" {
			active++
		}
		if run.Grade != nil {
			variantGrades[run.VariantID] = append(variantGrades[run.VariantID], *run.Grade)
		}
	}
	o.broadcast("run.progress", map[string]any{"experiment_id": experimentID, "completed": completed, "total": len(runs), "active": active})
	if completed != len(runs) || len(runs) == 0 {
		return nil
	}
	stats, err := o.grader.ComputeStats(ctx, experimentID, variantGrades)
	if err == nil {
		sort.Slice(stats, func(i, j int) bool { return stats[i].MetricName < stats[j].MetricName })
		_ = o.store.ReplaceExperimentStats(ctx, experimentID, stats)
	}
	_ = o.store.UpdateExperimentStatus(ctx, experimentID, "completed")
	o.broadcast("experiment.complete", map[string]any{"experiment_id": experimentID, "stats_summary": stats})
	return nil
}

func (o *Orchestrator) broadcast(eventType string, payload any) {
	if o.hub == nil {
		return
	}
	o.hub.Broadcast(eventType, payload)
}

// runTaskVerifications executes each task-defined test_command inside the
// sandbox (so compilers/interpreters are available) and returns the per-test
// results plus pass/fail totals. Commands that time out or exit non-zero are
// recorded as failures without aborting the run.
func (o *Orchestrator) runTaskVerifications(ctx context.Context, experiment models.Experiment, run models.Run, workspace string, task models.Task) ([]models.TestResult, int, int) {
	if err := materializeHiddenFiles(workspace, task.Metadata); err != nil {
		return []models.TestResult{{Name: "hidden test setup", Passed: false, Output: err.Error()}}, 0, 1
	}
	results := make([]models.TestResult, 0, len(task.TestCases))
	passed, failed := 0, 0
	for _, testCase := range task.TestCases {
		if strings.TrimSpace(testCase.TestCommand) == "" {
			continue
		}
		hidden := isHiddenTest(testCase)
		if hidden {
			o.broadcastRunLog(experiment, run, "verification", "Running hidden verification: "+testCase.Name)
		} else {
			o.broadcastRunLog(experiment, run, "verification", "$ "+testCase.TestCommand)
		}
		timeout := 120 * time.Second
		if testCase.TimeoutSeconds > 0 {
			timeout = time.Duration(testCase.TimeoutSeconds) * time.Second
		}
		caseCtx, cancel := context.WithTimeout(ctx, timeout)
		if strings.TrimSpace(testCase.SetupScript) != "" {
			setupOutput, setupErr := o.sandbox.RunShell(caseCtx, workspace, verificationEnvironment(task.ID), testCase.SetupScript)
			if !hidden {
				o.broadcastLogBlock(experiment, run, "verification", setupOutput)
			}
			if setupErr != nil {
				cancel()
				failed++
				if strings.TrimSpace(setupOutput) == "" {
					setupOutput = setupErr.Error()
				}
				if hidden {
					summary := "Hidden verification setup failed.\n" + strings.TrimSpace(setupOutput)
					o.broadcastRunLog(experiment, run, "verification", "Hidden verification setup failed: "+testCase.Name)
					o.broadcastLogBlock(experiment, run, "verification", setupOutput)
					results = append(results, models.TestResult{Name: testCase.Name, Passed: false, Output: summary})
				} else {
					o.broadcastRunLog(experiment, run, "verification", setupErr.Error())
					results = append(results, models.TestResult{Name: testCase.Name, Passed: false, Output: strings.TrimSpace(setupOutput)})
				}
				continue
			}
		}
		output, err := o.sandbox.RunShell(caseCtx, workspace, verificationEnvironment(task.ID), testCase.TestCommand)
		cancel()
		if !hidden {
			o.broadcastLogBlock(experiment, run, "verification", output)
		}
		ok := err == nil
		if ok {
			passed++
		} else {
			failed++
			if output == "" {
				output = err.Error()
			}
			if err != nil && !hidden {
				o.broadcastRunLog(experiment, run, "verification", err.Error())
			}
		}
		if hidden {
			summary := "Hidden verification passed."
			if !ok {
				summary = "Hidden verification failed.\n" + strings.TrimSpace(output)
				o.broadcastRunLog(experiment, run, "verification", summary+" "+testCase.Name)
				o.broadcastLogBlock(experiment, run, "verification", output)
			}
			results = append(results, models.TestResult{Name: testCase.Name, Passed: ok, Output: summary})
			continue
		}
		results = append(results, models.TestResult{Name: testCase.Name, Passed: ok, Output: strings.TrimSpace(output)})
	}
	return results, passed, failed
}

func (o *Orchestrator) broadcastLogBlock(experiment models.Experiment, run models.Run, stage string, content string) {
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		o.broadcastRunLog(experiment, run, stage, trimmed)
	}
}

func (o *Orchestrator) broadcastRunLog(experiment models.Experiment, run models.Run, stage string, line string) {
	o.broadcast("run.log", map[string]any{
		"experiment_id": experiment.ID,
		"variant_id":    run.VariantID,
		"run_id":        run.ID,
		"run_number":    run.RunNumber,
		"stage":         stage,
		"line":          line,
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
	})
}

type hiddenWorkspaceFile struct {
	Path       string `json:"path"`
	Content    string `json:"content"`
	Executable bool   `json:"executable"`
}

type hiddenWorkspaceMetadata struct {
	HiddenFiles []hiddenWorkspaceFile `json:"hidden_files"`
}

func materializeHiddenFiles(workspace string, metadata map[string]any) error {
	if len(metadata) == 0 {
		return nil
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal task metadata: %w", err)
	}
	var hidden hiddenWorkspaceMetadata
	if err := json.Unmarshal(payload, &hidden); err != nil {
		return fmt.Errorf("decode hidden workspace files: %w", err)
	}
	for _, file := range hidden.HiddenFiles {
		relativePath := filepath.Clean(file.Path)
		if relativePath == "." || relativePath == "" || strings.HasPrefix(relativePath, "..") || filepath.IsAbs(relativePath) {
			return fmt.Errorf("invalid hidden file path %q", file.Path)
		}
		targetPath := filepath.Join(workspace, relativePath)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("create hidden file dir: %w", err)
		}
		mode := os.FileMode(0o644)
		if file.Executable {
			mode = 0o755
		}
		if err := os.WriteFile(targetPath, []byte(file.Content), mode); err != nil {
			return fmt.Errorf("write hidden file %s: %w", file.Path, err)
		}
	}
	return nil
}

func isHiddenTest(testCase models.TestCase) bool {
	return strings.EqualFold(strings.TrimSpace(testCase.Visibility), "hidden")
}

func recomputeCompositeScore(grade models.Grade) float64 {
	codeScore := grade.TestPassRate * 10
	processScore := ((grade.SelfValidationRate * 0.4) + (grade.TokenEfficiency * 0.3) + (grade.ContextUtilization * 0.3)) * 10
	composite := (codeScore * 0.3) + (grade.JudgeCorrectness * 0.3) + (processScore * 0.2) + (grade.SpecInstructionCompliance * 0.2)
	return math.Round(composite*10000) / 10000
}

func runEnvironment(taskID string) map[string]string {
	return map[string]string{
		"FRAMEVAL_TASK_ID": taskID,
		"PATH":             ".frameval-venv/bin:node_modules/.bin:/root/.local/bin:/root/.cursor/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"PYTHONPATH":       ".",
		"VIRTUAL_ENV":      ".frameval-venv",
	}
}

func verificationEnvironment(taskID string) map[string]string {
	return runEnvironment(taskID)
}
