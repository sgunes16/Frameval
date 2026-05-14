# Run Inspector V2 — Design

**Status:** Draft
**Date:** 2026-05-14
**Owner:** sgunes16
**Related:** [[2026-05-12-agentdx-design]], `frontend/src/pages/experiments/monitor.tsx`, `frontend/src/components/run-monitor/log-viewer.tsx`

## 1. Motivation

The current run monitor (`monitor.tsx` + `log-viewer.tsx`, ~1000 LOC combined) flattens every assistant block — thinking, tool call, tool result, text — into an unstructured list of cards. There is no notion of a "turn" in the UI; a single agent decision (think → call Edit → see result) is split across three cards with nothing tying them together. The `transcript.patch` field is a single unified diff against the workspace baseline, not indexed by turn, so the user cannot answer "what did the agent change on turn 7?" without manually correlating tool-call cards with the global patch.

User pain (verbatim from this session): *"I couldn't follow certain thinking and changes at chat."*

Concretely, the viewer is missing five things:

1. **Turn grouping** — no container that groups `thinking + tool_use + tool_result` into one decision.
2. **Per-turn diffs** — the global patch isn't sliced; the user has to mentally diff between turns.
3. **Search and filter** — 500+ event runs require scrolling end-to-end.
4. **Forensic linkage** — diagnostic symptoms (`HAL_API`, `SCOPE_DRIFT`, …) live in a separate page; the actual turn that triggered the classification is never highlighted.
5. **Live structure** — WebSocket events stream raw text lines; the client cannot tell which turn a line belongs to.

## 2. Goals & non-goals

**Goals.**
- Restructure the timeline around *turns*. Every decision (thinking → action → result) is one expandable card.
- Show what changed in the workspace *for that turn* — a per-turn diff, computed from the tool calls that touched files.
- Searchable across thinking text, tool names, file paths, and error messages.
- Tool histogram in a sidebar that doubles as navigation (click `Edit×5` → jump to the first Edit).
- Glyph + tooltip on turns flagged by the diagnostic pipeline, with the failure-code rationale inline.
- Live runs stream into the same UI as completed runs — no special-case rendering.

**Non-goals.**
- Editing transcripts. This is read-only.
- Comparison across runs. That belongs to Compare V2.
- New diagnostic dimensions. We use what the diagnostic pipeline already emits.
- Replacing Recharts or the WebSocket transport. Both stay.

## 3. Approach selection

| Option | What it is | Trade-off | Verdict |
|---|---|---|---|
| **A. Pure client-side regrouping** | Parse `transcript.raw_output` line-by-line in React, group turns with heuristics. No backend changes. | Fastest to ship. Brittle: every executor parses differently; live streaming has no turn context. | Reject — repeats the current pain. |
| **B. Enrich `ParsedTurn` + lazy per-turn diff** ✅ | Add `turn_index`, `block_kind`, `tool_use_id`, `parent_turn_index`, `files_touched` to `ParsedTurn`. Compute per-turn diff client-side by filtering the global patch on `files_touched`. Live WS events gain a `turn_index` field. | Modest backend change (one new migration column, one struct expansion). Per-turn diff is approximate (a file edited by two turns shows the *cumulative* change in each), but matches the user's mental model. | **Recommended.** |
| **C. Full per-turn FS snapshots** | After every turn, snapshot the workspace into a `turn_snapshots` table (or content-addressed blob store). Exact per-turn diff. | Heavy: requires intercepting between turns inside the sandbox, doubles or triples DB size, requires GC. | Reject — wrong cost/benefit for MVP. |

Option B is the pragmatic answer because today's executors already emit one tool-call per file edit; *filtering the global patch by the files a turn touched* is good enough for 95% of cases. We document the edge case (same file edited across multiple turns) in the UI tooltip.

## 4. Architecture

### 4.1 Data model changes

**`engine/pkg/executor/executor.go`** — extend `ParsedTurn`:

```go
type ParsedTurn struct {
    Role             string   `json:"role"`              // existing
    Content          string   `json:"content"`           // existing
    Timestamp        string   `json:"timestamp"`         // existing
    Stage            string   `json:"stage,omitempty"`   // existing
    TurnIndex        int      `json:"turn_index"`        // NEW — monotonic across run
    BlockKind        string   `json:"block_kind"`        // NEW — "thinking" | "text" | "tool_use" | "tool_result" | "system"
    ToolUseID        string   `json:"tool_use_id,omitempty"`        // NEW — links tool_use to its tool_result
    ParentTurnIndex  int      `json:"parent_turn_index,omitempty"`  // NEW — groups thinking/tool_use/tool_result emitted in one decision
    ToolName         string   `json:"tool_name,omitempty"`          // NEW — "Edit", "Bash", "Read", …
    FilesTouched     []string `json:"files_touched,omitempty"`      // NEW — paths whose state changed because of this turn
    DurationMs       int      `json:"duration_ms,omitempty"`        // NEW — wall time to produce this block (if streamed)
    TokensIn         int      `json:"tokens_in,omitempty"`          // NEW — input tokens consumed by this block
    TokensOut        int      `json:"tokens_out,omitempty"`         // NEW — output tokens produced
}
```

Executor adapters (`aider.go`, `cursor.go`) populate the new fields as they parse. `ParentTurnIndex` is a small monotonic counter that increments once per "decision" — a group of contiguous blocks that share intent. The bare heuristic is: a `tool_use` and its matching `tool_result` (matched on `ToolUseID`) plus any `thinking` block immediately preceding the `tool_use` share a `ParentTurnIndex`. Text blocks that follow a `tool_result` and precede the next `thinking` also share that group.

**`engine/internal/storage/migrations/013_turn_index.sql`** — no column changes; the JSON is opaque to SQLite. We bump the migration only to document the version when the new fields became required, and to backfill existing transcripts with `block_kind="text"` and synthetic indices so older runs still render.

### 4.2 WebSocket protocol

Today the orchestrator broadcasts `run.progress` with `{run_id, line, timestamp, stage}`. We extend it without breaking existing consumers:

```json
{
  "type": "run.turn",                  // NEW event type
  "run_id": "abc",
  "turn_index": 7,
  "parent_turn_index": 3,
  "block_kind": "tool_use",
  "tool_name": "Edit",
  "content": "...",                    // tool input as rendered
  "tool_use_id": "tu_a1",
  "files_touched": ["src/main.go"],
  "timestamp": "2026-05-14T10:00:00Z",
  "tokens_in": 12,
  "tokens_out": 340
}
```

The old `run.progress` event remains for raw stdout lines that fall outside any parsed turn. The frontend prefers `run.turn` when both arrive for the same content.

### 4.3 Frontend architecture

```
pages/runs/[id]/inspect.tsx                  (NEW — replaces monitor.tsx for single-run view)
└─ components/run-inspector/
   ├─ run-inspector.tsx                      (root layout, 3-pane shell)
   ├─ turn-strip.tsx                         (horizontal cinema-strip at top)
   ├─ turn-list.tsx                          (vertical list, virtualized via react-virtual)
   ├─ turn-card.tsx                          (collapsible card per parent_turn_index)
   ├─ block-thinking.tsx                     (thinking block renderer)
   ├─ block-tool-use.tsx                     (tool_use renderer, with Edit/Bash/Read variants)
   ├─ block-tool-result.tsx                  (tool_result renderer)
   ├─ block-text.tsx                         (assistant prose)
   ├─ turn-diff-panel.tsx                    (per-turn diff, computed from files_touched ∩ patch)
   ├─ tool-histogram.tsx                     (sidebar; Read×12, Edit×5, …)
   ├─ symptom-glyph.tsx                      (links a turn to the diagnostic failure label)
   ├─ inspector-search.tsx                   (Cmd-K palette + filter chips)
   └─ live-cursor.tsx                        (typewriter-style streaming indicator)
└─ hooks/
   ├─ use-turns.ts                           (TanStack query: GET /api/runs/:id/turns)
   ├─ use-turn-stream.ts                     (WS subscription, normalises run.turn events)
   ├─ use-per-turn-diff.ts                   (computes filtered patch per turn, memoised)
   └─ use-turn-search.ts                     (fuzzy search index, built lazily)
```

The 3-pane shell: **(a)** turn strip across the top (60px), **(b)** vertical turn list (left, 60% width), **(c)** sticky right panel with tabs `Diff | Tool histogram | Symptoms` (40% width). The right panel reacts to the focused turn via a context provider.

### 4.4 Per-turn diff algorithm

Inputs: `transcript.patch` (unified diff against baseline), `turn.files_touched`, prior turns' `files_touched`.

Algorithm (`use-per-turn-diff.ts`):
1. Parse `transcript.patch` once into `Map<filePath, FileDiff>` (memoised).
2. For each `parent_turn`, collect the union of `files_touched` from its blocks.
3. For each file in that union:
   - If the file appears in no earlier turn's `files_touched`, show the full file diff for that file.
   - If it appears in earlier turns, show a **scoped diff**: the hunks of `FileDiff` that overlap any line range mentioned in any of *this* turn's tool calls (we already capture `lines added/removed` in tool metadata — see `log-viewer.tsx:633`). If no line range is available, fall back to highlighting the whole file diff and surface a small "shared with turn 3, turn 5" pill on the diff header.
4. Render with `react-diff-view` (existing dep candidate; otherwise inline custom).

This sidesteps the cost of true per-turn snapshots while giving the user a faithful answer 95% of the time.

### 4.5 Live streaming UX

A turn becomes "live" when its first block arrives and "closed" when either (a) a `tool_result` for the same `ParentTurnIndex` arrives, (b) the next `ParentTurnIndex` opens, or (c) the run reaches a terminal status. Live turns show a typewriter caret on the streaming block (`live-cursor.tsx`). The cinema strip shows a pulsing dot on the active turn.

### 4.6 Search & filter

`inspector-search.tsx` exposes:
- A Cmd-K palette with full-text search over thinking text, tool inputs/outputs, file paths, and error messages. Index is built lazily from the loaded turns; result rows include turn index + content snippet.
- Quick filter chips above the list: `Thinking`, `Tool use`, `Tool result`, `Errors only`, `Touched <path>`. Multiple chips are AND-combined.
- URL state — `/runs/:id/inspect?focus=7&filter=errors` makes turns deep-linkable for bug reports.

### 4.7 Symptom integration

For each `parent_turn`, we look up the run's `Diagnostic`. If any `EvidenceSpan.turn_index` falls within the parent's child block range, we render `symptom-glyph.tsx` next to the turn header: a small colored dot with the failure-code abbreviation, tooltip showing the verbatim quote and `rationale`. Clicking the glyph opens the diagnostic detail in the right panel.

## 5. Error handling & edge cases

- **Empty `parsed_turns`** (legacy runs): fall back to the existing flat raw-text view. Show a banner: "This run was captured before turn indexing was available; structured view unavailable."
- **Mismatched `tool_use_id`**: if a `tool_use` has no matching `tool_result`, render it as orphaned and tag with a warning glyph.
- **`patch` empty but `files_touched` non-empty**: this happens when the workspace has no `.git` directory. Show a "No diff captured" placeholder in the per-turn diff panel, with the file list still rendered as a flat tree.
- **Streaming gaps**: if WS drops mid-turn, the live cursor freezes; on reconnect, we fetch the full transcript via REST and reconcile (closed turns overwrite live state).
- **Multi-stage harnesses** (speckit, planner_coder): turn indices reset *per stage*. The strip groups turns by `Stage` with a small stage label.

## 6. Testing strategy

Driven by the Testing Foundation spec — see [[2026-05-14-testing-foundation-design]] for the harness setup. For this feature specifically:

**Unit (Vitest + RTL):**
- `turn-card.tsx`: renders collapsed/expanded, role-glyph color matches `block_kind`, symptom glyph only appears when diagnostic has matching span.
- `use-per-turn-diff.ts`: golden-file tests with fixture transcripts; covers the "shared file across turns" branch.
- `use-turn-search.ts`: fuzzy search returns expected ranking on a 200-turn fixture.
- `inspector-search.tsx`: chip combinations produce the expected filtered set.

**Integration (Vitest with a mocked WS server):**
- Streaming start → mid-turn drop → reconnect → reconciliation produces the same DOM state as a non-dropped run.
- Tool histogram counts match the number of `tool_use` blocks rendered.

**E2E (Playwright):**
- Launch diagnostic → wait for one run to finish → open `/runs/:id/inspect` → assert at least one turn card is visible, expand it, assert the per-turn diff panel populates.
- Search "Edit" → results panel narrows to tool-use turns with `tool_name=Edit`.
- Deep link `/runs/:id/inspect?focus=3` opens with turn 3 scrolled into view and expanded.

Coverage target for Run Inspector V2: 80% statements on `components/run-inspector/**` and `hooks/use-*`.

## 7. Migration plan

1. Land the `ParsedTurn` schema extension behind a feature flag `INSPECTOR_V2=true` on the new route `/runs/:id/inspect`. Old `/experiments/:id/monitor` stays for backwards compatibility for two weeks.
2. Backfill `block_kind`, `turn_index`, `parent_turn_index` for all existing transcripts via a one-shot script (`scripts/backfill-turn-index.go`) that re-parses `raw_output` using the executor's `ParseTranscript`. Idempotent.
3. Once 7 days of zero `/experiments/:id/monitor` traffic, redirect it to `/runs/:id/inspect`.
4. Delete `monitor.tsx`, `log-viewer.tsx`, `agent-event-viewer.tsx`.

## 8. Acceptance criteria

- A user can click any turn card and see the full thinking + tool input/output + per-turn diff in the right panel.
- A 500-turn run renders in under 200 ms (after data load) using react-virtual.
- Search "rate limit" surfaces every turn whose thinking, tool input, tool output, or file path contains the phrase, sorted by relevance.
- Tool histogram counts agree with `transcript.parsed_turns.filter(b => b.block_kind === "tool_use").length` per tool name.
- A turn flagged with `HAL_API` in the diagnostic carries the glyph + tooltip with the rationale.
- A Playwright test exercises live streaming (open inspector while a run is running, assert turns appear without page reload).
- Lighthouse Performance score ≥ 90 on a typical run (200 turns, 50 KB patch).

## 9. Out of scope / future

- Cross-run diff (Compare V2).
- Server-side full-text search (we use a client-side index sized for ≤ 2k turns).
- Editing or annotating turns.
- Exporting a turn as a shareable snippet (could be added with a Cmd-K command later).
