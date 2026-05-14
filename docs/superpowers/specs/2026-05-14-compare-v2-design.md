# Compare V2 — Design

**Status:** Draft
**Date:** 2026-05-14
**Owner:** sgunes16
**Related:** [[2026-05-14-run-inspector-v2-design]], [[2026-05-12-agentdx-design]], `frontend/src/pages/diagnostic/compare.tsx`

## 1. Motivation

The current Diagnostic Compare page (181 LOC + 5 Recharts components, ~600 LOC total) shows aggregate metrics across selected runs: a behavioral-radar overlay, failure-label breakdown, recovery timeline, cost/quality scatter, and a transcript-evidence table. It tells you *that* run A and run B differ; it does not tell you *where* they differ. There is no way to see "run A used Edit on turn 7, run B used Bash; run B then hit `HAL_API` two turns later." The page treats each run as a flat profile, not a sequence of decisions.

User pain (verbatim): *"cannot compare easily the experiments."*

The five existing panels are useful summaries and stay, but Compare V2 adds a second mode — **behavioral compare** — that operates on the turn-indexed data introduced in [[2026-05-14-run-inspector-v2-design]].

## 2. Goals & non-goals

**Goals.**
- Side-by-side turn-aligned view of 2–N runs, scrollable as a single synchronized timeline.
- Visual divergence markers where runs first stop matching ("fork glyphs").
- Similarity matrix (N×N) to identify outlier runs quickly when N > 3.
- Synced replay mode: a play head advances through all selected runs by turn index at user-selected speed.
- Artifact diff: when variants differ in CLAUDE.md / spec.md / etc., show the artifact diff alongside the behavioral diff to make causal claims defensible.
- Keep the existing 5 aggregate panels as a "summary" tab; behavioral compare is a new "Tape" tab.

**Non-goals.**
- Cross-experiment comparison. Compare V2 stays within one experiment.
- LLM-based "natural language diff" of runs. Out of MVP scope; can layer on later.
- Editing or replaying runs server-side. Replay is purely UI playback over stored data.

## 3. Approach selection

### 3.1 How to align turns across runs

| Option | What it is | Trade-off | Verdict |
|---|---|---|---|
| **A. Strict index alignment** | Run A's turn 5 lines up with run B's turn 5. | Trivial. But noisy: agents diverge by turn 1, so by turn 20 the comparison is meaningless. | Reject — produces false "diffs" on every turn. |
| **B. Anchor-based alignment** ✅ | Find shared anchors between runs (same tool name + same file path + same line range) and align around them. Between anchors, render side-by-side without forcing alignment. | More logic, but matches how a human reads: "they both edited `main.go:42`, but A read the test file first." Produces meaningful fork glyphs. | **Recommended.** |
| **C. Embedding-based alignment** | Embed each turn into a vector, do Smith–Waterman on similarity scores. | Best alignment quality but adds a model dependency and per-turn embedding cost. | Defer to v3. |

Anchor-based is the sweet spot. The diagnostic pipeline already emits `files_touched` per turn (after [[2026-05-14-run-inspector-v2-design]]). Anchoring is a pure-frontend computation over data we already have.

### 3.2 Render strategy

| Option | What it is | Trade-off | Verdict |
|---|---|---|---|
| **A. Two scrolling columns, virtualised** | Lay runs in columns. Each is its own virtual list; user scrolls them independently with optional sync. | Simple. Loses alignment when columns drift. | Acceptable for ≤ 3 runs. |
| **B. Unified tape with merged turn rows** ✅ | One scrollable column where each row is a "turn group" containing all N runs' versions of that decision. Aligned by anchor. | Anchors keep rows visually grouped; non-anchored turns get their own row with the other columns greyed. Single scroll bar. | **Recommended for 2–5 runs.** |
| **C. Matrix heatmap** | NxM grid (N=runs, M=anchors), each cell is a mini-card. | Great for ≥ 6 runs; awkward for 2. | Used only when N > 5. |

Default to **B** when 2 ≤ N ≤ 5; auto-switch to **C** when N > 5.

## 4. Architecture

### 4.1 Page structure

```
/experiments/:id/compare           (existing route, refactored)
└─ <CompareShell>
   ├─ <RunPicker>                  (variant + run multi-select, max 8)
   ├─ Tabs: Summary | Tape | Matrix | Replay | Artifacts
   ├─ <SummaryTab>                 (existing 5 Recharts panels — unchanged)
   ├─ <TapeTab>                    (NEW — anchor-aligned tape)
   ├─ <MatrixTab>                  (NEW — N×N similarity matrix; only enabled when N ≥ 3)
   ├─ <ReplayTab>                  (NEW — synced playback over tape data)
   └─ <ArtifactsTab>               (NEW — per-variant artifact diff)
```

The shell stores selected runs in URL search params (`?runs=id1,id2,id3&tab=tape`) so views are linkable. TanStack Query caches turns per run (already cached by Run Inspector V2 hooks).

### 4.2 Anchor detection algorithm

Inputs: for each selected run, the array of `parent_turns` with their constituent blocks (each block has `tool_name`, `files_touched`, `tool_use_id`, plus text content).

```
build anchor list:
  for each run R:
    for each parent_turn T:
      for each tool_use block B in T:
        anchor = (B.tool_name, sorted(B.files_touched))
        register anchor → (R, T)

filter anchors to those present in ≥ 2 runs

greedy align:
  walk all selected runs in parallel by parent_turn index
  whenever the next parent_turn in every run hits the same anchor → emit aligned row
  otherwise → emit a "drift" row containing the current parent_turn from each run, marked unaligned
  if one run's pointer is at an anchor that another run has not yet reached:
    pad the lagging runs with placeholder cells until they catch up or skip the anchor
```

Edge cases:
- An anchor present in run A but not run B is still rendered; run B's column shows a "no matching action" placeholder for that row.
- A tool call without `files_touched` (e.g., `Bash`) anchors on `(tool_name, command_hash[:8])`.
- Thinking-only turns are never anchors; they ride along with the nearest preceding tool turn.

### 4.3 Fork glyphs

Between two adjacent aligned rows, if the runs took *different* unaligned paths, render a fork glyph (a small Y-shaped icon) at the row boundary. Hover the glyph → popover lists each run's unaligned turns with one-line summaries. Click → opens a drawer with the full turns side-by-side.

### 4.4 Similarity matrix (Matrix tab)

For each pair of selected runs `(A, B)`, compute:

```
similarity(A, B) = jaccard(anchors(A), anchors(B))
                 * (1 - normalized(failure_label_distance))
                 * sequence_alignment_score(turns(A), turns(B))
```

Display as an N×N grid: cells colored on a colorblind-safe scale, value 0.00–1.00 inside. Click a cell → open the Tape tab restricted to those two runs. Diagonal is always 1.00 and visually muted. This view shines when comparing 6–8 runs from a calibration sweep.

### 4.5 Replay tab

A play head (transport bar at the bottom) advances through the merged anchor list. Two playback modes:

- **Anchor-step (default).** Each click of "next" advances to the next anchored row, all runs in sync.
- **Wall-clock.** Replays using the `timestamp` field; runs that finished faster get their later blocks revealed earlier.

Keyboard: `space` toggles play/pause, `→` step, `←` step back, `J/K` jump to next/prev fork, `1–8` jumps to that run's tape column.

A speed selector (0.5×, 1×, 2×, 5×, 25×) controls playback rate. Replay does not call the backend; it simply masks unrevealed rows.

### 4.6 Artifacts tab

Pulls each run's variant artifacts via `GET /api/variants/:id/artifacts`. Renders a per-artifact-type tabbed diff (CLAUDE.md, spec.md, …) using the same diff component as Run Inspector V2. When user hovers a paragraph in artifact A, any tape row whose thinking text quotes or strongly references that paragraph gets highlighted in the Tape tab — implemented with a lightweight n-gram match (no LLM).

### 4.7 Backend touchpoints

This feature is mostly client-side once Run Inspector V2 ships its data model. The only backend additions:

1. **`GET /api/experiments/:id/runs?include=turns`** — bulk-fetch the turns of every run in an experiment in one round-trip. Today the page makes N round-trips; this batches.
2. **`GET /api/experiments/:id/anchors`** — optional precomputed anchor table, server-side, for experiments with many runs. Caches under the experiment row. Pure perf optimization; can ship without.

## 5. Error handling & edge cases

- **Selected runs span variants with different artifacts.** Artifacts tab shows each variant's stack with a clear variant header; diff is between consecutive variants in URL order.
- **One run still streaming.** Tape tab renders completed turns; the live run's column shows the typewriter caret on its active turn. Anchor detection re-runs every 2 s while live.
- **Mixed harnesses (e.g., bare vs speckit).** Stage-aware anchors: speckit's "spec" stage anchors only against speckit runs; cross-harness anchors require matching `(tool_name, files)` regardless of stage.
- **A run failed mid-execution.** Failure row gets a red border at the last completed turn; subsequent rows show the other runs only.
- **Empty `parsed_turns`** (legacy runs): Tape tab disabled with a tooltip "this run predates turn indexing — reprocess it via Settings → Maintenance → Reindex." Summary tab still works.

## 6. Testing strategy

**Unit (Vitest):**
- `useAnchorAlignment.ts`: 12 golden fixtures covering (1) identical runs, (2) prefix divergence, (3) suffix divergence, (4) mid-fork rejoin, (5) one-run-shorter, (6) Bash-without-files anchor.
- `useSimilarityMatrix.ts`: matrix symmetry, diagonal = 1.0, monotonicity under turn deletion.

**Integration (Vitest + mocked API):**
- Tape tab renders the expected fork glyphs for 3 known fixtures.
- Replay step-forward keyboard shortcut advances the play head one anchor.

**E2E (Playwright):**
- Launch a 3-run diagnostic, wait for completion, open Compare → Tape: assert at least one fork glyph; click it; assert drawer opens.
- Compare → Matrix: assert diagonal cells render as 1.00; click off-diagonal cell; assert Tape tab opens scoped to those two runs.
- Compare → Replay: press space, assert play head advances; press J, assert it lands on a fork.

**Visual regression (Playwright + percy or playwright snapshots):**
- Snapshots of Summary, Tape, Matrix, Replay, Artifacts in both light and dark mode with a fixed 3-run fixture.

## 7. Acceptance criteria

- Selecting 2 runs and switching to Tape produces an aligned timeline with at least one anchored row when the runs share any tool+file action.
- Matrix shows correct symmetric similarity scores for ≥ 3 runs.
- Replay advances all runs in lockstep at the user-selected speed; pause works.
- Artifact diff renders for runs whose variants have different artifacts.
- URL `?runs=…&tab=tape&focus=anchor:Edit:src/main.go` deep-links to that fork.
- The page loads (TTI) in under 1 second on 4 runs × 200 turns each (with the existing data already cached locally).

## 8. Out of scope / future

- Cross-experiment compare ("how did our v2 harness do vs last week's runs?").
- Statistical confidence bands on similarity scores.
- LLM-narrated diff ("Run B got stuck on the import path; Run A skipped the imports entirely.").
- Annotation: pinning a fork with a researcher comment.
