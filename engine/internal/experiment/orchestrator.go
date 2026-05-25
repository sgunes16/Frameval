package experiment

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	builtinharness "github.com/mustafaselman/frameval/engine/internal/builtin/harness"
	"github.com/mustafaselman/frameval/engine/internal/diagnostic"
	"github.com/mustafaselman/frameval/engine/internal/executor"
	"github.com/mustafaselman/frameval/engine/internal/models"
	"github.com/mustafaselman/frameval/engine/internal/sandbox"
	"github.com/mustafaselman/frameval/engine/internal/storage"
	pkgexec "github.com/mustafaselman/frameval/engine/pkg/executor"
	pkgharness "github.com/mustafaselman/frameval/engine/pkg/harness"
)

type EventBroadcaster interface {
	Broadcast(eventType string, payload any)
}

type Orchestrator struct {
	store       *storage.Store
	queue       *Queue
	sandbox     *sandbox.Manager
	registry    *executor.Registry
	harnesses   *builtinharness.Registry
	grader      *GraderClient
	hub         EventBroadcaster
}

func NewOrchestrator(store *storage.Store, queue *Queue, manager *sandbox.Manager, registry *executor.Registry, harnesses *builtinharness.Registry, grader *GraderClient, hub EventBroadcaster) *Orchestrator {
	return &Orchestrator{store: store, queue: queue, sandbox: manager, registry: registry, harnesses: harnesses, grader: grader, hub: hub}
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
	// Regrade: pass the previously-stored verified test results so the
	// judge sees the real test_pass_rate (its own grade_code would otherwise
	// return 0/N — see grader_client.go:buildVerifiedTestResults).
	verified := loadVerifiedTestResults(ctx, o.store, run.ID)
	grade, err := o.grader.GradeRun(ctx, *task, artifact, *run.Transcript, verified)
	if err != nil {
		return err
	}
	// Re-grade is supposed to replace the existing grade with a fresh
	// real verdict. If the grader was unavailable and GradeRun returned
	// a synthetic fallback, refuse to overwrite the existing grade and
	// surface ErrGraderUnavailable so the HTTP handler can return 503
	// instead of silently persisting placeholder data.
	if grade.Source != models.GradeSourceGrader {
		return ErrGraderUnavailable
	}
	grade.ID = uuid.NewString()
	grade.RunID = run.ID
	return o.store.SaveGrade(ctx, grade)
}

func (o *Orchestrator) RegradeRunPayload(ctx context.Context, runID string, task models.Task, artifact *models.ArtifactVersion, transcript models.Transcript) (*models.Grade, error) {
	// JudgeConfig is populated from SQLite settings by GradeRun itself;
	// the grader falls back to its env defaults when the settings store
	// has no judge configuration or judge.enabled is not "true".
	verified := loadVerifiedTestResults(ctx, o.store, runID)
	grade, err := o.grader.GradeRun(ctx, task, artifact, transcript, verified)
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
	variant, variantErr := o.store.GetVariant(ctx, run.VariantID)
	if variantErr != nil {
		_ = o.store.UpdateRunStatus(ctx, run.ID, "failed", variantErr.Error())
		return variantErr
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
	harnessID := variant.HarnessID
	if harnessID == "" {
		harnessID = "bare"
	}
	harnessImpl, harnessErr := o.harnesses.Get(harnessID)
	if harnessErr != nil {
		_ = o.store.UpdateRunStatus(ctx, run.ID, "failed", harnessErr.Error())
		return harnessErr
	}
	workspace, beforeSnapshot, err := o.sandbox.PrepareWorkspace(ctx, *experiment, *task, artifacts)
	if err != nil {
		_ = o.store.UpdateRunStatus(ctx, run.ID, "failed", err.Error())
		return err
	}
	defer func() { _ = os.RemoveAll(workspace) }()
	_ = o.store.UpdateRunStatus(ctx, run.ID, "running", "")
	o.broadcast("run.status", map[string]any{"experiment_id": experiment.ID, "run_id": run.ID, "status": "running", "variant_id": run.VariantID, "harness_id": harnessID})

	// Harness lifecycle: Setup lays down harness-specific files (e.g. CLAUDE.md),
	// Invoke calls the executor — possibly multiple times for ralph/speckit/planner_coder —
	// and Teardown removes harness-owned files while preserving the agent's edits.
	harnessWorkspace := pkgharness.Workspace{Path: workspace}
	hRun, setupErr := harnessImpl.Setup(ctx, harnessWorkspace, *task, pkgharness.Budget{
		MaxIterations:  3,
		MaxWallSeconds: experiment.TimeoutSeconds,
	})
	if setupErr != nil {
		_ = o.store.UpdateRunStatus(ctx, run.ID, "failed", fmt.Sprintf("harness setup: %v", setupErr))
		return setupErr
	}
	// Incremental transcript streamer: each output line is appended to
	// a running buffer; on every paragraph break (blank line) the
	// buffer is re-parsed via the executor's structural parser, the
	// transcript row is UPSERTed with the latest ParsedTurns, and a
	// run.turn event is broadcast so the frontend invalidates its
	// turns query. The end-of-run SaveTranscript call OVERWRITES this
	// partial state with the full transcript (patch + output_files +
	// filesystem_diff), so no data is lost. Lock guards against
	// concurrent OnOutput invocations from the sandbox writer.
	stream := &incrementalTranscriptStreamer{
		store:        o.store,
		exec:         execImpl,
		runID:        run.ID,
		experimentID: experiment.ID,
		broadcast:    o.broadcast,
		ctx:          ctx,
	}
	hRun.BaseRunConfig = pkgexec.RunConfig{
		WorkspacePath: workspace,
		Model:         experiment.Model,
		Environment:   runEnvironment(task.ID),
		OnOutput: func(line string) {
			o.broadcastRunLog(*experiment, *run, "executor", line)
			stream.append(line)
		},
	}
	result, execErr := harnessImpl.Invoke(ctx, hRun, execImpl)
	if tdErr := harnessImpl.Teardown(ctx, hRun); tdErr != nil {
		o.broadcastRunLog(*experiment, *run, "executor", "harness teardown warning: "+tdErr.Error())
	}
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
	// Inspector V2: announce that this run's transcript is ready. The
	// frontend fetches the turns themselves via the REST endpoint added
	// in this PR (/api/runs/:id/turns); the WS event is just the signal
	// to invalidate the query cache. Keeping the payload small means
	// json.Marshal on the orchestrator goroutine stays O(1) even when a
	// pathological 500-turn run completes, and the 256-slot broadcast
	// channel doesn't fill from a single fan-out. Per-block live
	// streaming (an event per turn as the agent produces it) is a
	// separate story (#62 / #64) and will land its own contract.
	o.broadcast("run.turn", map[string]any{
		"experiment_id": experiment.ID,
		"run_id":        run.ID,
		"turn_count":    len(result.ParsedTurns),
	})
	// Run the task's verification commands inside the sandbox (which has the
	// compilers/interpreters), not in the grader container.
	testResults, passed, failed := o.runTaskVerifications(ctx, *experiment, *run, workspace, *task)
	slog.Info("grader.verifications", "run_id", run.ID, "experiment_id", experiment.ID, "passed", passed, "failed", failed)
	_ = o.store.UpdateRunStatus(ctx, run.ID, "grading", "")
	o.broadcast("run.status", map[string]any{"experiment_id": experiment.ID, "run_id": run.ID, "status": "grading", "variant_id": run.VariantID})
	o.broadcast("grading.progress", map[string]any{"run_id": run.ID, "grader": "composite", "status": "running"})
	// Phase 1: persist deterministic grade fields immediately so the frontend
	// can render the grading page without waiting for the LLM judge (which
	// can take 30-90s on free-tier models). The full grade (Phase 2) will
	// replace this row via SaveGrade's DELETE-then-INSERT semantics.
	partialTotal := passed + failed
	partialPassRate := 0.0
	if partialTotal > 0 {
		partialPassRate = float64(passed) / float64(partialTotal)
	}
	partial := models.Grade{
		ID:             uuid.NewString(),
		RunID:          run.ID,
		TestResults:    testResults,
		TestPassCount:  passed,
		TestFailCount:  failed,
		TestPassRate:   partialPassRate,
		FileStateValid: len(outputFiles) > 0,
		LintScore:      10.0,
		TypeCheckPass:  true,
		Source:         models.GradeSourceGrader,
		GradedAt:       time.Now().UTC().Format(time.RFC3339),
	}
	if err := o.store.SaveGrade(ctx, partial); err != nil {
		slog.Warn("partial grade save failed; continuing to full grade", "run_id", run.ID, "err", err)
	}
	gradeStart := time.Now()
	slog.Info("grader.start", "run_id", run.ID, "experiment_id", experiment.ID)
	// Pass the sandbox-side verification results so the judge sees the
	// real test_pass_rate. Without this, grader/code_grader.grade re-runs
	// the tests in its own tempdir (which lacks the test files) and
	// reports 0/N, causing the judge to hallucinate "tests failed"
	// rationales even for runs that actually pass.
	grade, gradeErr := o.grader.GradeRun(ctx, *task, artifact, transcript, testResults)
	if gradeErr != nil {
		slog.Error("grader.failed", "run_id", run.ID, "experiment_id", experiment.ID, "err", gradeErr, "duration_ms", time.Since(gradeStart).Milliseconds())
		_ = o.store.UpdateRunStatus(ctx, run.ID, "failed", gradeErr.Error())
		return gradeErr
	}
	slog.Info("grader.done",
		"run_id", run.ID,
		"experiment_id", experiment.ID,
		"duration_ms", time.Since(gradeStart).Milliseconds(),
		"composite", grade.CompositeScore,
		"judge_dimensions", len(grade.JudgeScores),
		"spec_compliance", grade.SpecInstructionCompliance,
		"source", grade.Source,
	)
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
	// Compute the AgentDx Diagnostic Profile — deterministic Fingerprint +
	// Symptoms + Recovery, plus optional LLM failure classification. Errors
	// in any stage degrade gracefully rather than fail the run: a missing
	// diagnostic row is better than throwing away a successful grading pass.
	o.persistDiagnostic(ctx, *experiment, *run, *task, *variant, transcript, grade, passed, failed)
	// Compare V2: refresh the cached anchor bundle so the Tape tab has
	// fresh alignment data for this experiment. Best-effort — failure
	// here only stales the cache by one run; never blocks completion.
	o.refreshExperimentAnchors(ctx, experiment.ID)
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

// persistDiagnostic runs the three deterministic extractors (Fingerprint,
// Symptoms, Recovery) and the optional LLM failure classifier, then writes
// a Diagnostic row. Best-effort — any stage that errors is logged and the
// run still completes; the goal is for the Compare view to have data.
func (o *Orchestrator) persistDiagnostic(
	ctx context.Context,
	experiment models.Experiment,
	run models.Run,
	taskRec models.Task,
	variant models.Variant,
	transcript models.Transcript,
	grade models.Grade,
	testsPassed, testsFailed int,
) {
	fp := diagnostic.NewFingerprintExtractor().Extract(transcript.ParsedTurns, diagnostic.TaskContext{
		TaskPrompt:       taskRec.TaskPrompt,
		InstructionFiles: instructionFilesForHarness(variant.HarnessID),
	})
	recovery := diagnostic.NewRecoveryAnalyzer().Analyze(transcript.ParsedTurns)
	// Symptoms.CompileFailed needs *positive* evidence of a compile-time
	// failure, not "no type checker ran". TypeCheckPass is the bool the
	// grader sets only if a type-check stage actually executed; LintScore=0
	// is "no lint configured" not "lint failed". The cheapest reliable
	// proxy is "all tests failed and at least one test ran" — that
	// matches every greenfield syntax-error scenario.
	compileFailed := testsFailed > 0 && testsPassed == 0
	outcome := diagnostic.RunOutcome{
		TestsPassed:   testsPassed,
		TestsFailed:   testsFailed,
		TestsTotal:    testsPassed + testsFailed,
		CompileFailed: compileFailed,
		FilesTouched:  filenamesOfOutputs(transcript.OutputFiles),
	}
	symptoms := diagnostic.NewSymptomExtractor().Extract(transcript.ParsedTurns, outcome, taskRec)

	rec := storage.DiagnosticRecord{
		RunID:       run.ID,
		Fingerprint: fp,
		Symptoms:    symptoms,
		Recovery:    recovery,
	}

	// LLM classifier is opt-in via FRAMEVAL_ENABLE_LLM_JUDGE=true (set in .env).
	// Until then we persist deterministic stages only and leave failure_label NULL.
	if os.Getenv("FRAMEVAL_ENABLE_LLM_JUDGE") == "true" {
		tail := transcript.RawOutput
		if len(tail) > 4000 {
			tail = tail[len(tail)-4000:]
		}
		clsResult := o.grader.ClassifyFailure(ctx, run.ID, symptoms, taskRec.Description, tail, "claude-haiku-4-5")
		if clsResult.Classification.Confidence > 0 {
			label := clsResult.Classification
			rec.FailureLabel = &label
			rec.ClassifierModel = clsResult.ClassifierModel
			rec.ClassifierLatencyMs = clsResult.LatencyMs
		}
	}

	if err := o.store.SaveDiagnostic(ctx, rec); err != nil {
		o.broadcastRunLog(experiment, run, "diagnostic", "save failed: "+err.Error())
		return
	}
	o.broadcast("diagnostic.ready", map[string]any{"run_id": run.ID, "experiment_id": experiment.ID})
}

// loadVerifiedTestResults reads the stored test results for runID off the
// last persisted grade row. Used on regrade paths so the LLM judge sees
// the same test_pass_rate the engine originally verified, not the 0/N
// the grader's own sandbox-less subprocess runner would compute. Returns
// nil if no prior grade exists or the row had no test results.
func loadVerifiedTestResults(ctx context.Context, store *storage.Store, runID string) []models.TestResult {
	prior, err := store.GetGradeByRun(ctx, runID)
	if err != nil || prior == nil {
		return nil
	}
	return prior.TestResults
}

func filenamesOfOutputs(files []models.OutputFile) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

// instructionFilesForHarness maps a harness ID to the canonical filename
// its Setup step lays down in the workspace. Used by the fingerprint's
// ContextReferenceRate dimension — without this mapping, every non-claudemd
// run scores near zero on that dimension because the extractor only knew
// to look for CLAUDE.md.
func instructionFilesForHarness(harnessID string) []string {
	switch harnessID {
	case "claudemd":
		return []string{"CLAUDE.md"}
	case "speckit":
		return []string{"AGENTS.md", "constitution.md", "spec.md"}
	case "planner_coder":
		return []string{"PLAN.md"}
	case "ralph", "bare":
		fallthrough
	default:
		// No instruction file injected; the dimension scores ~zero for these,
		// which is the correct behavior (no context to reference).
		return nil
	}
}

// ReparseRunTranscript re-runs the executor's ParseTranscript on the
// stored raw_output of a finished run and overwrites the persisted
// ParsedTurns. The orchestrator does NOT re-run the agent — this only
// regenerates the *structured view* of an already-captured output, so
// transcripts written before a parser improvement can pick up the new
// classification without a full re-execute.
//
// Picks the executor by experiment.agent_cli. Errors when the
// transcript / experiment / executor is missing; otherwise returns the
// new turn count so the caller can render a confirmation toast.
func (o *Orchestrator) ReparseRunTranscript(ctx context.Context, runID string) (int, error) {
	run, err := o.store.GetRun(ctx, runID)
	if err != nil {
		return 0, fmt.Errorf("reparse: get run: %w", err)
	}
	transcript, err := o.store.GetTranscriptByRun(ctx, runID)
	if err != nil {
		return 0, fmt.Errorf("reparse: get transcript: %w", err)
	}
	experiment, err := o.store.GetExperiment(ctx, run.ExperimentID)
	if err != nil {
		return 0, fmt.Errorf("reparse: get experiment: %w", err)
	}
	exec, err := o.registry.Get(experiment.AgentCLI)
	if err != nil {
		return 0, fmt.Errorf("reparse: resolve executor: %w", err)
	}
	turns, err := exec.ParseTranscript([]byte(transcript.RawOutput))
	if err != nil {
		return 0, fmt.Errorf("reparse: parse: %w", err)
	}
	if err := o.store.UpdateTranscriptParsedTurns(ctx, runID, turns); err != nil {
		return 0, fmt.Errorf("reparse: persist: %w", err)
	}
	// Re-anchor since structure changed.
	o.refreshExperimentAnchors(ctx, run.ExperimentID)
	return len(turns), nil
}

// incrementalTranscriptStreamer drives live Inspector V2 updates
// while a run is still in progress. The executor's OnOutput callback
// feeds it one line at a time; on every paragraph break (a blank
// line in the agent's output) the buffer is re-parsed via the
// executor's structural parser and the transcript row is UPSERTed
// with the latest ParsedTurns. A run.turn WS event then fires so
// the frontend's useTurnStream invalidates its cache and refetches
// the new turns via REST.
//
// The end-of-run SaveTranscript call OVERWRITES this partial state
// with the full transcript (patch, output_files, filesystem_diff),
// so the streaming intermediate writes never lose data — they're
// just a strictly-monotonic prefix of the final transcript.
type incrementalTranscriptStreamer struct {
	store        *storage.Store
	exec         pkgexec.AgentExecutor
	runID        string
	experimentID string
	broadcast    func(eventType string, payload any)
	ctx          context.Context

	mu           sync.Mutex
	buf          strings.Builder
	prevWasBlank bool
	lastFlushLen int
}

// append accumulates `line` and decides whether the buffer is in a
// flushable state. Two executor output styles have to be supported:
//
//   - aider prints chat-style prose where paragraphs are delimited
//     by blank lines. We flush on the first blank line ending a
//     paragraph.
//   - opencode (--format json) emits one JSON event per line, no
//     blank lines at all. Without a JSON-line trigger here, flush
//     would never fire mid-run and the Inspector would only show
//     turns after the end-of-run SaveTranscript call — i.e. all at
//     once.
//
// Both triggers gate on `buf.Len() > lastFlushLen` so empty no-op
// flushes never reach the DB.
func (s *incrementalTranscriptStreamer) append(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf.WriteString(line)
	s.buf.WriteByte('\n')
	trimmed := strings.TrimSpace(line)
	blank := trimmed == ""
	// NDJSON heuristic: a complete JSON object line — opencode emits
	// these one per event (step_start, tool_use, text, …). We don't
	// fully validate; the parser will quietly drop malformed lines
	// at flush time anyway.
	isJSONLine := strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")
	paragraphEnd := blank && !s.prevWasBlank
	if (paragraphEnd || isJSONLine) && s.buf.Len() > s.lastFlushLen {
		s.flushLocked()
	}
	s.prevWasBlank = blank
}

func (s *incrementalTranscriptStreamer) flushLocked() {
	text := s.buf.String()
	s.lastFlushLen = s.buf.Len()
	turns, err := s.exec.ParseTranscript([]byte(text))
	if err != nil {
		return
	}
	transcript := models.Transcript{
		// Leave ID empty — SaveTranscript assigns one the first
		// time and ON CONFLICT(run_id) reuses the existing row on
		// every subsequent write.
		RunID:       s.runID,
		RawOutput:   text,
		ParsedTurns: turns,
		TotalTurns:  len(turns),
		TotalTokens: len(strings.Fields(text)),
	}
	if err := s.store.SaveTranscript(s.ctx, transcript); err != nil {
		return
	}
	s.broadcast("run.turn", map[string]any{
		"experiment_id": s.experimentID,
		"run_id":        s.runID,
		"turn_count":    len(turns),
	})
}

// refreshExperimentAnchors rebuilds the cached AnchorBundle for an
// experiment and persists it to experiments.anchors_json. Called
// best-effort after each run finalize. Errors flow through slog
// (Warn) so they're visible in structured-log aggregators with the
// experiment_id key; the run continues regardless of failure —
// stale anchors are acceptable, blocked finalize is not.
func (o *Orchestrator) refreshExperimentAnchors(ctx context.Context, experimentID string) {
	log := slog.Default()
	perRun, err := o.store.ListTurnsByExperiment(ctx, experimentID)
	if err != nil {
		log.Warn("anchors refresh: list turns failed", "experiment_id", experimentID, "err", err)
		return
	}
	// ListTurnsByExperiment returns models.ParsedTurn keyed by run ID.
	// models.ParsedTurn is a type alias for executor.ParsedTurn (see
	// engine/internal/models/run.go), so this map assignment is a
	// no-op conversion at the type system level.
	pkgPerRun := make(map[string][]pkgexec.ParsedTurn, len(perRun))
	for runID, turns := range perRun {
		pkgPerRun[runID] = turns
	}
	bundle := diagnostic.BuildAnchors(pkgPerRun)
	payload, err := json.Marshal(bundle)
	if err != nil {
		log.Warn("anchors refresh: marshal failed", "experiment_id", experimentID, "err", err)
		return
	}
	if err := o.store.SetExperimentAnchors(ctx, experimentID, string(payload)); err != nil {
		log.Warn("anchors refresh: persist failed", "experiment_id", experimentID, "err", err)
	}
}

func (o *Orchestrator) refreshExperimentState(ctx context.Context, experimentID string) error {
	runs, err := o.store.ListRunsByExperiment(ctx, experimentID)
	if err != nil {
		return err
	}
	completed := 0
	active := 0
	for _, run := range runs {
		if run.Status == "completed" || run.Status == "failed" {
			completed++
		}
		if run.Status == "running" || run.Status == "grading" || run.Status == "queued" {
			active++
		}
	}
	o.broadcast("run.progress", map[string]any{"experiment_id": experimentID, "completed": completed, "total": len(runs), "active": active})
	if completed != len(runs) || len(runs) == 0 {
		return nil
	}
	// AgentDx pivot retired pairwise variant statistics in favor of the
	// per-run diagnostic profile shown on /diagnostic/compare. The legacy
	// grader.ComputeStats / stats_repo plumbing was removed in the cleanup
	// PR; experiments still transition to "completed" here so the UI
	// reflects state, but no aggregated stats are persisted.
	_ = o.store.UpdateExperimentStatus(ctx, experimentID, "completed")
	o.broadcast("experiment.complete", map[string]any{"experiment_id": experimentID})
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
	// Drop the truth-set into the workspace right before verification
	// fires. Doing it here (not in PrepareWorkspace) keeps the test
	// files off the agent's filesystem during its turn — the agent
	// can't `cat tests/test_race_fixed.py` to peek at the expected
	// outcomes and reverse-engineer a pass.
	if err := o.sandbox.MaterializeTaskTests(workspace, task); err != nil {
		slog.Warn("verification.materialize_tests_failed", "run_id", run.ID, "task_id", task.ID, "err", err)
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
	// Derive a single judge score from the map: average all dimensions when present.
	judgeScore := 0.0
	if len(grade.JudgeScores) > 0 {
		sum := 0.0
		for _, v := range grade.JudgeScores {
			sum += v
		}
		judgeScore = sum / float64(len(grade.JudgeScores))
	}
	composite := (codeScore * 0.3) + (judgeScore * 0.3) + (processScore * 0.2) + (grade.SpecInstructionCompliance * 0.2)
	return math.Round(composite*10000) / 10000
}

func runEnvironment(taskID string) map[string]string {
	return map[string]string{
		"FRAMEVAL_TASK_ID": taskID,
		"PATH":             ".frameval-venv/bin:node_modules/.bin:/root/.local/bin:/root/.cursor/bin:/root/.opencode/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"PYTHONPATH":       ".",
		"VIRTUAL_ENV":      ".frameval-venv",
	}
}

func verificationEnvironment(taskID string) map[string]string {
	return runEnvironment(taskID)
}
