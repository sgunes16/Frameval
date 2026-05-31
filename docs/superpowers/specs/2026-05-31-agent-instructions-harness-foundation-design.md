# Agent instructions harness + per-variant config foundation

**Date:** 2026-05-31
**Scope:** Engine (Go) + frontend launcher + Run inspect. No grader changes.

## Goal

Replace the task-bundled `claudemd` harness with a user-supplied `agent_instructions` harness whose content is typed into the launcher per run. Same change installs the per-variant harness config plumbing (storage, API, launcher UI panel) that the upcoming Multi-Agent harness (Project 2) and Spec-Kit catalog (Project 3) PRs will reuse without re-inventing the wiring.

## Motivation

The current `claudemd` harness reads `task.TaskRootPath/harness_context/claudemd.md` at Setup time. That binds the CLAUDE.md content to the task, which makes the harness pointless for the thesis question — comparing how different CLAUDE.md *wordings* affect agent behavior. Today every claudemd run uses the same file. To experiment with N wordings you'd have to clone N copies of the task. That is a UX failure that has prevented us from actually running the wording comparison the thesis needs.

The same launcher needs a per-variant config mechanism for two upcoming projects:
- Multi-Agent (Project 2): user-defined N roles, each with its own prompt + executor/model assignment.
- Spec-Kit (Project 3): user picks ≥1 community extension; each selected extension is its own variant.

Without a shared plumbing pattern, each project would rebuild the same storage + API + UI infra. Project 1 lays down that pattern once.

## Out of scope

- Multi-Agent harness (Project 2)
- Spec-Kit community catalog (Project 3)
- Compare view side-by-side diff of agent_instructions content (deferred — Inspector card is enough for now)
- Backfilling `harness_config_json` for the existing 80+ experiments (NULL stays NULL; the harness rename means old claudemd runs are no longer re-runnable as-is, which is acceptable)

## Architecture

Three layers in lockstep:

```
SQLite migration 020  →  Go interface + repo + harness + API  →  React launcher panel
```

- **Storage**: new `variants.harness_config_json TEXT NULL` column.
- **API**: launch endpoints (`/diagnostic/launch` and `/diagnostic/launch-suite`) accept an optional `harness_configs: Record<string, any>` keyed by harness id (single-launch can carry one variant's worth; suite passes the same shape).
- **Harness interface**: `Setup` gets a new `cfg map[string]any` parameter. The 4 unchanged harnesses (`bare`, `ralph`, `planner_coder`, `speckit`) accept and ignore it. The renamed `agent_instructions` harness consumes `cfg["agent_instructions"]["content"]` and refuses Setup if empty.
- **Launcher UI**: a reusable `<HarnessConfigPanel>` component dispatches per-harness config forms. Inline-expands below each selected harness chip. This PR implements the `agent_instructions` case only (textarea + non-empty validation); future PRs add `multiagent`, `speckit` cases.
- **Inspector**: Run Inspect page shows a small "Agent instructions used" card pulled from the run's variant config.

## Detailed design

### 1. Schema migration

New file: `engine/internal/storage/migrations/020_variant_harness_config.sql`

```sql
-- 020_variant_harness_config.sql
--
-- Adds opaque JSON per-variant harness config. The blob is keyed by
-- harness id; each value is whatever shape that harness expects.
--
-- Today this carries `agent_instructions.content` (the user-typed
-- CLAUDE.md text). Future harnesses (multiagent, speckit) put their
-- own configs here without further schema work.

ALTER TABLE variants ADD COLUMN harness_config_json TEXT;
```

No index — every read happens by `variant_id` which already has a PK index.

### 2. Go model + repo

**`engine/internal/models/experiment.go`** (`Variant` struct):

```go
type Variant struct {
    // …existing fields…
    HarnessConfig map[string]any `json:"harness_config,omitempty"`
}
```

Add the same field to `VariantRequest`.

**`engine/internal/storage/experiment_repo.go`**:
- `CreateExperiment`'s INSERT into `variants` adds `harness_config_json` column + placeholder; the value is `marshalJSON(variantReq.HarnessConfig)` (existing helper, already used for composite_weights).
- `ListVariantsByExperiment` SELECT adds `harness_config_json` column.
- Scan helper for variants unmarshals the JSON back into `map[string]any` via the existing `unmarshalJSON` helper, default to `nil` map.

### 3. Harness interface change

**`engine/pkg/harness/harness.go`**:

```go
type Harness interface {
    Name() string
    Description() string

    // Setup prepares the workspace before agent invocation.
    // cfg carries per-variant configuration the launcher supplied,
    // keyed by harness id. A harness that doesn't need config can
    // ignore it.
    Setup(ctx context.Context, ws Workspace, t task.Task, budget Budget, cfg map[string]any) (HarnessRun, error)

    Invoke(ctx context.Context, run HarnessRun, exec executor.AgentExecutor) (*executor.RunResult, error)
    Teardown(ctx context.Context, run HarnessRun) error
}
```

The 4 unchanged harnesses get a no-op signature update: add `_ map[string]any` parameter, body unchanged.

### 4. Agent instructions harness (rename + reshape)

Rename `engine/internal/builtin/harness/claudemd.go` → `agent_instructions.go`.

```go
package harness

const (
    AgentInstructionsHarnessID     = "agent_instructions"
    agentInstructionsConfigKey     = "agent_instructions" // key inside cfg
    agentInstructionsTargetFile    = "CLAUDE.md"          // filename in workspace
)

var ErrAgentInstructionsContentMissing = errors.New(
    "agent_instructions harness: cfg.agent_instructions.content is empty; user must supply text in the launcher")

type AgentInstructions struct{}

func NewAgentInstructions() *AgentInstructions { return &AgentInstructions{} }

func (h *AgentInstructions) Name() string        { return AgentInstructionsHarnessID }
func (h *AgentInstructions) Description() string {
    return "Lay down user-supplied CLAUDE.md (typed in the launcher) into the workspace before the agent runs"
}

func (h *AgentInstructions) Setup(_ context.Context, ws harness.Workspace, t task.Task, b harness.Budget, cfg map[string]any) (harness.HarnessRun, error) {
    content, ok := extractAgentInstructionsContent(cfg)
    if !ok || strings.TrimSpace(content) == "" {
        return harness.HarnessRun{}, ErrAgentInstructionsContentMissing
    }
    dst := filepath.Join(ws.Path, agentInstructionsTargetFile)
    if _, statErr := os.Stat(dst); statErr == nil {
        return harness.HarnessRun{}, fmt.Errorf("agent_instructions harness: workspace already contains %s; refusing to overwrite", agentInstructionsTargetFile)
    } else if !errors.Is(statErr, os.ErrNotExist) {
        return harness.HarnessRun{}, fmt.Errorf("agent_instructions harness: stat dst: %w", statErr)
    }
    if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
        return harness.HarnessRun{}, fmt.Errorf("agent_instructions harness: write %s: %w", agentInstructionsTargetFile, err)
    }
    return harness.HarnessRun{
        HarnessName: h.Name(),
        Task: t, Workspace: ws, Budget: b,
        Metadata: map[string]any{"agent_instructions.owned": true},
    }, nil
}

// Invoke + Teardown stay structurally identical to the old claudemd ones,
// just with the new ID; Teardown removes CLAUDE.md only when the Setup
// metadata flag says we created it.
```

`extractAgentInstructionsContent` is a small helper that safely walks `cfg[agentInstructionsConfigKey].(map[string]any)["content"].(string)`.

**Files removed:**
- `tasks/brownfield-fix-async-race/harness_context/claudemd.md`
- `tasks/greenfield-cli-wordfreq/harness_context/claudemd.md`
- `tasks/greenfield-rate-limiter-fastapi/harness_context/claudemd.md`
- The whole `harness_context/` directories if they're empty after the delete (verify per task).

**Registry update** (`engine/internal/builtin/harness/registry.go`):
- `NewClaudeMd` → `NewAgentInstructions`
- Registered ID `"claudemd"` → `"agent_instructions"`

### 5. Orchestrator wiring

Wherever the orchestrator calls `harness.Setup(...)` it now also unmarshals the variant's `HarnessConfig` and passes it through:

```go
cfg := variant.HarnessConfig // already a map[string]any after the repo unmarshals
run, err := h.Setup(ctx, ws, task, budget, cfg)
```

Find every Setup call (likely just one or two in `engine/internal/experiment/orchestrator.go`).

### 6. Launch endpoints accept harness_configs

**Single endpoint** (`/diagnostic/launch`):

```go
type LaunchDiagnosticRequest struct {
    // …existing fields…
    HarnessConfigs map[string]any `json:"harness_configs,omitempty"`
}
```

When building each `VariantRequest`, copy `HarnessConfigs` into `VariantRequest.HarnessConfig`. (Per-variant means per-harness-cell — since the existing endpoint creates one variant per harness, all of them get the same configs map.)

**Suite endpoint** (`/diagnostic/launch-suite`): same field. Each spawned variant gets the same configs.

### 7. Launcher UI: HarnessConfigPanel + per-cell config

**New file** `frontend/src/components/launcher/HarnessConfigPanel.tsx`:

```tsx
export type HarnessConfigValue = Record<string, unknown>;

interface PanelProps {
  harnessId: string;
  value: HarnessConfigValue | undefined;
  onChange: (next: HarnessConfigValue) => void;
}

export function HarnessConfigPanel({ harnessId, value, onChange }: PanelProps) {
  switch (harnessId) {
    case 'agent_instructions':
      return <AgentInstructionsForm value={value as { content?: string } | undefined} onChange={onChange} />;
    default:
      return null;
  }
}

function AgentInstructionsForm({ value, onChange }: {
  value: { content?: string } | undefined;
  onChange: (next: HarnessConfigValue) => void;
}) {
  return (
    <div className="mt-2 rounded-md border border-border bg-bg-elev-1 p-3">
      <label className="block text-xs uppercase tracking-wider text-fg-muted">
        Agent instructions (laid down as CLAUDE.md)
      </label>
      <textarea
        className="mt-1 w-full min-h-32 rounded-md border border-border bg-bg p-2 font-mono text-xs text-fg"
        placeholder="# Project rules&#10;&#10;Keep changes focused..."
        value={value?.content ?? ''}
        onChange={(e) => onChange({ content: e.target.value })}
      />
    </div>
  );
}
```

**Launcher** (`frontend/src/pages/diagnostic/launch.tsx`):

- New state: `harnessConfigs: Record<string, HarnessConfigValue>`.
- Below the harness chip row, for every harness in `selectedHarnesses`, render `<HarnessConfigPanel harnessId={h} value={harnessConfigs[h]} onChange={(v) => setHarnessConfigs(prev => ({ ...prev, [h]: v }))} />`.
- Gate: when `selectedHarnesses` includes `'agent_instructions'`, `canSubmit` also requires `harnessConfigs.agent_instructions?.content?.trim()` to be non-empty. Tooltip on the disabled button: "Type agent instructions before launching."
- In `handleLaunch`, every per-cell launch payload includes `harness_configs: pickConfigsForCell(harnessConfigs, selectedHarnesses)` — for now this just passes through the whole `harnessConfigs` map; the per-harness narrowing matches what the backend expects.

### 8. Inspector card

`frontend/src/pages/runs/inspect.tsx` — add a small section above the transcript: if the run's variant has `harness_config.agent_instructions.content`, render a collapsible "Agent instructions" panel showing the text in a `<pre>` so the user can verify what the agent saw.

(Compare view diff: deferred. Inspector card is enough to ground individual runs.)

### 9. Naming refactor (UI label only, not just the harness id)

- Sidebar / launcher / experiments labels that said "claudemd" → "Agent instructions"
- Harness registry's UI display name uses the new label
- `harness_id` literal at the wire level is `"agent_instructions"` (single canonical id; no aliases — clean break)

## Data flow

```
[User types content in launcher's AgentInstructionsForm textarea]
     ↓
handleLaunch packs harness_configs = { agent_instructions: { content: "..." } }
     ↓
POST /api/diagnostic/launch{,-suite} per cell
     ↓
[engine] CreateExperiment writes each variant row with harness_config_json
     ↓
[orchestrator] Setup phase reads variant.HarnessConfig, calls h.Setup(..., cfg)
     ↓
[harness] AgentInstructions.Setup extracts content, writes CLAUDE.md to workspace
     ↓
agent runs; reads CLAUDE.md naturally; transcript captured
     ↓
[Run inspect page] reads variant.harness_config, shows the content above the transcript
```

## Error handling

- Empty content at launch time: UI gate blocks Launch button. If a client bypasses the gate (e.g., curl), the harness Setup returns `ErrAgentInstructionsContentMissing` and the run is marked failed.
- Migration: SQLite ADD COLUMN is metadata-only and idempotent under our migration runner. No rollback path needed for the local-first DB.
- Existing experiments: variants without `harness_config_json` (NULL) are re-runnable only with harnesses that don't need config. `agent_instructions` re-runs fail Setup with the missing-content sentinel — the user can edit the variant in the future (out of scope) or accept that old runs are historical.
- Brownfield workspace already has CLAUDE.md: harness refuses to stomp it (same guard as old claudemd had).

## Testing

**Backend (Go, stdlib testing):**

- `engine/internal/storage/experiment_repo_test.go` (existing file from PR #143) — add `TestVariantHarnessConfigRoundTrip`: create an experiment with one variant whose `HarnessConfig = {"agent_instructions": {"content": "hello"}}`, fetch back, assert deep equality.
- `engine/internal/builtin/harness/agent_instructions_test.go` (renamed from claudemd_test.go):
  - `TestSetupWritesCLAUDEMD` — cfg with content, asserts file exists with the content.
  - `TestSetupRejectsEmptyContent` — cfg empty / missing, asserts `ErrAgentInstructionsContentMissing`.
  - `TestSetupRefusesExistingCLAUDEMD` — workspace has CLAUDE.md, asserts refusal.
  - `TestTeardownRemovesOwnedFile` — Setup created file, Teardown removes it.
- `engine/internal/api/diagnostic_launch_test.go` — `TestLaunchDiagnosticPersistsHarnessConfig`: POST with `harness_configs`, fetch the resulting variant, assert config round-trip.

**Frontend (Vitest):**

- `frontend/src/components/launcher/HarnessConfigPanel.test.tsx` — render with `harnessId='agent_instructions'`, assert textarea exists and onChange fires with `{ content: "..." }`.
- `frontend/src/components/launcher/HarnessConfigPanel.test.tsx` — render with unknown id, asserts component returns null.

**Manual:**

- Launch a run with `agent_instructions` harness selected and a typed CLAUDE.md → verify it lands in the workspace (Inspector card shows it; sandbox log shows the agent referencing it).
- Try to Launch with empty textarea → button disabled, tooltip shown.
- Hit the API directly with empty harness_configs → 202 returns, run fails with the missing-content error visible in run.error_message.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| `Setup` signature change is a wide blast radius (4 harnesses + tests + orchestrator) | Mechanical signature update; each harness gets a single-line `_ map[string]any` parameter add. The Go compiler catches every missed call site. |
| Existing experiments using `claudemd` id break post-rename | Acceptable per the design: this is a clean break, no aliasing. Old experiments are historical; user accepted that bundled files are deleted. |
| `harness_configs` payload size unbounded (textarea content) | SQLite TEXT is fine up to ~1 GB per row; realistic CLAUDE.md content is <100 KB. No throttle needed. |
| Per-cell handleLaunch passing the whole harnessConfigs map sends configs the cell's harness doesn't care about | Cheap — backend just persists the map; consumers only read their own key. Future PRs can narrow if needed. |

## Open questions

None — all decisions locked during brainstorming.
