package harness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
	"github.com/mustafaselman/frameval/engine/pkg/harness"
	"github.com/mustafaselman/frameval/engine/pkg/task"
)

// defaultRalphMaxIterations is the cap when Budget.MaxIterations is unset (<= 0).
const defaultRalphMaxIterations = 8

// noProgressStreakHalt is the number of consecutive iterations whose workspace
// fingerprint matches the previous iteration's, after which Ralph declares the
// loop stuck and halts. The first iteration always runs to establish a
// baseline fingerprint; counting begins from iteration 1. With the value at 2,
// a fully idle agent halts after 3 invocations (iter 0 baseline, iter 1 first
// match, iter 2 second match → halt).
const noProgressStreakHalt = 2

// Ralph wraps a single-invocation bare-style call in a while-loop. State
// persists across iterations because the workspace directory is shared.
//
// Halt conditions, evaluated in priority order:
//  1. Context cancellation (returns ctx.Err())
//  2. Budget.MaxIterations exhausted
//  3. No-progress: workspace fingerprint unchanged for `noProgressStreakHalt`
//     consecutive iterations
//
// Test-pass halting (Budget.StopOnSuccess) requires the orchestrator to
// expose an "evaluate now" callback (run eval.sh between iterations). That
// wiring is deferred to a follow-up; Ralph in this iteration honors only
// budget + no-progress, which is the most common practical configuration.
type Ralph struct{}

// NewRalph constructs the Ralph harness. Stateless; safe to share.
func NewRalph() *Ralph { return &Ralph{} }

func (h *Ralph) Name() string { return "ralph" }

func (h *Ralph) Description() string {
	return "While-loop wrapper around bare invocation; halts on budget, ctx cancel, or no-progress"
}

func (h *Ralph) Setup(_ context.Context, ws harness.Workspace, t task.Task, b harness.Budget) (harness.HarnessRun, error) {
	if b.MaxIterations <= 0 {
		b.MaxIterations = defaultRalphMaxIterations
	}
	return harness.HarnessRun{
		HarnessName: h.Name(),
		Task:        t,
		Workspace:   ws,
		Budget:      b,
	}, nil
}

func (h *Ralph) Invoke(ctx context.Context, run harness.HarnessRun, exec executor.AgentExecutor) (*executor.RunResult, error) {
	var perIteration []*executor.RunResult
	var lastErr error
	var prevFingerprint string
	noProgressStreak := 0
	maxIter := run.Budget.MaxIterations

iterations:
	for i := 0; i < maxIter; i++ {
		select {
		case <-ctx.Done():
			lastErr = ctx.Err()
			break iterations
		default:
		}

		result, err := exec.Execute(ctx, harness.MergeConfig(run.BaseRunConfig, executor.RunConfig{
			Prompt:        ralphPrompt(run.Task, i, maxIter),
			WorkspacePath: run.Workspace.Path,
			Stage:         fmt.Sprintf("iteration-%d", i),
		}))
		if result == nil {
			result = &executor.RunResult{}
		}
		perIteration = append(perIteration, result)
		if err != nil {
			lastErr = fmt.Errorf("ralph: iteration %d: %w", i, err)
			break iterations
		}

		fp, fpErr := fingerprintDir(run.Workspace.Path)
		if fpErr != nil {
			// fingerprint failure is non-fatal; the loop continues but disables
			// the no-progress detector for safety
			continue
		}
		if fp == prevFingerprint && prevFingerprint != "" {
			noProgressStreak++
			if noProgressStreak >= noProgressStreakHalt {
				break iterations
			}
		} else {
			noProgressStreak = 0
		}
		prevFingerprint = fp
	}

	return mergeIterationTranscripts(perIteration), lastErr
}

func (h *Ralph) Teardown(_ context.Context, _ harness.HarnessRun) error { return nil }

// ralphPrompt builds the per-iteration prompt. The first iteration uses the
// task prompt verbatim; subsequent iterations prepend a continuation cue so
// the agent knows it is resuming partial work rather than starting fresh.
func ralphPrompt(t task.Task, iteration, maxIter int) string {
	if iteration == 0 {
		return t.TaskPrompt
	}
	return fmt.Sprintf(
		"You have already worked on this task in previous iterations. Continue from the current workspace state. (Iteration %d of %d.)\n\n%s",
		iteration+1, maxIter, t.TaskPrompt,
	)
}

// mergeIterationTranscripts concatenates per-iteration RunResults into one,
// stamping `--- Iteration: N ---` markers in RawOutput and `iteration-N` on
// each ParsedTurn's Stage field.
func mergeIterationTranscripts(results []*executor.RunResult) *executor.RunResult {
	merged := &executor.RunResult{}
	var b strings.Builder
	for i, r := range results {
		b.WriteString(fmt.Sprintf("--- Iteration: %d ---\n", i))
		if r != nil {
			b.WriteString(r.RawOutput)
			if !strings.HasSuffix(r.RawOutput, "\n") {
				b.WriteString("\n")
			}
			stage := fmt.Sprintf("iteration-%d", i)
			for _, turn := range r.ParsedTurns {
				turn.Stage = stage
				merged.ParsedTurns = append(merged.ParsedTurns, turn)
			}
		}
	}
	merged.RawOutput = b.String()
	return merged
}

// fingerprintDir walks the workspace and returns a SHA-256 hash of its
// stable shape: sorted list of (relative path, file size, mtime nanos)
// across all regular files. Empty workspace returns the zero-hash sentinel.
// Used to detect "agent did nothing this iteration" without reading file
// contents.
func fingerprintDir(root string) (string, error) {
	if root == "" {
		return "", errors.New("ralph: empty workspace path")
	}
	type entry struct {
		path  string
		size  int64
		mtime int64
	}
	var entries []entry
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		entries = append(entries, entry{path: rel, size: info.Size(), mtime: info.ModTime().UnixNano()})
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })

	h := sha256.New()
	for _, e := range entries {
		fmt.Fprintf(h, "%s\x00%d\x00%d\x00", e.path, e.size, e.mtime)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

