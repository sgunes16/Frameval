package diagnostic

import (
	"regexp"
	"sort"
	"strings"

	"github.com/mustafaselman/frameval/engine/pkg/diagnostic"
	"github.com/mustafaselman/frameval/engine/pkg/executor"
	"github.com/mustafaselman/frameval/engine/pkg/task"
)

// SymptomExtractor compresses a finished run into the compact Symptoms packet
// the failure classifier consumes via gRPC. Deterministic: same inputs ⇒
// same packet, byte-for-byte.
type SymptomExtractor struct{}

// NewSymptomExtractor constructs an extractor. Stateless; safe to share.
func NewSymptomExtractor() *SymptomExtractor { return &SymptomExtractor{} }

// RunOutcome is the supplementary signal SymptomExtractor needs beyond the
// transcript: deterministic test results and the file-system delta the run
// produced. Both fields are optional — when absent, the corresponding
// symptoms remain zero-valued.
type RunOutcome struct {
	TestsPassed   int
	TestsFailed   int
	TestsTotal    int
	CompileFailed bool
	LintErrors    []string

	WallClockSeconds float64
	TimeoutHit       bool

	FilesTouched []string // all files touched (created + modified)
	FilesCreated []string
	FilesDeleted []string

	// ExpectedFilesModified mirrors task.yaml's expected_files_modified
	// (or equivalent), used to compute UnexpectedFilesModified for
	// brownfield SCOPE_DRIFT detection. Empty list disables the check.
	ExpectedFilesModified []string
}

// lastClaimMaxLen caps the tail-of-transcript snippet so the symptom packet
// stays under ~1 KB even on verbose runs.
const lastClaimMaxLen = 500

// Extract produces the Symptoms struct for one run.
//
// For brownfield tasks where task.yaml lists expected_files_modified,
// supply RunOutcome.ExpectedFilesModified; UnexpectedFilesModified gets
// populated with the set difference.
func (e *SymptomExtractor) Extract(turns []executor.ParsedTurn, outcome RunOutcome, _ task.Task) diagnostic.Symptoms {
	s := diagnostic.Symptoms{
		TestsPassed:      outcome.TestsPassed,
		TestsFailed:      outcome.TestsFailed,
		TestsTotal:       outcome.TestsTotal,
		CompileFailed:    outcome.CompileFailed,
		LintErrors:       outcome.LintErrors,
		FilesTouched:     dedupSorted(outcome.FilesTouched),
		FilesCreated:     dedupSorted(outcome.FilesCreated),
		FilesDeleted:     dedupSorted(outcome.FilesDeleted),
		WallClockSeconds: outcome.WallClockSeconds,
		TimeoutHit:       outcome.TimeoutHit,
	}

	s.ToolFailures = collectToolFailures(turns)
	s.LastErrorMessage = lastErrorMessage(turns)
	s.LastAssistantClaim = truncateUTF8(lastAssistantContent(turns), lastClaimMaxLen)
	s.DeclaredCompletion = anyMatches(turns, doneClaimRE)
	s.AcknowledgedFailure = anyMatches(turns, failureClaimRE)
	s.TimeToFirstError = timeToFirstError(turns)
	s.UnexpectedFilesModified = computeUnexpected(outcome.FilesTouched, outcome.ExpectedFilesModified)

	return s
}

// ---------- Helpers (deterministic, no LLM) ----------

// toolErrorRE catches the "this tool call errored" signature in tool-role
// turns. Distinct from errorSignatureRE (assistant-role traceback markers).
var toolErrorRE = regexp.MustCompile(`(?i)\b(error|failed|exception|traceback)\b`)

func collectToolFailures(turns []executor.ParsedTurn) []diagnostic.ToolFailure {
	var failures []diagnostic.ToolFailure
	for i, t := range turns {
		if !strings.EqualFold(t.Role, "tool") {
			continue
		}
		if !toolErrorRE.MatchString(t.Content) {
			continue
		}
		failures = append(failures, diagnostic.ToolFailure{
			TurnIndex: i,
			ToolName:  inferToolName(turns, i),
			Message:   truncateUTF8(strings.TrimSpace(t.Content), 200),
		})
	}
	return failures
}

// inferToolName looks one turn back for a `tool: <name>` invocation hint;
// falls back to "unknown". This is best-effort — a clean architecture
// would tag tool-result turns with the originating tool name directly,
// which is a future executor enhancement.
func inferToolName(turns []executor.ParsedTurn, i int) string {
	if i == 0 {
		return "unknown"
	}
	prev := turns[i-1]
	if m := toolCallRE.FindStringSubmatch(prev.Content); len(m) >= 2 {
		return strings.ToLower(m[1])
	}
	return "unknown"
}

func lastErrorMessage(turns []executor.ParsedTurn) string {
	for i := len(turns) - 1; i >= 0; i-- {
		t := turns[i]
		if looksLikeError(t) {
			return truncateUTF8(strings.TrimSpace(t.Content), 200)
		}
	}
	return ""
}

func lastAssistantContent(turns []executor.ParsedTurn) string {
	for i := len(turns) - 1; i >= 0; i-- {
		t := turns[i]
		if strings.EqualFold(t.Role, roleAssistant) {
			return t.Content
		}
	}
	return ""
}

func anyMatches(turns []executor.ParsedTurn, re *regexp.Regexp) bool {
	for _, t := range turns {
		if re.MatchString(t.Content) {
			return true
		}
	}
	return false
}

func timeToFirstError(turns []executor.ParsedTurn) int {
	for i, t := range turns {
		if looksLikeError(t) {
			return i
		}
	}
	return -1
}

func computeUnexpected(touched, expected []string) []string {
	if len(expected) == 0 || len(touched) == 0 {
		return nil
	}
	allowed := map[string]struct{}{}
	for _, f := range expected {
		allowed[f] = struct{}{}
	}
	var unexpected []string
	seen := map[string]struct{}{}
	for _, f := range touched {
		if _, ok := allowed[f]; ok {
			continue
		}
		if _, dup := seen[f]; dup {
			continue
		}
		seen[f] = struct{}{}
		unexpected = append(unexpected, f)
	}
	sort.Strings(unexpected)
	return unexpected
}

func dedupSorted(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// truncateUTF8 caps a string at n bytes without splitting a multi-byte rune.
// Used to keep the symptom packet under ~4 KB even on verbose runs.
func truncateUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	// Walk back to a rune boundary.
	end := n
	for end > 0 && (s[end]&0xC0) == 0x80 { // continuation byte
		end--
	}
	return s[:end]
}
