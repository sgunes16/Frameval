# Spec-kit catalog + multi-select extensions

**Date:** 2026-05-31
**Scope:** Engine (Go) + frontend launcher + Run Inspect. No grader, no schema (uses Project 1's `variants.harness_config_json`).

## Goal

Replace the hardcoded canonical 4-stage `speckit` harness with a small backend catalog of 6 curated spec-kit extensions (canonical, lite, tdd-first, research-first, rigorous, dual-role). The launcher exposes the catalog as a multi-select; each selected extension becomes its own variant — letting the user compare, say, canonical vs lite vs dual-role on a single task in one batch. Run Inspect shows which extension a given run used.

## Motivation

Today every `speckit` run executes the same hardcoded 4-stage workflow. The user's research question is wider than the canonical pipeline: "which spec-kit shape works better — lite, rigorous, multi-agent?" — and answering it currently requires forking the harness file. The community's spec-kit extension page lists 6+ archetypes worth comparing; bringing them in-tree (as Go data) lets the user pick any subset from the launcher and get one variant per selection without re-compiling.

A curated 6-entry catalog also fixes a UX confusion the user already flagged ("which spec-kit am I running?"): the launcher will show the name + a short description for every selectable extension, and Run Inspect will print the extension used for each completed run.

## Out of scope

- User-defined / custom spec-kit extensions via UI (catalog is curated Go data; PR to add an entry)
- Live fetch from `github.github.com/spec-kit/community/extensions.html` (local-first; offline-first design)
- Per-stage timing analysis in Inspector (specify took 3s, implement took 2m, etc.)
- Compare view side-by-side diff of extensions (the matrix expansion already lands them as adjacent variants; richer side-by-side rendering is a follow-up)
- Backfill / aliasing: pre-existing speckit experiments with NULL `harness_config_json` fail at re-run, same clean-break pattern Projects 1+2 used

## Architecture

Four layers, leaning on Projects 1 + 2's foundations:

```
catalog (Go map)  →  speckit harness (extension-aware)  →  launcher multi-select + matrix axis  →  inspector card
```

- **Storage**: nothing new. Per-variant extension id lives in `variants.harness_config_json["speckit"]["extension_id"]`.
- **Backend**: new `engine/internal/builtin/speckit/catalog.go` package with a 6-entry `SpecKitExtension` map. Old hardcoded `speckitStages` + `stagePrompt` deleted from the harness. The harness now reads the extension id from cfg, looks up the catalog, walks `extension.Stages` in order; each stage carries an optional `Role` (used by dual-role) which threads into `RunConfig.Role` so Project 2's Inspector role accent works for free.
- **API**: `GET /api/harnesses/speckit/catalog` returns the public catalog shape (id, name, description, stage names, multi-agent flag). Frontend caches via TanStack Query.
- **Frontend**: a new `<SpecKitForm>` panel plugged into `HarnessConfigPanel`. Multi-select chip list keyed on the catalog. Default selection: `canonical` (preserves today's behavior when user doesn't touch it). The launcher's matrix expansion gains a 4th axis: `tasks × executors × models × speckitExtensions`, only when speckit is among the selected harnesses; otherwise the axis collapses to `['']`.
- **Inspector**: a `<SpecKitExtensionCard>` mirrors Project 1's `<AgentInstructionsCard>` — same aside-panel placement, collapsible, shows extension name + multi-agent badge + the ordered stage list.

---

## Detailed design

### 1. Catalog data

New file: `engine/internal/builtin/speckit/catalog.go`

```go
package speckit

type Stage struct {
    Name            string // "specify", "plan", "tasks", "implement"
    SlashCommand    string // "/speckit.specify"
    PromptTemplate  string // "/speckit.specify\n\n{{TASK}}"
    Role            string // optional; non-empty only for dual-role
}

type SpecKitExtension struct {
    ID          string  // wire id, e.g. "canonical"
    Name        string  // UI display name
    Description string  // one-line summary
    Stages      []Stage // ordered
    MultiAgent  bool    // UI badge; doesn't change execution semantics
    SourceURL   string  // optional pointer to the community extension page
}
```

The curated initial 6 entries (locked during brainstorming):

| ID | Name | Stages | Multi-agent | Notes |
|---|---|---|---|---|
| `canonical` | Canonical (default) | specify → plan → tasks → implement | no | Mirrors today's hardcoded behavior; baseline |
| `lite` | Lite (2-stage) | specify → implement | no | Fast, low-ceremony comparison point |
| `tdd-first` | TDD-first | specify → tests → plan → implement → verify | no | Tests written before the plan |
| `research-first` | Research-first | research → specify → plan → tasks → implement | no | Research stage gathers context first |
| `rigorous` | Rigorous review | specify → plan → tasks → implement → review | no | Post-implement review pass |
| `dual-role` | Dual-role (multi-agent) | specify → plan → tasks → implement (architect, architect, coder, coder) | **yes** | Stage.Role set per stage; Project 2's role accent renders automatically |

Prompt templates use the same `{{TASK}}` / `{{TECHNICAL_DETAILS}}` token style Project 2's multiagent harness introduced. Concrete templates land in the catalog file at implementation time; the canonical entry preserves the exact text the old `stagePrompt` produced so existing runs are reproducible.

Public lookup helpers:

```go
func List() []SpecKitExtension                  // ordered: canonical first, then alphabetical
func Lookup(id string) (SpecKitExtension, bool) // exact-match
```

### 2. Spec-kit harness rewrite

File: `engine/internal/builtin/harness/speckit.go`

- Delete the hardcoded `speckitStages` slice and `stagePrompt` helper.
- `Setup`: read `cfg["speckit"]["extension_id"]` (string). Empty / missing → `ErrSpecKitExtensionMissing`. Unknown id → `ErrSpecKitExtensionNotFound`. On success, stash the resolved `SpecKitExtension` in `run.Metadata["speckit.extension"]`.
- Constitution handling stays — Setup still reads `task.TaskRootPath/harness_context/constitution.md` if present and writes it to `.specify/memory/constitution.md`. (Task-bundled, not user-supplied — same as today.)
- `Invoke`: walk `extension.Stages` in order. For each stage:

  ```go
  prompt := expandSpecKitPrompt(stage.PromptTemplate, map[string]string{
      "TASK":              run.Task.TaskPrompt,
      "TECHNICAL_DETAILS": run.Task.TechnicalDetail,
  })
  result, err := exec.Execute(ctx, harness.MergeConfig(run.BaseRunConfig, executor.RunConfig{
      Prompt:        prompt,
      WorkspacePath: run.Workspace.Path,
      Stage:         stage.Name,
      Role:          stage.Role, // empty for non-dual-role; populated otherwise
  }))
  ```

- ctx-cancel and per-stage failure handling preserve today's behavior (bail on cancel, accumulate / break on stage error).
- `mergeStageTranscripts` keeps its current logic — `Stage` is what it already tags; `Role` is set by `MergeConfig` so Project 2's `groupTurns` picks it up for the role accent.

New sentinels:

```go
var ErrSpecKitExtensionMissing  = errors.New("speckit harness: cfg.speckit.extension_id is empty")
var ErrSpecKitExtensionNotFound = errors.New("speckit harness: extension_id does not match any catalog entry")
```

Pure helper:

```go
func expandSpecKitPrompt(template string, vars map[string]string) string
// Replaces {{TASK}} and {{TECHNICAL_DETAILS}} literally; unknown tokens preserved.
```

(Parallel to Project 2's `expandPrompt` — kept as its own helper so the implementations stay focused.)

### 3. API endpoint

New route: `GET /api/harnesses/speckit/catalog` → JSON of `[]SpecKitExtensionPublic` where the public shape clips `Stages` to just names and slash commands (full prompt templates are server-only):

```go
type SpecKitExtensionPublic struct {
    ID          string         `json:"id"`
    Name        string         `json:"name"`
    Description string         `json:"description"`
    Stages      []StagePublic  `json:"stages"`
    MultiAgent  bool           `json:"multi_agent"`
    SourceURL   string         `json:"source_url,omitempty"`
}
type StagePublic struct {
    Name         string `json:"name"`
    SlashCommand string `json:"slash_command"`
    Role         string `json:"role,omitempty"`
}
```

Registered in `engine/internal/api/router.go` next to the other GET catalog endpoints (e.g. harnesses, executors).

### 4. Frontend launcher

**Types** (`frontend/src/lib/types.ts`):

```ts
export type SpecKitExtensionPublic = {
  id: string;
  name: string;
  description: string;
  stages: { name: string; slash_command: string; role?: string }[];
  multi_agent: boolean;
  source_url?: string;
};

export type SpecKitConfig = {
  extension_id: string;
};
```

**Hook** (`frontend/src/lib/hooks.ts`):

```ts
export function useSpecKitCatalog() {
  return useQuery({
    queryKey: ['speckit', 'catalog'],
    queryFn: () => api.get<SpecKitExtensionPublic[]>('/harnesses/speckit/catalog'),
    staleTime: 24 * 60 * 60 * 1000, // catalog is static within a process; cache for a day
  });
}
```

**New component** `frontend/src/components/launcher/SpecKitForm.tsx`:

- Calls `useSpecKitCatalog()`.
- Renders a multi-select chip list keyed by `extension.id`. Each chip shows `extension.name`; multi-agent extensions get a small "Multi-agent" `Badge` next to the name; description renders in a hover tooltip (existing `Chip title` prop).
- The state lives in the parent — the form receives `value: { extension_ids: string[] } | undefined` and emits the same shape via `onChange`. The HarnessConfigPanel switch case adapts the wire shape `{ extension_id: <one> }` per cell (per the matrix axis), but the form's local state is the multi-id list.
- Default state when `value === undefined`: a single `useEffect` emits `{ extension_ids: ['canonical'] }` on mount — same seed pattern as Project 2's MultiAgentForm.
- Footer hint: "Each selected extension becomes its own variant — N selections × M tasks × executors × models = N × M experiments."

Actual `SpecKitConfig` wire shape is per-variant (`{ extension_id: 'canonical' }`), not the launcher's multi-id list. The launcher tracks the multi-id list in its `harnessConfigs.speckit.extension_ids`; the matrix expansion converts that into per-cell single-id payloads (`harness_configs.speckit.extension_id`) right before posting.

**HarnessConfigPanel switch** (`HarnessConfigPanel.tsx`):

```tsx
case 'speckit':
  return (
    <SpecKitForm
      value={value as { extension_ids?: string[] } | undefined}
      onChange={(next) => onChange(next as unknown as Record<string, unknown>)}
    />
  );
```

### 5. Launcher matrix expansion — 4th axis

Update `frontend/src/pages/diagnostic/launch-matrix.ts`:

```ts
export interface ExpansionInput {
  taskIds: string[];
  executorIds: string[];
  modelIds: string[];
  speckitExtensions: string[]; // collapses to [''] when speckit isn't selected
}

export interface LaunchCell {
  taskId: string;
  executorId: string;
  modelId: string;
  speckitExtension: string; // empty string when not applicable
}
```

`expandLaunchMatrix` walks four nested loops, innermost = speckitExtensions. `countExperiments` multiplies by `Math.max(speckitExtensions.length, 1)`.

`frontend/src/pages/diagnostic/launch.tsx`:

```ts
const speckitConfig = harnessConfigs.speckit as { extension_ids?: string[] } | undefined;
const selectedExtensions = selectedHarnesses.includes('speckit')
  ? (speckitConfig?.extension_ids ?? [])
  : [''];

const cells = expandLaunchMatrix({
  taskIds: taskIDs,
  executorIds: selectedExecutors,
  modelIds: selectedModels,
  speckitExtensions: selectedExtensions.length > 0 ? selectedExtensions : [''],
});
```

Per-cell launch payload (single + multi paths) extends `harness_configs.speckit` to the cell's single extension:

```ts
const cellConfigs: Record<string, unknown> = { ...harnessConfigs };
if (cell.speckitExtension && cellConfigs.speckit) {
  cellConfigs.speckit = { extension_id: cell.speckitExtension };
}
```

(The launcher's multi-id list lives only in launcher state; it never goes on the wire as-is.)

Submit gate: when `speckit` is among the selected harnesses AND `extension_ids` is empty → button reads "Pick a spec-kit extension" + disabled.

Button label branch ordering (extends Project 1+2 chain):

```
Launching… > Pick a task > Pick a variant > Type agent instructions > Configure multiagent roles > Pick a spec-kit extension > Launch · ...
```

### 6. Inspector card

`frontend/src/pages/runs/inspect.tsx` — add a new collapsible card alongside the existing `AgentInstructionsCard`. Render when `variant.harness_config?.speckit?.extension_id` is present:

```tsx
{variant?.harness_config?.speckit?.extension_id && (
  <div className="mt-3">
    <SpecKitExtensionCard extensionId={String(variant.harness_config.speckit.extension_id)} />
  </div>
)}
```

`<SpecKitExtensionCard>` calls `useSpecKitCatalog()` to look up the extension and renders:
- Header: extension name + multi-agent badge if applicable
- Body: numbered list of stages with their slash commands and (when set) role tag — same styling as the other aside cards

Role accent: zero extra Inspector code. Dual-role extension's stages set `Stage.Role`, the harness passes it into `RunConfig.Role`, the transcript turns carry it through, Project 2's `groupTurns` + `TurnGroupCard` render the role badge + colored left border automatically.

---

## Data flow

```
[User picks 'speckit' chip + multi-selects N extensions in <SpecKitForm>]
     ↓
[Launcher matrix expansion] cells include speckitExtension per cell (4th axis)
     ↓
[handleLaunch] per cell, posts harness_configs.speckit = { extension_id: cell.speckitExtension }
     ↓
[Backend] variant rows persist with that config; orchestrator passes cfg to Setup
     ↓
[speckit.Setup] resolves extension via catalog.Lookup, stashes in HarnessRun.Metadata
     ↓
[speckit.Invoke] walks extension.Stages, executes each with the stage's prompt + role
     ↓
[Transcript] every ParsedTurn carries Stage and (for dual-role) Role
     ↓
[Run Inspect] SpecKitExtensionCard shows the extension; TurnGroupCard renders the role accent for dual-role runs
```

## Error handling

- Empty / missing extension_id in cfg at Setup → `ErrSpecKitExtensionMissing` (run marked failed)
- Unknown extension id → `ErrSpecKitExtensionNotFound` (likely a stale link or a deleted catalog entry; user can re-launch with a current id)
- Catalog API failure on the frontend (network blip): TanStack Query retries once, shows an inline "Could not load spec-kit catalog" message inside `<SpecKitForm>` and disables the harness chip until it loads. Launch button gate refuses to submit speckit cells in that state.
- Matrix expansion with `selectedExtensions = []` AND speckit selected: handled by the submit gate (button disabled + "Pick a spec-kit extension"). The expansion itself never produces empty cells.

## Testing

**Backend (Go, stdlib testing):**

- `engine/internal/builtin/speckit/catalog_test.go`:
  - `TestListReturnsAllEntries` — exactly 6 entries
  - `TestLookupKnownAndUnknown` — known id returns the entry; empty / "missing" id returns `_, false`
  - `TestCanonicalEntryPreservesOldStagePrompts` — locks the canonical extension's stage prompt strings against today's `stagePrompt` output (regression guard so existing data still round-trips)
- `engine/internal/builtin/harness/speckit_test.go`:
  - `TestSpecKitSetupRejectsMissingExtensionID` — cfg empty / missing key
  - `TestSpecKitSetupRejectsUnknownExtensionID` — id not in catalog
  - `TestSpecKitInvokeWalksExtensionStagesInOrder` — fake executor records stage names per call; assert they match the chosen extension
  - `TestSpecKitDualRoleSetsRoleOnRunConfig` — dual-role extension's third stage hits the executor with the configured Role string
  - `TestExpandSpecKitPrompt` — `{{TASK}}` / `{{TECHNICAL_DETAILS}}` replacement, unknown tokens preserved
- `engine/internal/api/speckit_catalog_handler_test.go`:
  - `TestSpecKitCatalogHandlerReturnsAllEntries`

**Frontend (Vitest):**

- `frontend/src/components/launcher/SpecKitForm.test.tsx`:
  - Renders 6 chips when catalog query resolves
  - Seeds `{ extension_ids: ['canonical'] }` on mount when value is undefined
  - Toggling chips updates `extension_ids` (multi-select)
  - Multi-agent badge appears on the dual-role chip
  - Shows the "Could not load" message when the query errors
- `frontend/src/pages/diagnostic/launch-matrix.test.ts`:
  - Extends the existing Vitest with cases for the new `speckitExtensions` axis (cardinality, empty collapse, deep cross-product order)

**Manual:**

- Pick speckit harness → 6 chips visible, canonical seeded
- Multi-select 3 chips (canonical, lite, dual-role) → preview reads "3 experiments × N runs each"
- Launch → 3 variants, each tagged with its extension_id; the run picker shows them distinctly
- Open Inspector on the dual-role run → SpecKitExtensionCard shows the stage list with role tags; TurnGroupCard rows have role-colored left borders + role badges
- Open Inspector on the canonical run → SpecKitExtensionCard shows the four canonical stages; no role accents (legacy non-role transcript)
- Direct curl with empty config → 202 + run fails with `ErrSpecKitExtensionMissing` in error_message

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Existing speckit experiments fail at re-run because `harness_config_json` is NULL | Acceptable per spec (clean break, Projects 1+2 pattern). User can delete or ignore historical runs. |
| Catalog data drift between code and what the community page actually lists | The 6 entries are curated archetypes, intentionally generic. If the community page diverges, the catalog still represents the comparison shapes we care about for the thesis. Updates are a PR. |
| Multi-select × matrix axis surprises users with 10+ variant counts | Existing `totalExperiments > 8` warning in the launcher fires; nothing new needed. |
| Dual-role catalog entry mis-tags stages | Locked in `catalog.go`'s test (`TestSpecKitDualRoleSetsRoleOnRunConfig`) — any future edit that breaks role tagging fails CI. |
| TanStack Query catalog cache stalls when a new entry is added in dev | `staleTime` is 24 h but `invalidateQueries(['speckit','catalog'])` on dev refresh closes the loop. Not a production concern (catalog rebuilds at server start). |

## Open questions

None — all decisions locked during brainstorming.
