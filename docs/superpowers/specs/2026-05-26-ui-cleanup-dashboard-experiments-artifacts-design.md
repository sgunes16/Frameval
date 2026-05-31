# UI cleanup: remove Dashboard + Artifacts, redesign Experiments

**Date:** 2026-05-26
**Scope:** Frontend only (no backend, no API, no schema changes)

## Goal

Cut two low-signal pages (Dashboard, Artifacts) and tighten the
Experiments page so the user lands on a screen that reflects the
actual thesis workflow (configure → run → inspect) instead of a
landing page that mostly shows stale stats and "Coming Soon" tiles.

## Motivation

Frameval has drifted past the original layout: the LLM-judge work
made the Dashboard's "Coming soon" tile obsolete (judge is shipped),
the Artifacts page has been "Preview / read-only" since v0.1 and
hasn't been used in any thesis run, and the Experiments table grew
to 8 columns over time, making it noisy to scan. The user's actual
loop is `Run diagnostic → Experiments → Run inspect`, so Experiments
should be the entry point.

## Out of scope

- Backend or API changes
- Tasks, Settings, Rubrics, RunInspect, RunGrading,
  DiagnosticLaunch, DiagnosticCompare pages
- The Header component
- Introducing a shared `<PageHeader>` primitive or other broader
  consistency normalization (deferred — separate PR if/when needed)

## Architecture overview

Three independent changes share one PR because they all touch
`routes.tsx` + `sidebar.tsx`:

1. **Remove Dashboard.** Delete the page file, make `/` redirect to
   `/experiments`, drop the sidebar nav item.
2. **Remove Artifacts.** Delete the two page files and the
   sidebar nav item; delete `components/artifacts/` and the
   `useArtifacts` / `useCreateArtifact` hooks if no other page
   imports them (verify via `grep` before deleting).
3. **Redesign Experiments.** Replace the 8-column table with a
   5-column one, add a single-line summary chip bar above the
   filter row, and inline the cost number under the runs count
   instead of giving it its own column.

No backend interaction changes. The existing `useExperiments()`
hook supplies everything the redesigned page needs.

## Detailed design

### 1. Routing and navigation

**`frontend/src/routes.tsx`:**

```tsx
import { Navigate, Routes, Route } from 'react-router-dom';
// (drop DashboardPage, ArtifactsPage, ArtifactDetailPage imports)

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/experiments" replace />} />
      <Route path="/tasks" element={<TasksPage />} />
      <Route path="/tasks/new" element={<NewTaskPage />} />
      <Route path="/tasks/:id" element={<TaskDetailPage />} />
      <Route path="/experiments" element={<ExperimentsPage />} />
      <Route path="/experiments/:id/monitor" element={<ExperimentMonitorPage />} />
      <Route path="/runs/:id/inspect" element={<RunInspectPage />} />
      <Route path="/runs/:id/grading" element={<RunGradingPage />} />
      <Route path="/diagnostic/launch" element={<DiagnosticLaunchPage />} />
      <Route path="/diagnostic/compare" element={<DiagnosticComparePage />} />
      <Route path="/settings" element={<SettingsPage />} />
      <Route path="/rubrics" element={<RubricsPage />} />
      <Route path="*" element={<Navigate to="/experiments" replace />} />
    </Routes>
  );
}
```

The trailing wildcard redirect covers stale bookmarks for the removed
`/artifacts` and `/` URLs without producing a blank page.

**`frontend/src/components/layout/sidebar.tsx`:** remove the
`{ to: '/', label: 'Dashboard', ... }` and
`{ to: '/artifacts', label: 'Artifacts', hint: 'Preview', ... }`
entries from `navItems`. Resulting order:

1. Run diagnostic
2. Compare runs
3. Experiments
4. Task library
5. Settings
6. Rubrics

### 2. Dashboard removal

Delete `frontend/src/pages/dashboard/index.tsx` (the whole directory
if it's the only file). No other code imports `DashboardPage` once
the route is gone.

### 3. Artifacts removal

Delete:

- `frontend/src/pages/artifacts/index.tsx`
- `frontend/src/pages/artifacts/detail.tsx`
- The `artifacts/` directory itself once empty

**Components and hooks cleanup (verify first):**

Run before deleting:

```bash
grep -rn "components/artifacts\|ArtifactDiff\|ArtifactEditor\|ArtifactUpload" frontend/src --include='*.tsx' --include='*.ts'
grep -rn "useArtifacts\|useCreateArtifact" frontend/src --include='*.tsx' --include='*.ts'
```

If the only hits are inside `pages/artifacts/` (about to be deleted)
and `components/artifacts/` itself, delete:

- `frontend/src/components/artifacts/` (whole directory)
- The `useArtifacts` and `useCreateArtifact` exports in
  `frontend/src/lib/hooks.ts`
- Any artifact-only types in `frontend/src/lib/types.ts` that
  nothing else references

If anything else imports them, leave those files alone and document
the surviving consumers in the PR description.

### 4. Experiments redesign

**`frontend/src/pages/experiments/index.tsx`:**

Page structure top-down:

```
<div className="space-y-4">
  <SummaryChips experiments={experiments} />
  <FilterBar ... />               {/* existing Card, padding tightened */}
  <ExperimentsTable rows={filtered} />   {/* or EmptyState */}
</div>
```

#### Summary chips

A single-line row above the filter Card. Not a Card itself — just a
flex row of inline chips so it reads as supplementary information,
not a control.

```tsx
function SummaryChips({ experiments }: { experiments: Experiment[] }) {
  const total = experiments.length;
  const running = experiments.filter((e) => e.status === 'running').length;
  const queued = experiments.filter((e) => e.status === 'draft' || e.status === 'queued').length;
  const completed = experiments.filter((e) => e.status === 'completed').length;
  return (
    <div className="flex flex-wrap items-center gap-2 px-1 text-xs text-fg-muted">
      <span className="font-medium text-fg">{total} experiments</span>
      <span className="text-fg-subtle">·</span>
      <span>{running} running</span>
      <span className="text-fg-subtle">·</span>
      <span>{queued} queued</span>
      <span className="text-fg-subtle">·</span>
      <span>{completed} completed</span>
    </div>
  );
}
```

Status names mirror what `statusLabel()` already exposes. If the
backend never uses `queued` we drop that chip; check via:

```bash
grep -rn '"status":\|status:' engine/internal/models --include='*.go' | grep -i 'queued\|draft\|running\|completed'
```

#### Filter bar

Unchanged structurally — same status pills + search input + "New
diagnostic run" CTA. Pad the Card with `py-3 px-4` instead of the
current default to make it visually less imposing now that summary
chips sit above it.

#### Table — 5 columns

| Column | Width | Content |
|---|---|---|
| Experiment | flex | `name` (font-medium, fg) + 1-line clamped description (xs, fg-muted) |
| Status | narrow | `<Badge tone={statusTone(status)}>` |
| Agent · Model | narrow | `agent_cli · model` (fg-muted, monospace optional) |
| Runs | narrow | Line 1: `{variants}v × {runs_per_variant}r` (fg). Line 2: `~{formatCurrency(cost)}` (fg-subtle, xs) |
| Created | narrow, right-aligned | `formatTimeAgo(created_at)` + inline `Open →` link (text only, not a Button — keeps the row visually quiet) |

The previous "Variants" and "Estimated cost" columns are removed.
Cost lives under Runs; raw variant count lives in the Runs cell
itself. Cost is omitted entirely if `estimated_cost_usd == 0`.

The "Open" link rendered next to the timestamp uses the same
`Link` from react-router but styled as a text link, not a `Button`:

```tsx
<Link
  to={`/experiments/${experiment.id}/monitor`}
  className="text-fg-muted hover:text-fg"
>
  Open →
</Link>
```

This drops one visual element per row (the `Button` outline) and
keeps the table calmer to scan.

#### Empty state

Same `<EmptyState>` component, copy refined to:

```
title: "No experiments yet"  (when 0 in DB) OR "No experiments match" (when filtered)
description: "Start a diagnostic run to compare harnesses on a task."
action: New diagnostic run button
```

Decide between the two title strings based on whether the underlying
list is empty vs. filtered-empty (use `experiments.length === 0`).

### 5. Color and spacing

Use existing tokens: `bg`, `bg-elev-1`, `bg-elev-2`, `border`,
`border-strong`, `fg`, `fg-muted`, `fg-subtle`. No new tokens.
No new shadcn primitives. Spacing follows the page's existing
`space-y-4` rhythm.

## Data flow

Unchanged. Everything still flows:

```
React Query (useExperiments) ─→ ExperimentsPage ─→ rendered components
```

No new endpoints, no new query keys, no new mutations.

## Error handling

Unchanged. `useExperiments()` already surfaces loading / error
states through React Query; the existing fallback patterns
(rendering `[]` while loading) carry over.

## Testing

This is a UI-only refactor with no new logic. Verification is:

1. `cd frontend && npm run typecheck` — must pass cleanly.
2. `cd frontend && npm run lint` — must pass cleanly (no leftover
   `DashboardPage` / `ArtifactsPage` imports, no unused hooks).
3. Start the dev server and walk through:
   - Visit `http://localhost:5173/` → URL becomes
     `http://localhost:5173/experiments`
   - Visit `http://localhost:5173/artifacts` → URL becomes
     `http://localhost:5173/experiments` (via wildcard redirect)
   - Sidebar shows 6 items in the order listed above
   - Experiments page shows the summary chip row above the
     filter Card
   - Table has 5 columns, "Open →" is a text link (no Button),
     cost is rendered under the runs count
   - Empty/filtered-empty states render the right copy
4. No new unit tests are required for this change. If
   `pages/experiments/index.test.tsx` exists (check first), update
   it to reflect the new column count and table structure;
   otherwise don't introduce one in this PR.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| `components/artifacts/` reused elsewhere we didn't notice | Run the two `grep` commands above before deleting; if hits found outside artifacts/, leave files in place and document in PR |
| Status enum missing `queued` value | Render the queued chip only when `queued > 0`. If the grep shows no `queued` status anywhere in the codebase, drop the chip from the template entirely instead of relying on the conditional. |
| Bookmarks to `/` or `/artifacts` 404 | Wildcard route + root redirect both go to `/experiments` |
| Cost column removal hides a number a user wanted | Cost still visible under Runs; only the dedicated column goes away |

## Open questions

None remaining — all routing, scope, and table shape decisions
locked during brainstorming.
