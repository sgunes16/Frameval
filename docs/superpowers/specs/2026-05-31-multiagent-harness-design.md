# Multi-agent harness (rename planner_coder → multiagent) + Inspector role display

**Date:** 2026-05-31
**Scope:** Engine (Go) + frontend launcher + Run Inspect. No grader changes, no schema changes (uses Project 1's `variants.harness_config_json`).

## Goal

Replace the hardcoded two-role `planner_coder` harness with a user-configurable `multiagent` harness where the user defines N sequential roles (1–5), each with its own name and prompt template. Make those roles visible in Run Inspect via per-turn role badges and color-coded left-edge accents on each turn card.

## Motivation

`planner_coder` ships exactly two hardcoded prompts (planner + coder). That works for one comparison ("with vs without a planning stage") but the thesis question is wider: "does an N-stage role decomposition help, and which stages matter?". The user wants to type in 3 roles, or swap "coder" for "reviewer", or try a "critic → planner → coder" chain — none of which is possible without a recompile today.

Inspector currently has the `turn.role` field in the data model (Project 2 backend will populate it for multiagent) but renders nothing distinct for it. A user looking at a multiagent transcript today sees one undifferentiated stream of turns; per-role visual demarcation is the missing piece.

## Out of scope

- Per-role executor / model overrides (every role uses the run-level executor + model — deferred per brainstorming)
- Compare-view side-by-side rendering of role-tagged turns (deferred)
- Turn filtering / grouping by role in Inspector (chronology preserved; accent + badge is the only display change)
- Backfill / aliasing for the old `planner_coder` harness id (clean break, mirrors Project 1's `claudemd` → `agent_instructions` pattern)

---

## Architecture

Three layers in lockstep, leaning heavily on Project 1's foundations:

```
Harness rename + config-driven roles  →  Launcher panel switch case  →  Inspector role accent
```

- **Storage**: nothing new. The role list lives inside `variants.harness_config_json` under the `multiagent` key, persisted by the same machinery Project 1 introduced.
- **Backend**: rename + rewrite the harness. The new `multiagent.go` reads `cfg["multiagent"]["roles"]` (array of `{name, prompt}`), runs them sequentially, tags every emitted turn with its role, merges transcripts.
- **Frontend**: a new `<MultiAgentForm>` component plugged into `HarnessConfigPanel`'s switch. Launcher gates Launch until every role has a non-empty name + prompt.
- **Inspector**: pure-function `roleAccent(role)` mapping a role name to one of N stable palette colors; each turn card renders a small `<Badge>` plus a colored left border keyed on `turn.role`.

---

## Detailed design

### 1. Harness rename + config schema

Rename `engine/internal/builtin/harness/planner_coder.go` → `multiagent.go`. Same for `planner_coder_test.go` → `multiagent_test.go`. Registry constructor: `NewPlannerCoder` → `NewMultiAgent`. Wire id `"planner_coder"` → `"multiagent"` (no aliasing).

**Config schema** (the JSON shape the harness consumes from `cfg["multiagent"]`):

```json
{
  "roles": [
    { "name": "planner", "prompt": "You are the planner...\n\nTask:\n{{TASK}}" },
    { "name": "coder",   "prompt": "Plan from planner:\n{{PREV_OUTPUT}}\n\nTask:\n{{TASK}}" }
  ]
}
```

Constraints validated in `Setup`:
- `roles` non-empty array with ≥ 1 and ≤ 5 elements
- every `name` non-empty, matches `^[a-z][a-z0-9_]*$` (snake_case, ASCII-only — keeps log lines and color-keying predictable)
- every `name` unique within the array
- every `prompt` non-empty after `strings.TrimSpace`

Sentinel errors so the orchestrator can render meaningful messages:
- `ErrMultiAgentRolesMissing` — empty / nil config
- `ErrMultiAgentInvalidRole` — name invalid, prompt empty, duplicate, or count outside 1–5

### 2. Prompt substitution

Pure helper `expandPrompt(template string, vars map[string]string) string`:

- Replace literal `{{TASK}}` with `vars["TASK"]`
- Replace literal `{{PREV_OUTPUT}}` with `vars["PREV_OUTPUT"]` (empty string for the first role)
- Unknown `{{...}}` tokens are left untouched (the agent might be writing those itself)
- No regex engine — simple two-call `strings.ReplaceAll`. Predictable, fast, easy to unit-test.

### 3. Sequential execution + role tagging

`Invoke` walks `roles` in array order:

```go
for i, role := range roles {
    prompt := expandPrompt(role.Prompt, map[string]string{
        "TASK":        run.Task.TaskPrompt,
        "PREV_OUTPUT": prevOutput,
    })
    result, err := exec.Execute(ctx, harness.MergeConfig(run.BaseRunConfig, executor.RunConfig{
        Prompt:        prompt,
        WorkspacePath: run.Workspace.Path,
        Role:          role.Name,
        Stage:         role.Name,
    }))
    // append to merged transcript, tag turns with role.Name
    // capture prevOutput from this role's result for the next iteration
    // on ctx.Canceled/DeadlineExceeded: bail with what we have
    // on other errors: continue to next role, accumulate via fmt.Errorf("%w; %w", ...)
}
```

`prevOutput` is computed by the existing `extractPlanFromResult` helper (renamed to `extractLastAssistantOutput` since it's no longer planner-specific). The first iteration's `prevOutput` is the empty string.

Transcript merge re-uses `mergeRoleTranscripts`'s logic generalized to N roles: prepend `--- Role: <name> ---\n` before each role's raw output, walk its `ParsedTurns`, set `turn.Role = role.Name` and `turn.Stage = role.Name`, append to merged result.

### 4. Frontend types + HarnessConfigPanel case

New type in `frontend/src/lib/types.ts`:

```ts
export type MultiAgentRole = {
  name: string;
  prompt: string;
};

export type MultiAgentConfig = {
  roles: MultiAgentRole[];
};
```

`HarnessConfigPanel` (`frontend/src/components/launcher/HarnessConfigPanel.tsx`) adds a new switch case:

```tsx
case 'multiagent':
  return <MultiAgentForm value={value as MultiAgentConfig | undefined} onChange={onChange} />;
```

### 5. `<MultiAgentForm>` component

New file `frontend/src/components/launcher/MultiAgentForm.tsx`:

- Top-of-form caption: "Sequential roles — each runs after the previous one finishes. Output is captured and available to the next role via `{{PREV_OUTPUT}}`."
- A list of role rows. Each row contains:
  - `Name` text input (validates snake_case live; red border on invalid)
  - Prompt textarea (~6 lines visible, monospace)
  - Top-right of the row: small "Remove" button (disabled when there's only one role)
  - Top-right: "↑" and "↓" buttons (disabled at the top / bottom respectively) for reordering
- Below the list: a single `+ Add role` button. Disabled with the helper text "max 5 roles" when `roles.length === 5`.
- Below that: a one-line muted hint: `Available substitutions: {{TASK}}, {{PREV_OUTPUT}}.`
- **Initial state when `value` is undefined**: two roles seeded — `planner` and `coder` — pre-filled with the same prompt text the old `planner_coder` harness used. This preserves the "easy default" workflow while letting power users edit anything.

### 6. Launcher submit gate

In `frontend/src/pages/diagnostic/launch.tsx`, extend the existing `agentInstructionsReady` pattern with a parallel `multiagentReady`:

```ts
const needsMultiAgent = selectedHarnesses.includes('multiagent');
const multiagentConfig = harnessConfigs.multiagent as MultiAgentConfig | undefined;
const multiagentReady = !needsMultiAgent || validateMultiAgentConfig(multiagentConfig);
const canSubmit =
  taskIDs.length > 0 &&
  variants.length > 0 &&
  agentInstructionsReady &&
  multiagentReady &&
  !launch.isPending;
```

`validateMultiAgentConfig` returns `true` iff `roles` has 1–5 entries with non-empty trimmed name + prompt and all names match the snake_case regex and are unique.

Button label adds a `'Configure multiagent roles'` branch ahead of the existing chain.

### 7. Inspector role accent + badge

`frontend/src/pages/runs/inspect.tsx` (or whichever child component renders `<TurnCard>` — verify during implementation):

- Each turn card's outer wrapper picks up `roleAccent(turn.role)` for its left-edge border class.
- Card header gets a small `<Badge tone="neutral">{turn.role}</Badge>` rendered iff `turn.role` is non-empty.

Pure helper in a new file `frontend/src/pages/runs/role-accent.ts`:

```ts
const PALETTE = [
  'border-l-info-fg/50',
  'border-l-success-fg/50',
  'border-l-warning-fg/50',
  'border-l-accent-fg/50',
  'border-l-fg-muted/50',
];

export function roleAccent(role: string | undefined): string {
  if (!role) return 'border-l-border';
  let h = 0;
  for (let i = 0; i < role.length; i++) h = (h * 31 + role.charCodeAt(i)) >>> 0;
  return PALETTE[h % PALETTE.length];
}
```

Stable: same role name → same color across all runs and pages. No new tokens introduced (re-uses existing semantic color tokens with alpha modifier).

---

## Data flow

```
[User builds N roles in launcher's MultiAgentForm]
     ↓
handleLaunch packs harness_configs.multiagent = { roles: [...] } into every cell's payload
     ↓
POST /api/diagnostic/launch{,-suite} → variants persisted with harness_config_json
     ↓
[orchestrator] Setup phase reads variant.HarnessConfig, calls multiagent.Setup(..., cfg)
     ↓
[multiagent harness] Validates roles. Loops:
    - expandPrompt with TASK + PREV_OUTPUT
    - exec.Execute with RunConfig.Role / RunConfig.Stage = role.Name
    - extract PREV_OUTPUT for the next role
    - tag all emitted ParsedTurns with role.Name
     ↓
Merged transcript persisted; ParsedTurn.role populated per turn
     ↓
[Run Inspect] reads transcript; each turn card renders the role badge + color accent
```

## Error handling

- Empty / malformed `multiagent` config at Setup → sentinel error, run marked failed, error surfaces in `run.error_message`.
- Mid-loop context cancellation → bail with the partial merged transcript (caller decides what to do with the partial run).
- Mid-loop role failure (non-cancel) → continue to next role with `prevOutput = "(role <name> failed; proceeding without its output)"`. Accumulate errors via `fmt.Errorf("multiagent: role %q: %w; ...", ...)` so the run's final error message lists every failing role.
- Launcher UI: button stays disabled with a tooltip when config invalid; the user cannot submit garbage in normal flow.

## Testing

**Backend (Go, stdlib testing):**

- `engine/internal/builtin/harness/multiagent_test.go`:
  - `TestMultiAgentNameAndDescription`
  - `TestSetupRejectsMissingRoles` (nil cfg, missing key, empty array)
  - `TestSetupRejectsInvalidRoleNames` (uppercase, non-ASCII, leading digit)
  - `TestSetupRejectsDuplicateNames`
  - `TestSetupRejectsEmptyPrompt`
  - `TestSetupRejectsMoreThanFiveRoles`
  - `TestInvokeRunsRolesSequentially` — fake executor records the prompts it received in order; assert role 2's prompt contains role 1's output where `{{PREV_OUTPUT}}` was placed.
  - `TestInvokeTagsTurnsWithRole` — fake executor returns a ParsedTurn; assert merged transcript's turns carry role + stage = role name.
  - `TestExpandPromptSubstitutions` — pure helper test: `{{TASK}}` and `{{PREV_OUTPUT}}` replaced, unknown tokens preserved.

**Frontend (Vitest):**

- `frontend/src/components/launcher/MultiAgentForm.test.tsx`:
  - Renders initial planner/coder roles when value is undefined.
  - Add role button creates a new empty row; max-5 limit enforced.
  - Remove disabled when there's only one role.
  - Up/down arrows reorder rows.
  - onChange fires with the right shape on name / prompt edits.
- `frontend/src/pages/runs/role-accent.test.ts`:
  - Same role → same palette entry.
  - Empty / undefined → fallback `border-l-border`.
  - Distribution: at least 3 distinct role names produce 3 distinct palette entries (verifies the hash isn't trivially constant).

**Manual:**

- Launcher: select `multiagent`, edit roles, add a 3rd role, verify the `{{PREV_OUTPUT}}` hint and that Launch enables only when valid.
- Run a 3-role experiment end-to-end; open Inspector; verify each turn card has its role badge and a colored left bar; verify two turns from the same role share the same color.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Old experiments with `harness_id = 'planner_coder'` fail at re-run | Acceptable per spec (clean break, same pattern as Project 1's `claudemd`). User can delete or ignore historical experiments. |
| 5-role cap is arbitrary | Backend constant `multiagentMaxRoles = 5`. Easy to lift in a follow-up; ~5 keeps the form / transcript readable in the meantime. |
| Tokens like `{{TASK}}` accidentally appear in user content unrelated to substitution | Acceptable. Substitution is one-pass literal `ReplaceAll`. If the user really needs a literal `{{TASK}}` in the agent's view they can escape it themselves — vanishingly rare in practice. |
| Role color palette collisions with > 5 distinct roles across the app | Cap is 5 per variant, but across N variants the user could see up to N×5 distinct names. Hash collisions just mean two unrelated roles share a color; not a correctness issue. |
| `extractPlanFromResult` rename touches existing code | Function lives only inside this package; the rename is local. Grep before/after to confirm. |

## Open questions

None — all decisions locked in brainstorming.
