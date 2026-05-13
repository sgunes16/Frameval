// Package diagnostic implements the deterministic stages of the AgentDx
// pipeline: behavioral fingerprint, symptom extraction, and recovery analysis.
//
// All three stages take a fully-parsed Transcript and produce structured
// types defined in engine/pkg/diagnostic (the public API). No LLM is involved;
// outputs are deterministic functions of the transcript alone.
//
// The LLM-driven failure classifier lives in the Python grader sidecar and is
// invoked via gRPC; see grader/failure_classifier and proto/grader.proto.
package diagnostic

import (
	"math"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mustafaselman/frameval/engine/pkg/diagnostic"
	"github.com/mustafaselman/frameval/engine/pkg/executor"
)

// FingerprintExtractor computes the 10-dimensional behavioral fingerprint
// defined in spec Appendix B. Stateless; safe to share.
type FingerprintExtractor struct{}

// NewFingerprintExtractor constructs an extractor.
func NewFingerprintExtractor() *FingerprintExtractor { return &FingerprintExtractor{} }

// TaskContext supplies the metadata an extractor needs beyond the transcript:
// the original task prompt and the names of any harness_context files that
// reference detection should look for. Both are optional — when empty the
// corresponding dimensions are conservatively scored.
type TaskContext struct {
	TaskPrompt        string
	InstructionFiles  []string // e.g., ["CLAUDE.md", "constitution.md"]
}

// Extract computes the fingerprint for the supplied transcript.
//
// All dimensions are floats in [0, 1] except RecoveryLatency which is a turn
// count (>= 0). An empty transcript returns the zero fingerprint with all
// fields at 0.
func (e *FingerprintExtractor) Extract(turns []executor.ParsedTurn, ctx TaskContext) diagnostic.Fingerprint {
	if len(turns) == 0 {
		return diagnostic.Fingerprint{}
	}
	fp := diagnostic.Fingerprint{
		PlanningDepth:        planningDepth(turns),
		ToolCallDiversity:    toolCallDiversity(turns),
		SelfValidationRate:   selfValidationRate(turns),
		BacktrackRate:        backtrackRate(turns),
		FileFocus:            fileFocus(turns),
		RecoveryLatency:      recoveryLatency(turns),
		PrematureCompletion:  prematureCompletion(turns),
		TurnEfficiency:       turnEfficiency(turns),
		ContextReferenceRate: contextReferenceRate(turns, ctx),
		IdleThinkingRatio:    idleThinkingRatio(turns),
	}
	return clampFingerprint(fp)
}

// ---------- Dimension functions ----------
//
// Each dimension has a comment explaining the heuristic it implements and how
// it maps onto the spec's Appendix B definition. Sticking to deterministic
// regex + counting keeps the output reproducible for the same transcript.

var (
	intentKeywordRE = regexp.MustCompile(`(?im)^\s*(I[' ]?ll|I will|My plan|Let me|First[,:]|Step \d|Next[,:]|Now I[' ]?ll)`)
	backtrackRE     = regexp.MustCompile(`(?i)(let me try (?:a different|another)|actually,?\s+let|undo|revert|start over|on second thought|never mind|nvm)`)
	doneClaimRE     = regexp.MustCompile(`(?i)(I[' ]?ve\s+(?:completed|finished|implemented)|task\s+(?:is\s+)?(?:complete|done|finished)|all\s+done|implementation\s+(?:is\s+)?complete)`)
	failureClaimRE  = regexp.MustCompile(`(?i)(I cannot|I[' ]?m unable|failed to|gave up|cannot proceed|stuck)`)
	toolCallRE      = regexp.MustCompile(`(?i)(?:^|\s)(?:tool|call)[: ]+([A-Za-z_][A-Za-z0-9_]*)`)
	testRunRE       = regexp.MustCompile(`(?i)\b(pytest|go test|npm test|cargo test|jest|mocha|run_tests|run tests|test_command)\b`)
	buildRE         = regexp.MustCompile(`(?i)\b(go build|npm run build|tsc|cargo build|make build)\b`)
	fileWriteRE     = regexp.MustCompile(`(?i)\b(write_file|edit_file|create_file|file_write|file_edit|str_replace)\b`)
	fileReadRE      = regexp.MustCompile(`(?i)\b(read_file|cat\b|file_read|view_file|open\s+["'])`)
	filePathRE      = regexp.MustCompile(`(?:^|[\s"'(])([./]?[a-zA-Z0-9_\-]+(?:/[a-zA-Z0-9_\-./]+)*\.[a-zA-Z]{1,6})`)
	stateChangingRE = regexp.MustCompile(`(?i)\b(write_file|edit_file|create_file|file_write|file_edit|str_replace|run_command|shell_exec|bash|sh\s+-c)\b`)
	// errorSignatureRE is the hot-path detector for "this turn carries an
	// error". Promoted out of looksLikeError so it compiles once at init
	// rather than on every turn of a long transcript.
	errorSignatureRE = regexp.MustCompile(`(?i)\b(traceback|importerror|modulenotfounderror|attributeerror|syntaxerror|exception\s*:|exit code [1-9])`)
	roleAssistant    = "assistant"
)

// planningDepth: ratio of turns that announce intent / plan structure WITHOUT
// also issuing a tool call. Pre-action narration is the signal — turns that
// say "I'll do X" while also calling a tool don't count (the action drowns
// out the plan).
func planningDepth(turns []executor.ParsedTurn) float64 {
	planning := 0
	for _, t := range turns {
		if !strings.EqualFold(t.Role, roleAssistant) {
			continue
		}
		if !intentKeywordRE.MatchString(t.Content) {
			continue
		}
		if hasToolCall(t) {
			continue
		}
		planning++
	}
	return safeDivide(planning, len(turns))
}

// toolCallDiversity: Shannon entropy of the tool-call type distribution,
// **normalized** to [0, 1] by dividing by log2(N) where N is the unique tool
// count. Spec Appendix B's table lists the range as `[0, log(N)]` (the raw
// entropy); the spec body §4.7.1 and the public Fingerprint.ToolCallDiversity
// field both expect a clamped [0, 1] value, so this implementation matches
// the latter. The Appendix B table will be corrected in a follow-up doc PR.
func toolCallDiversity(turns []executor.ParsedTurn) float64 {
	counts := map[string]int{}
	total := 0
	for _, t := range turns {
		for _, m := range toolCallRE.FindAllStringSubmatch(t.Content, -1) {
			if len(m) >= 2 {
				counts[strings.ToLower(m[1])]++
				total++
			}
		}
	}
	if total == 0 || len(counts) <= 1 {
		return 0
	}
	entropy := 0.0
	for _, c := range counts {
		p := float64(c) / float64(total)
		entropy -= p * math.Log2(p)
	}
	maxEntropy := math.Log2(float64(len(counts)))
	if maxEntropy == 0 {
		return 0
	}
	return entropy / maxEntropy
}

// selfValidationRate: (test/build/file-reread turns) / code-producing turns.
// If no code-producing turns happened, returns 0 — there was nothing to
// validate. We treat any state-changing tool call as code-producing.
func selfValidationRate(turns []executor.ParsedTurn) float64 {
	validation, producing := 0, 0
	for _, t := range turns {
		if !strings.EqualFold(t.Role, roleAssistant) && !strings.EqualFold(t.Role, "tool") {
			continue
		}
		if stateChangingRE.MatchString(t.Content) {
			producing++
		}
		if testRunRE.MatchString(t.Content) || buildRE.MatchString(t.Content) || fileReadRE.MatchString(t.Content) {
			validation++
		}
	}
	return safeDivide(validation, producing)
}

// backtrackRate: explicit-backtrack patterns / total tool calls.
// Catches "let me try a different approach", "undo", "revert", etc.
func backtrackRate(turns []executor.ParsedTurn) float64 {
	backtracks, toolCalls := 0, 0
	for _, t := range turns {
		if backtrackRE.MatchString(t.Content) {
			backtracks++
		}
		if hasToolCall(t) {
			toolCalls++
		}
	}
	if toolCalls == 0 {
		return 0
	}
	return safeDivide(backtracks, toolCalls)
}

// fileFocus: 1 - (unique files / total file ops). Higher = more focused on
// a small set of files. Lower = scattered across many files. Returns 0 when
// no file ops are detected (nothing to focus on).
func fileFocus(turns []executor.ParsedTurn) float64 {
	unique := map[string]struct{}{}
	totalOps := 0
	for _, t := range turns {
		if fileWriteRE.MatchString(t.Content) || fileReadRE.MatchString(t.Content) {
			totalOps++
		}
		for _, m := range filePathRE.FindAllStringSubmatch(t.Content, -1) {
			if len(m) >= 2 {
				unique[filepath.Clean(m[1])] = struct{}{}
			}
		}
	}
	if totalOps == 0 || len(unique) == 0 {
		return 0
	}
	ratio := float64(len(unique)) / float64(totalOps)
	if ratio > 1 {
		ratio = 1
	}
	return 1 - ratio
}

// recoveryLatency: mean number of turns from an error event to the next
// state-changing tool call by the assistant (a corrective action).
// Returns 0 when no errors are observed.
func recoveryLatency(turns []executor.ParsedTurn) float64 {
	var deltas []int
	for i, t := range turns {
		if !looksLikeError(t) {
			continue
		}
		// Walk forward looking for the next state-changing assistant turn.
		for j := i + 1; j < len(turns); j++ {
			if strings.EqualFold(turns[j].Role, roleAssistant) && stateChangingRE.MatchString(turns[j].Content) {
				deltas = append(deltas, j-i)
				break
			}
		}
	}
	if len(deltas) == 0 {
		return 0
	}
	sum := 0
	for _, d := range deltas {
		sum += d
	}
	return float64(sum) / float64(len(deltas))
}

// prematureCompletion: 1 if the agent declares done while error/failure
// markers are still present in the recent transcript, else 0. Binary metric.
func prematureCompletion(turns []executor.ParsedTurn) float64 {
	hasDoneClaim := false
	hasUnresolvedFailure := false
	for _, t := range turns {
		if doneClaimRE.MatchString(t.Content) {
			hasDoneClaim = true
		}
	}
	if !hasDoneClaim {
		return 0
	}
	// Look at the last 5 turns; if a failure signature appears there, treat
	// the "done" claim as premature.
	tail := turns
	if len(turns) > 5 {
		tail = turns[len(turns)-5:]
	}
	for _, t := range tail {
		if looksLikeError(t) || failureClaimRE.MatchString(t.Content) {
			hasUnresolvedFailure = true
		}
	}
	if hasUnresolvedFailure {
		return 1
	}
	return 0
}

// turnEfficiency: (state-changing tool calls) / total turns.
// High = every turn produces output; low = lots of idle thinking.
func turnEfficiency(turns []executor.ParsedTurn) float64 {
	stateChanges := 0
	for _, t := range turns {
		if stateChangingRE.MatchString(t.Content) {
			stateChanges++
		}
	}
	return safeDivide(stateChanges, len(turns))
}

// contextReferenceRate: fraction of turns that mention any of the
// TaskContext.InstructionFiles by name (case-insensitive). Returns 0 when
// no instruction files were configured for the task — the metric is only
// meaningful when the agent had a context file to reference.
func contextReferenceRate(turns []executor.ParsedTurn, ctx TaskContext) float64 {
	if len(ctx.InstructionFiles) == 0 {
		return 0
	}
	referenced := 0
	for _, t := range turns {
		lower := strings.ToLower(t.Content)
		for _, file := range ctx.InstructionFiles {
			if strings.Contains(lower, strings.ToLower(file)) {
				referenced++
				break
			}
		}
	}
	return safeDivide(referenced, len(turns))
}

// idleThinkingRatio: turns that produced text without any tool call.
func idleThinkingRatio(turns []executor.ParsedTurn) float64 {
	idle := 0
	for _, t := range turns {
		if strings.EqualFold(t.Role, roleAssistant) && strings.TrimSpace(t.Content) != "" && !hasToolCall(t) {
			idle++
		}
	}
	return safeDivide(idle, len(turns))
}

// ---------- Helpers ----------

func hasToolCall(t executor.ParsedTurn) bool {
	return toolCallRE.MatchString(t.Content) ||
		stateChangingRE.MatchString(t.Content) ||
		fileWriteRE.MatchString(t.Content) ||
		fileReadRE.MatchString(t.Content)
}

func looksLikeError(t executor.ParsedTurn) bool {
	c := t.Content
	if strings.EqualFold(t.Role, "tool") {
		// Tool result turns frequently carry the error payload verbatim.
		lc := strings.ToLower(c)
		if strings.Contains(lc, "error") || strings.Contains(lc, "traceback") ||
			strings.Contains(lc, "failed") || strings.Contains(lc, "exception") {
			return true
		}
	}
	return errorSignatureRE.MatchString(c)
}

func safeDivide(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

// clampFingerprint guards against numerical errors (NaN/Inf) and clamps
// each dimension into its documented range. RecoveryLatency is a turn count
// and is left unbounded above (clamped only to non-negative).
func clampFingerprint(fp diagnostic.Fingerprint) diagnostic.Fingerprint {
	clamp01 := func(v float64) float64 {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0
		}
		if v < 0 {
			return 0
		}
		if v > 1 {
			return 1
		}
		return v
	}
	fp.PlanningDepth = clamp01(fp.PlanningDepth)
	fp.ToolCallDiversity = clamp01(fp.ToolCallDiversity)
	fp.SelfValidationRate = clamp01(fp.SelfValidationRate)
	fp.BacktrackRate = clamp01(fp.BacktrackRate)
	fp.FileFocus = clamp01(fp.FileFocus)
	if math.IsNaN(fp.RecoveryLatency) || math.IsInf(fp.RecoveryLatency, 0) || fp.RecoveryLatency < 0 {
		fp.RecoveryLatency = 0
	}
	fp.PrematureCompletion = clamp01(fp.PrematureCompletion)
	fp.TurnEfficiency = clamp01(fp.TurnEfficiency)
	fp.ContextReferenceRate = clamp01(fp.ContextReferenceRate)
	fp.IdleThinkingRatio = clamp01(fp.IdleThinkingRatio)
	return fp
}
