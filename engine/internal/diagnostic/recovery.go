package diagnostic

import (
	"regexp"
	"strings"

	"github.com/mustafaselman/frameval/engine/pkg/diagnostic"
	"github.com/mustafaselman/frameval/engine/pkg/executor"
)

// RecoveryAnalyzer walks a transcript to build an error timeline + recovery
// metrics. Deterministic; no LLM involvement. Output is fed directly into
// the RecoveryTimeline frontend component and is one of the four pieces of
// a Diagnostic Profile.
type RecoveryAnalyzer struct{}

// NewRecoveryAnalyzer constructs a fresh analyzer. Stateless; safe to share.
func NewRecoveryAnalyzer() *RecoveryAnalyzer { return &RecoveryAnalyzer{} }

// correctionWindow is the max number of turns after an error within which a
// state-changing assistant action counts as a "correction attempt". An
// error with no corrective action inside this window is classified as a
// silent skip.
const correctionWindow = 3

// recurrenceWindow is the max number of turns after a corrective action
// within which the SAME error type recurring counts as a failed correction.
const recurrenceWindow = 3

var (
	// compileErrorRE distinguishes compile-time errors from generic stderr
	// noise; used to bucket events into the typed ErrorKind enum.
	compileErrorRE = regexp.MustCompile(`(?i)\b(syntaxerror|compile (?:error|failed)|cannot compile|won't compile|fails to compile|tsc:|build failed)\b`)
	// testFailureRE catches "N tests failed" / "N failed" lines emitted by
	// pytest, go test, jest, etc.
	testFailureRE = regexp.MustCompile(`(?i)(\d+\s+failed|tests?\s+failed|FAILED\b)`)
)

// Analyze produces the RecoveryProfile for the supplied transcript.
//
// An empty or error-free transcript returns a zero profile (no events, all
// rates 0). Documented behavior — callers can distinguish "no errors
// observed" from "errors observed but never acknowledged" by checking
// `len(ErrorEvents)`.
func (a *RecoveryAnalyzer) Analyze(turns []executor.ParsedTurn) diagnostic.RecoveryProfile {
	events := collectErrorEvents(turns)
	if len(events) == 0 {
		return diagnostic.RecoveryProfile{ErrorEvents: nil}
	}

	acknowledged := 0
	correctionDeltas := 0
	correctionAttempts := 0
	correctionSuccesses := 0
	silentSkips := 0

	for _, e := range events {
		isAck := errorAcknowledgedInNextTurn(turns, e.TurnIndex)
		correctionTurn, hasCorrection := nextCorrectiveAction(turns, e.TurnIndex)

		if isAck {
			acknowledged++
		}
		if hasCorrection {
			correctionAttempts++
			correctionDeltas += correctionTurn - e.TurnIndex
			if !errorRecursWithin(turns, correctionTurn, e.Type, recurrenceWindow) {
				correctionSuccesses++
			}
		} else if !isAck {
			// Neither acknowledged in the next turn nor corrected within the
			// window — agent silently moved on.
			silentSkips++
		}
	}

	profile := diagnostic.RecoveryProfile{
		ErrorEvents:             events,
		ErrorAcknowledgmentRate: safeDivide(acknowledged, len(events)),
		SilentSkipCount:         silentSkips,
	}
	// CorrectionLatencyMean and CorrectionSuccessRate are only meaningful
	// when correctionAttempts > 0. Consumers can disambiguate "no
	// corrections attempted" from "all corrections were instantaneous and
	// successful" by checking the relationship between len(ErrorEvents),
	// SilentSkipCount, and these fields:
	//
	//   if len(ErrorEvents) - SilentSkipCount == 0 {
	//       // no corrections attempted; latency/success are meaningless
	//   }
	if correctionAttempts > 0 {
		profile.CorrectionLatencyMean = float64(correctionDeltas) / float64(correctionAttempts)
		profile.CorrectionSuccessRate = float64(correctionSuccesses) / float64(correctionAttempts)
	}
	return profile
}

// collectErrorEvents scans the transcript and emits one ErrorEvent per
// distinct error signal. Adjacent error turns (e.g., a tool result + a
// follow-up "the build failed" assistant note) are not deduped — each is
// its own event — so the timeline reflects observed surfaces, not unique
// failures.
func collectErrorEvents(turns []executor.ParsedTurn) []diagnostic.ErrorEvent {
	var events []diagnostic.ErrorEvent
	for i, t := range turns {
		kind, ok := classifyError(t)
		if !ok {
			continue
		}
		events = append(events, diagnostic.ErrorEvent{
			TurnIndex: i,
			Type:      kind,
			ToolName:  inferToolName(turns, i),
			Message:   truncateUTF8(strings.TrimSpace(t.Content), 200),
		})
	}
	return events
}

// classifyError buckets a turn's content into one of the four ErrorKind
// values, or returns (-, false) if no error signal is present.
//
// Order matters: compile errors are checked first because their stderr
// often contains the substring "error" which would otherwise be caught by
// the generic tool-failure bucket.
func classifyError(t executor.ParsedTurn) (diagnostic.ErrorKind, bool) {
	c := t.Content
	if compileErrorRE.MatchString(c) {
		return diagnostic.ErrorKindCompileError, true
	}
	if testFailureRE.MatchString(c) {
		return diagnostic.ErrorKindTestFailure, true
	}
	if strings.EqualFold(t.Role, "tool") && toolErrorRE.MatchString(c) {
		return diagnostic.ErrorKindToolFailure, true
	}
	if errorSignatureRE.MatchString(c) {
		return diagnostic.ErrorKindStderr, true
	}
	return "", false
}

// errorAcknowledgedInNextTurn is true if the FIRST assistant-role turn after
// the error references it (explicit "error"/"failed" mention, or a corrective
// idiom like "let me", "fix", "investigate"). Walks past intervening tool /
// system turns up to `correctionWindow` so tool-heavy transcripts (where a
// tool result interposes between the error and the agent's reply) are not
// undercounted as silent skips.
func errorAcknowledgedInNextTurn(turns []executor.ParsedTurn, errorIndex int) bool {
	limit := errorIndex + 1 + correctionWindow
	if limit > len(turns) {
		limit = len(turns)
	}
	for j := errorIndex + 1; j < limit; j++ {
		next := turns[j]
		if !strings.EqualFold(next.Role, roleAssistant) {
			continue
		}
		lc := strings.ToLower(next.Content)
		for _, keyword := range []string{"error", "failed", "let me", "i'll", "fix", "investigate", "looks like"} {
			if strings.Contains(lc, keyword) {
				return true
			}
		}
		// First assistant turn after the error is the one that "owns" the
		// acknowledgment opportunity. If it doesn't mention the error, we
		// don't keep scanning — later turns may be unrelated.
		return false
	}
	return false
}

// nextCorrectiveAction walks forward up to `correctionWindow` turns and
// returns the index of the first state-changing assistant action. If none
// is found in the window, returns (0, false).
func nextCorrectiveAction(turns []executor.ParsedTurn, errorIndex int) (int, bool) {
	limit := errorIndex + 1 + correctionWindow
	if limit > len(turns) {
		limit = len(turns)
	}
	for j := errorIndex + 1; j < limit; j++ {
		t := turns[j]
		if strings.EqualFold(t.Role, roleAssistant) && stateChangingRE.MatchString(t.Content) {
			return j, true
		}
	}
	return 0, false
}

// errorRecursWithin checks the `recurrenceWindow` turns AFTER correctionTurn
// for another error of the same kind. True ⇒ the correction failed.
func errorRecursWithin(turns []executor.ParsedTurn, correctionTurn int, kind diagnostic.ErrorKind, window int) bool {
	limit := correctionTurn + 1 + window
	if limit > len(turns) {
		limit = len(turns)
	}
	for j := correctionTurn + 1; j < limit; j++ {
		if k, ok := classifyError(turns[j]); ok && k == kind {
			return true
		}
	}
	return false
}
