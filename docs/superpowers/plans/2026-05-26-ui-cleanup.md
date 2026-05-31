# UI cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the unused Dashboard and Artifacts pages, redirect `/` and unknown URLs to `/experiments`, and redesign the Experiments page with a summary chip row and a tighter 5-column table.

**Architecture:** Frontend-only refactor — three independent UI changes wired through `routes.tsx` and `sidebar.tsx`. No backend, no API, no schema, no new dependencies. All work done on a single branch (`feature/ui-cleanup-2026-05-26`, already created).

**Tech Stack:** React 18, TypeScript, React Router v6, Tailwind CSS, TanStack Query, shadcn/ui primitives — all already in the project.

**Spec:** [`docs/superpowers/specs/2026-05-26-ui-cleanup-dashboard-experiments-artifacts-design.md`](../specs/2026-05-26-ui-cleanup-dashboard-experiments-artifacts-design.md)

**Pre-flight verification (done during planning):**
- `useArtifacts` is consumed by `frontend/src/components/compare/ArtifactsTab.tsx` (DiagnosticCompare). **Keep the hook.**
- `useCreateArtifact` is consumed only by `frontend/src/pages/artifacts/detail.tsx` — safe to delete.
- `frontend/src/components/artifacts/{artifact-editor,artifact-upload,artifact-diff}.tsx` are imported only by `pages/artifacts/detail.tsx` — safe to delete after the page is gone.
- `frontend/src/components/compare/ArtifactDiff.tsx` (capital A, no hyphen) is a separate component used by Compare; **do not touch.**

---

## File map

| File | Action |
|---|---|
| `frontend/src/pages/dashboard/index.tsx` | DELETE (whole `dashboard/` dir) |
| `frontend/src/pages/artifacts/index.tsx` | DELETE |
| `frontend/src/pages/artifacts/detail.tsx` | DELETE (whole `artifacts/` dir) |
| `frontend/src/components/artifacts/artifact-editor.tsx` | DELETE |
| `frontend/src/components/artifacts/artifact-upload.tsx` | DELETE |
| `frontend/src/components/artifacts/artifact-diff.tsx` | DELETE (whole `components/artifacts/` dir) |
| `frontend/src/lib/hooks.ts` | MODIFY (drop `useCreateArtifact`; keep `useArtifacts`) |
| `frontend/src/routes.tsx` | MODIFY (drop two imports + routes; add root + wildcard redirect) |
| `frontend/src/components/layout/sidebar.tsx` | MODIFY (drop Dashboard + Artifacts nav items) |
| `frontend/src/pages/experiments/index.tsx` | MODIFY (summary chips + 5-column table + text-link Open) |

Status enum status check informs whether the `queued` chip is rendered or omitted (Task 5).

---

## Task 1: Sidebar — drop Dashboard and Artifacts nav items

**Files:**
- Modify: `frontend/src/components/layout/sidebar.tsx`

- [ ] **Step 1: Remove the Dashboard entry from navItems**

In `frontend/src/components/layout/sidebar.tsx`, locate the `navItems` array (starts around line 13). Delete this entry:

```tsx
{
  to: '/',
  label: 'Dashboard',
  icon: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" className="h-4 w-4">
      <path strokeLinecap="round" strokeLinejoin="round" d="M3 12l9-9 9 9" />
      <path strokeLinecap="round" strokeLinejoin="round" d="M5 10v10h14V10" />
    </svg>
  ),
},
```

- [ ] **Step 2: Remove the Artifacts entry from navItems**

In the same array, delete this entry:

```tsx
{
  to: '/artifacts',
  label: 'Artifacts',
  hint: 'Preview',
  icon: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" className="h-4 w-4">
      <path strokeLinecap="round" strokeLinejoin="round" d="M4 7l8-4 8 4M4 7v10l8 4 8-4V7M4 7l8 4 8-4M12 11v10" />
    </svg>
  ),
},
```

- [ ] **Step 3: Verify the array now has 6 entries in this order**

The remaining `navItems` order must be:

1. `to: '/diagnostic/launch'` — Run diagnostic
2. `to: '/diagnostic/compare'` — Compare runs
3. `to: '/experiments'` — Experiments
4. `to: '/tasks'` — Task library
5. `to: '/settings'` — Settings
6. `to: '/rubrics'` — Rubrics

If the file changes typecheck cleanly (next step), the order is right.

- [ ] **Step 4: Run typecheck**

Run from the repo root: `cd frontend && npm run typecheck`
Expected: no errors. (Sidebar still has unused-by-route-only entries; `/` is removed in Task 2 and that's fine — `NavLink` doesn't care whether a route exists.)

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/layout/sidebar.tsx
git commit -m "Drop Dashboard and Artifacts entries from sidebar nav"
```

---

## Task 2: Routes — redirect `/` + wildcard, drop dead imports/routes

**Files:**
- Modify: `frontend/src/routes.tsx`

- [ ] **Step 1: Rewrite routes.tsx with the redirects and trimmed imports**

Replace the entire contents of `frontend/src/routes.tsx` with:

```tsx
import { Navigate, Routes, Route } from 'react-router-dom';
import { TasksPage } from './pages/tasks';
import { TaskDetailPage } from './pages/tasks/detail';
import { NewTaskPage } from './pages/tasks/new';
import { ExperimentsPage } from './pages/experiments';
import { ExperimentMonitorPage } from './pages/experiments/monitor';
import { RunInspectPage } from './pages/runs/inspect';
import { RunGradingPage } from './pages/runs/grading';
import { DiagnosticComparePage } from './pages/diagnostic/compare';
import { DiagnosticLaunchPage } from './pages/diagnostic/launch';
import { SettingsPage } from './pages/settings';
import { RubricsPage } from './pages/rubrics';

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

Removed compared to the previous version:
- `import { DashboardPage } from './pages/dashboard';`
- `import { ArtifactsPage } from './pages/artifacts';`
- `import { ArtifactDetailPage } from './pages/artifacts/detail';`
- `<Route path="/" element={<DashboardPage />} />` → replaced by redirect
- `<Route path="/artifacts" ... />` and `<Route path="/artifacts/:id" ... />` → dropped
- Added `Navigate` to the `react-router-dom` import
- Added the catch-all `*` redirect at the end

- [ ] **Step 2: Run typecheck**

Run: `cd frontend && npm run typecheck`
Expected: errors complaining that `frontend/src/pages/dashboard/index.tsx` and `frontend/src/pages/artifacts/*` no longer have referenced exports — but these files still exist and still export. Should still pass cleanly because we just removed unused imports.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/routes.tsx
git commit -m "Redirect / and unknown URLs to /experiments, drop Dashboard + Artifacts routes"
```

---

## Task 3: Delete the Dashboard page

**Files:**
- Delete: `frontend/src/pages/dashboard/` (directory)

- [ ] **Step 1: Confirm no remaining references**

Run: `grep -rn "pages/dashboard\|DashboardPage" frontend/src --include='*.tsx' --include='*.ts'`
Expected: no output (zero remaining references after Tasks 1–2).

If any reference is found, **stop and fix it** before continuing.

- [ ] **Step 2: Delete the directory**

```bash
rm -rf frontend/src/pages/dashboard
```

- [ ] **Step 3: Run typecheck + lint**

```bash
cd frontend && npm run typecheck && npm run lint
```
Expected: both pass cleanly.

- [ ] **Step 4: Commit**

```bash
git add -A frontend/src/pages/dashboard
git commit -m "Delete unused Dashboard page"
```

(The `-A` is needed because we deleted files; `git add` of a non-existent path otherwise errors.)

---

## Task 4: Delete the Artifacts pages, components, and the `useCreateArtifact` hook

**Files:**
- Delete: `frontend/src/pages/artifacts/` (whole dir)
- Delete: `frontend/src/components/artifacts/` (whole dir)
- Modify: `frontend/src/lib/hooks.ts` (remove `useCreateArtifact` only)

- [ ] **Step 1: Re-verify the safe-to-delete set**

```bash
grep -rn "pages/artifacts" frontend/src --include='*.tsx' --include='*.ts'
grep -rn "components/artifacts" frontend/src --include='*.tsx' --include='*.ts'
grep -rn "useCreateArtifact" frontend/src --include='*.tsx' --include='*.ts'
```

Expected:
- `pages/artifacts` — only the files inside `pages/artifacts/` itself
- `components/artifacts` — only the files inside `components/artifacts/` itself + the import line inside `pages/artifacts/detail.tsx`
- `useCreateArtifact` — only the export line in `lib/hooks.ts` + the import in `pages/artifacts/detail.tsx`

If anything else surfaces, **stop and update the plan** rather than deleting blind.

- [ ] **Step 2: Delete the artifacts page and component directories**

```bash
rm -rf frontend/src/pages/artifacts
rm -rf frontend/src/components/artifacts
```

- [ ] **Step 3: Remove `useCreateArtifact` from hooks.ts**

Open `frontend/src/lib/hooks.ts`. The function starts at line 317 with `export function useCreateArtifact()`. Delete the entire function block (export through closing `}`). **Keep `useArtifacts`** (starts around line 235) — it is still used by `components/compare/ArtifactsTab.tsx`.

If `useCreateArtifact` references a helper imported only for its use (e.g. an `apiClient.createArtifact` import), remove that import line as well. Check for stranded imports by reading the file's import block after the deletion.

- [ ] **Step 4: Run typecheck + lint**

```bash
cd frontend && npm run typecheck && npm run lint
```
Expected: both pass cleanly. The lint pass will catch any leftover unused imports we forgot to delete.

- [ ] **Step 5: Commit**

```bash
git add -A frontend/src/pages/artifacts frontend/src/components/artifacts frontend/src/lib/hooks.ts
git commit -m "Delete unused Artifacts page, components, and useCreateArtifact hook"
```

---

## Task 5: Decide whether to render the `queued` chip

**Files:**
- Read-only: `engine/internal/models/*.go`

- [ ] **Step 1: Check whether the backend ever emits `status = "queued"`**

```bash
grep -rn 'Status\|"queued"\|"draft"\|"running"\|"completed"\|"failed"' engine/internal/models --include='*.go' | head -20
grep -rn '"queued"' engine/internal --include='*.go' | head
```

Look for a Status enum or constant set on the Experiment model. Note which of `{draft, running, completed, failed, queued, ...}` actually exist.

- [ ] **Step 2: Record the decision inline**

If `queued` exists in the backend enum, the SummaryChips template includes:
```tsx
<span>{queued} queued</span>
```
and renders it only when `queued > 0`.

If `queued` does **not** exist, drop that chip entirely from the template in Task 6 — do not render a conditional that can never fire.

No commit for this task; the decision feeds Task 6.

---

## Task 6: Experiments — add `<SummaryChips />` above the filter bar

**Files:**
- Modify: `frontend/src/pages/experiments/index.tsx`

- [ ] **Step 1: Add the SummaryChips component inside the same file**

Add this component definition at the bottom of `frontend/src/pages/experiments/index.tsx`, after the existing `ExperimentsPage` export (alongside any other helpers):

```tsx
function SummaryChips({ experiments }: { experiments: ReturnType<typeof useExperiments>['data'] }) {
  const list = experiments ?? [];
  const total = list.length;
  const running = list.filter((e) => e.status === 'running').length;
  const queued = list.filter((e) => e.status === 'draft' || e.status === 'queued').length;
  const completed = list.filter((e) => e.status === 'completed').length;

  return (
    <div className="flex flex-wrap items-center gap-2 px-1 text-xs text-fg-muted">
      <span className="font-medium text-fg">{total} experiments</span>
      <span className="text-fg-subtle">·</span>
      <span>{running} running</span>
      {queued > 0 && (
        <>
          <span className="text-fg-subtle">·</span>
          <span>{queued} queued</span>
        </>
      )}
      <span className="text-fg-subtle">·</span>
      <span>{completed} completed</span>
    </div>
  );
}
```

If Task 5 found that `queued` is never emitted by the backend AND `draft` is also unused, simplify to just the `running` / `completed` chips and drop the `queued` filter expression entirely.

- [ ] **Step 2: Render SummaryChips at the top of the page**

Inside the `ExperimentsPage` JSX, the outer wrapper is:

```tsx
return (
  <div className="space-y-4">
    <Card>
      <div className="flex flex-wrap items-center justify-between gap-3">
        ...
```

Insert the new line immediately after `<div className="space-y-4">`:

```tsx
return (
  <div className="space-y-4">
    <SummaryChips experiments={experiments} />
    <Card>
      ...
```

- [ ] **Step 3: Run typecheck**

`cd frontend && npm run typecheck`
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/pages/experiments/index.tsx
git commit -m "Add experiment summary chips above the filter bar"
```

---

## Task 7: Experiments — collapse to 5 columns + inline Open link

**Files:**
- Modify: `frontend/src/pages/experiments/index.tsx`

- [ ] **Step 1: Replace the existing `<table>` block with the new 5-column layout**

In `frontend/src/pages/experiments/index.tsx`, locate the existing `<table className="min-w-full text-sm">` (it has 8 `<th>` elements). Replace the entire `<table>...</table>` with:

```tsx
<table className="min-w-full text-sm">
  <thead className="bg-bg-elev-2 text-xs uppercase tracking-wider text-fg-muted">
    <tr>
      <th className="px-4 py-2 text-left font-medium">Experiment</th>
      <th className="px-4 py-2 text-left font-medium">Status</th>
      <th className="px-4 py-2 text-left font-medium">Agent · Model</th>
      <th className="px-4 py-2 text-left font-medium">Runs</th>
      <th className="px-4 py-2 text-right font-medium">Created</th>
    </tr>
  </thead>
  <tbody>
    {filtered.map((experiment) => {
      const variantCount = experiment.variants?.length ?? 0;
      const runsLabel = `${variantCount}v × ${experiment.runs_per_variant}r`;
      const cost = experiment.estimated_cost_usd ?? 0;
      return (
        <tr key={experiment.id} className="border-t border-border bg-bg-elev-1 hover:bg-bg-elev-2/60">
          <td className="px-4 py-3">
            <div className="font-medium text-fg">{experiment.name}</div>
            {experiment.description && (
              <div className="mt-0.5 line-clamp-1 text-xs text-fg-muted">{experiment.description}</div>
            )}
          </td>
          <td className="px-4 py-3">
            <Badge tone={statusTone(experiment.status)}>{statusLabel(experiment.status)}</Badge>
          </td>
          <td className="px-4 py-3 text-fg-muted">
            {experiment.agent_cli}
            <span className="text-fg-subtle"> · </span>
            {experiment.model}
          </td>
          <td className="px-4 py-3">
            <div className="text-fg">{runsLabel}</div>
            {cost > 0 && (
              <div className="text-xs text-fg-subtle">~{formatCurrency(cost)}</div>
            )}
          </td>
          <td className="px-4 py-3 text-right text-fg-muted">
            <div className="flex items-center justify-end gap-3">
              <span>{formatTimeAgo(experiment.created_at)}</span>
              <Link
                to={`/experiments/${experiment.id}/monitor`}
                className="text-fg-muted hover:text-fg"
              >
                Open →
              </Link>
            </div>
          </td>
        </tr>
      );
    })}
  </tbody>
</table>
```

- [ ] **Step 2: Drop the now-unused `Button` import if nothing else in the file uses it**

Search the file for any remaining `<Button` usages. The filter row still uses `<Button size="sm">New diagnostic run</Button>` — so **keep the Button import**.

Also confirm `formatCurrency` is still imported (the runs cell uses it); it was already in the import list, so no change needed.

- [ ] **Step 3: Run typecheck + lint**

```bash
cd frontend && npm run typecheck && npm run lint
```
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/pages/experiments/index.tsx
git commit -m "Collapse experiments table to 5 columns with inline Open link"
```

---

## Task 8: Experiments — refine empty-state copy

**Files:**
- Modify: `frontend/src/pages/experiments/index.tsx`

- [ ] **Step 1: Replace the existing EmptyState block**

In `frontend/src/pages/experiments/index.tsx`, find:

```tsx
{filtered.length === 0 ? (
  <EmptyState
    title="No experiments match"
    description="Adjust your filters or start a new diagnostic run."
    action={
      <Link to="/diagnostic/launch">
        <Button size="sm">Start diagnostic run</Button>
      </Link>
    }
  />
) : (
```

Replace with:

```tsx
{filtered.length === 0 ? (
  <EmptyState
    title={experiments.length === 0 ? 'No experiments yet' : 'No experiments match'}
    description={
      experiments.length === 0
        ? 'Start a diagnostic run to compare harnesses on a task.'
        : 'Adjust your filters or clear the search to see all experiments.'
    }
    action={
      <Link to="/diagnostic/launch">
        <Button size="sm">New diagnostic run</Button>
      </Link>
    }
  />
) : (
```

- [ ] **Step 2: Run typecheck**

`cd frontend && npm run typecheck`
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/experiments/index.tsx
git commit -m "Refine experiments empty-state copy for empty vs filtered-empty"
```

---

## Task 9: Full repo sanity pass — typecheck + lint + build

**Files:** none

- [ ] **Step 1: Typecheck**

```bash
cd frontend && npm run typecheck
```
Expected: clean.

- [ ] **Step 2: Lint**

```bash
cd frontend && npm run lint
```
Expected: clean. If any "unused import" or "missing dependency" warnings surface, fix them in the offending file and commit the fix as its own commit (`Fix lint after UI cleanup`).

- [ ] **Step 3: Build**

```bash
cd frontend && npm run build
```
Expected: build succeeds. If the project doesn't have a `build` script, skip this step.

- [ ] **Step 4: Run any frontend tests**

```bash
cd frontend && npm test -- --run 2>&1 | tail -30
```
Expected: clean. If Vitest picks up a test that references the deleted Dashboard / Artifacts pages, delete that test file in its own commit and re-run.

If everything's clean, no commit needed for this task.

---

## Task 10: Manual verification in the dev server

**Files:** none (runtime verification only)

- [ ] **Step 1: Start the dev server**

```bash
cd frontend && npm run dev
```

Note the local URL (typically `http://localhost:5173`).

- [ ] **Step 2: Walk through the verification list**

Open a browser and check each of the following:

1. Visit `/` → URL becomes `/experiments` after the redirect fires.
2. Visit `/artifacts` → URL becomes `/experiments` (catch-all redirect).
3. Visit `/somewhere-bogus` → URL becomes `/experiments`.
4. Sidebar shows exactly 6 nav items in the order: Run diagnostic, Compare runs, Experiments, Task library, Settings, Rubrics.
5. The Experiments page renders a single-line summary chip row above the filter Card: `N experiments · M running · [Q queued] · C completed`.
6. The table has 5 column headers: Experiment, Status, Agent · Model, Runs, Created.
7. Each row's Runs cell shows `Xv × Yr` on the first line and (when cost > 0) `~$0.NN` underneath in fg-subtle.
8. Each row's right edge shows the relative timestamp followed by an "Open →" text link (no button outline).
9. Clicking "Open →" navigates to `/experiments/{id}/monitor`.
10. Filter / search behaves as before — only the layout changed.
11. With zero experiments (e.g. fresh DB) the empty state reads "No experiments yet" with a "New diagnostic run" button.
12. With experiments present but the search query matching nothing, the empty state reads "No experiments match".

- [ ] **Step 3: Stop the dev server**

`Ctrl+C` in the terminal where it's running.

- [ ] **Step 4: If anything in Step 2 failed, file a follow-up task and STOP**

A failure in manual verification is a real bug, not a polish issue. Stop the plan, root-cause the failure, fix it in a focused commit, then re-run Task 10.

If everything passed, mark this task complete and proceed to Task 11.

---

## Task 11: Push branch, open PR, request review

**Files:** none (git + GitHub operations)

- [ ] **Step 1: Push the branch**

```bash
git push -u origin feature/ui-cleanup-2026-05-26
```

- [ ] **Step 2: Open the PR**

```bash
gh pr create --title "UI cleanup: remove Dashboard + Artifacts, redesign Experiments" --body "$(cat <<'EOF'
## Summary

Frontend-only refactor that:

- Deletes the Dashboard page and redirects `/` → `/experiments`
- Deletes the Artifacts page (read-only/unused since v0.1) plus the artifacts components and the `useCreateArtifact` hook; preserves `useArtifacts` because Compare's ArtifactsTab still needs it
- Adds a wildcard route so stale `/artifacts` (or any unknown) URLs redirect to `/experiments`
- Drops the two corresponding entries from the sidebar (6 nav items left)
- Redesigns the Experiments page: single-line summary chip row above the filter, 5-column table (was 8), inline "Open →" link instead of an outlined button, cost merged under the runs cell

No backend, API, schema, or dependency changes.

Spec: [`docs/superpowers/specs/2026-05-26-ui-cleanup-dashboard-experiments-artifacts-design.md`](docs/superpowers/specs/2026-05-26-ui-cleanup-dashboard-experiments-artifacts-design.md)

## Test plan

- [x] `npm run typecheck` clean
- [x] `npm run lint` clean
- [x] `npm run build` succeeds
- [x] `npm test` clean (or no tests touch the removed surfaces)
- [x] Manual: `/`, `/artifacts`, `/whatever` all land on Experiments
- [x] Manual: sidebar shows 6 items in the new order
- [x] Manual: Experiments page renders chip bar + 5-col table + inline Open link
- [x] Manual: empty / filtered-empty copy differs correctly
EOF
)"
```

- [ ] **Step 3: Dispatch code-reviewer subagent**

After the PR is open, dispatch `feature-dev:code-reviewer` on the branch HEAD per the repo's standing convention (every non-trivial PR is reviewed).

- [ ] **Step 4: Watch CI**

Once CI starts, monitor until all checks pass (or fix any failures and push the fix as a focused commit).

---

## Self-review

**Spec coverage:**
- Routing & navigation changes → Tasks 1, 2
- Dashboard removal → Task 3
- Artifacts removal (pages + components + hook) → Task 4
- Experiments redesign — summary chips → Tasks 5, 6
- Experiments redesign — 5-column table → Task 7
- Experiments redesign — empty-state copy → Task 8
- Spec's "Test plan" section (typecheck, lint, dev server walk) → Tasks 9, 10
- PR + review per memory `feedback_github_workflow` → Task 11

**No placeholders** — each step shows exact code, exact commands, exact expected output.

**Type consistency** — table columns use `experiment.variants`, `experiment.runs_per_variant`, `experiment.estimated_cost_usd`, `experiment.agent_cli`, `experiment.model`, `experiment.created_at`, `experiment.status`, `experiment.description`, `experiment.name`, `experiment.id` — all fields that already exist on the type used by the current 8-column table, so no new type work is needed.

**Status enum risk** — Task 5 verifies the backend's status enum before Task 6 hardcodes a `queued` filter that might match nothing. Both branches (with-queued and without-queued) are documented in Task 6.

**Ordering** — Tasks are dependency-ordered: sidebar/route surface first (cheap, decouples nav from page existence), then page deletions (which depend on no remaining route references), then Experiments redesign (independent of the deletions but bundled because it's the same PR), then sanity + verification + PR.
