// Package diagnostic anchor builder — Compare V2 data layer.
//
// An "anchor" is a shared decision point between runs. Two runs are
// said to share an anchor when both invoked the same tool on the same
// set of files (or, for tools like Bash that don't touch files,
// against the same hashed command input). Anchors give Compare V2 a
// stable scaffold to align turn timelines around: rendering the
// matched anchor in the same column for every run, then filling in
// the diverging turns ("forks") between anchors.
//
// The builder is a pure function over ParsedTurn slices — no DB
// access, no clock — so it's trivially testable and deterministic.
// The HTTP handler and orchestrator wire it to persistence in a
// separate layer.
package diagnostic

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
)

// AnchorKey identifies the shared decision point. For file-touching
// tools (Edit, Write, etc.) it's `tool|sorted(files)`. For Bash and
// other no-file tools it's `tool|sha256(content)[:8]` — the first
// eight hex characters of the command hash is enough to disambiguate
// the common case while staying short for tooltips.
type AnchorKey string

// Anchor binds a key to the turn that produced it. ParentTurnIndex
// lets the Tape UI scroll the focused group into view directly.
type Anchor struct {
	Key             AnchorKey `json:"key"`
	TurnIndex       int       `json:"turn_index"`
	ParentTurnIndex int       `json:"parent_turn_index"`
}

// RunAnchors is the per-run anchor list, ordered by TurnIndex.
type RunAnchors struct {
	RunID   string   `json:"run_id"`
	Anchors []Anchor `json:"anchors"`
}

// AnchorBundle is the persisted shape (one entry per run in an
// experiment) serialized to `experiments.anchors_json`.
type AnchorBundle struct {
	Runs []RunAnchors `json:"runs"`
}

// BuildAnchors walks every run's turns and emits an AnchorBundle in
// stable order: runs sorted by ID, anchors within a run sorted by
// TurnIndex. Determinism is what makes the bundle safe to cache:
// re-running the builder on the same input always produces an
// identical JSON payload, so the orchestrator can skip writes when
// nothing changed.
func BuildAnchors(perRun map[string][]executor.ParsedTurn) AnchorBundle {
	runIDs := make([]string, 0, len(perRun))
	for id := range perRun {
		runIDs = append(runIDs, id)
	}
	sort.Strings(runIDs)

	bundle := AnchorBundle{Runs: make([]RunAnchors, 0, len(runIDs))}
	for _, id := range runIDs {
		bundle.Runs = append(bundle.Runs, RunAnchors{
			RunID:   id,
			Anchors: anchorsForRun(perRun[id]),
		})
	}
	return bundle
}

func anchorsForRun(turns []executor.ParsedTurn) []Anchor {
	out := make([]Anchor, 0)
	for _, t := range turns {
		// Only tool_use blocks produce anchors. The matching
		// tool_result block carries the same files_touched in some
		// adapters but emitting both would double-count the same
		// decision.
		if t.BlockKind != "tool_use" {
			continue
		}
		if t.ToolName == "" {
			continue
		}
		key := anchorKey(t)
		if key == "" {
			continue
		}
		out = append(out, Anchor{
			Key:             key,
			TurnIndex:       t.TurnIndex,
			ParentTurnIndex: t.ParentTurnIndex,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].TurnIndex < out[j].TurnIndex })
	return out
}

// anchorKey produces the canonical (tool, files-or-hash) identifier.
// For file-touching tools, files are sorted to canonicalize order.
// For tools without files_touched, the turn's Content is hashed
// because it's the closest stable signal we have to "what did this
// invocation actually do" (shell command, search pattern, etc.).
func anchorKey(t executor.ParsedTurn) AnchorKey {
	if len(t.FilesTouched) > 0 {
		files := make([]string, len(t.FilesTouched))
		copy(files, t.FilesTouched)
		sort.Strings(files)
		return AnchorKey(t.ToolName + "|" + joinComma(files))
	}
	if t.Content == "" {
		// Truly nothing to anchor on — drop. Better than producing a
		// noisy "Bash|" / "Read|" key that would falsely match every
		// other no-input turn for that tool.
		return ""
	}
	sum := sha256.Sum256([]byte(t.Content))
	return AnchorKey(t.ToolName + "|" + hex.EncodeToString(sum[:])[:8])
}

func joinComma(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += "," + p
	}
	return out
}
