# Design System V2 — "Telemetry Chic"

**Status:** Draft
**Date:** 2026-05-14
**Owner:** sgunes16
**Related:** [[2026-05-14-run-inspector-v2-design]], [[2026-05-14-compare-v2-design]], `frontend/tailwind.config.ts`, `frontend/src/components/ui/`

## 1. Motivation

The current frontend has the bones of a design system (shadcn/ui primitives, Tailwind, a `tailwind.config.ts` color palette) but its execution is inconsistent. Symptoms found while auditing:

- Type sizes are ad-hoc: `text-[10px]`, `text-[11px]`, `text-xs`, `text-sm` all coexist with no rule for when to use which.
- Spacing is set by hand on every component: `p-3`, `p-4`, `p-6`, `mt-0.5`, `gap-1.5` chosen by feel, not by a scale.
- Chart colors are hardcoded (`#2563eb`, `#16a34a`, `#dc2626`) and don't follow the palette tokens, so dark mode would break them.
- Dark mode is configured (`darkMode: ['class']`) but never used — most components only render correctly in light mode.
- Turkish placeholder text in `patch-viewer.tsx:28` ("Patch gormek icin tamamlanmis bir run sec").
- 8+ ad-hoc empty-state implementations, 0 standardized loading skeletons, 0 toast notifications, no error boundary.

This is a tool for engineers and researchers who will spend hours staring at terminals and dashboards. The right aesthetic is what we're calling **telemetry chic**: a restrained, high-density, dark-mode-first language that borrows from observability tools (Grafana, Datadog, Linear), not from consumer SaaS. Generous whitespace where it aids reading, monospace where it carries semantic weight (paths, tokens, hashes), color used sparingly and meaningfully.

## 2. Goals & non-goals

**Goals.**
- Establish design tokens (color, typography, spacing, radius, shadow, motion) and enforce them via Tailwind config + CSS variables.
- Ship dark-mode-first; light mode is a derived theme, not the default.
- Replace every ad-hoc class string in the codebase with token references.
- Replace every chart color with a CSS variable so charts auto-theme.
- Standardise: empty state, loading skeleton, error state, toast, error boundary, button hierarchy, modal, drawer.
- Distinctive AgentDx components: TurnCard, DiffBadge, ScoreBar, SymptomGlyph, StatusDot, TokenChip, FilePath.
- Removed: Turkish placeholder text, all hardcoded colors in components/, all `text-[Npx]` arbitrary sizes.

**Non-goals.**
- Rewriting the shadcn primitives. We keep `components/ui/*`; we extend them.
- Custom icon library. We continue using Lucide.
- New routing or page structure (that's Run Inspector V2 / Compare V2).
- Animation library. Tailwind's `transition-*` + a tiny `motion-safe` wrapper is enough.

## 3. Design tokens

### 3.1 Color (Tailwind CSS variables in `:root` / `.dark`)

We define semantic tokens, not literal colors. Light mode values are derived from dark mode values for contrast parity. All values are HSL.

| Token | Dark | Light | Purpose |
|---|---|---|---|
| `--bg` | `222 14% 8%` | `0 0% 100%` | Page background |
| `--bg-elev-1` | `222 14% 11%` | `222 14% 98%` | Cards, sidebar |
| `--bg-elev-2` | `222 14% 14%` | `222 14% 96%` | Popovers, dropdowns |
| `--fg` | `220 14% 96%` | `222 14% 12%` | Primary text |
| `--fg-muted` | `220 8% 65%` | `222 8% 40%` | Secondary text |
| `--fg-subtle` | `220 8% 45%` | `222 8% 60%` | Tertiary text, icons |
| `--border` | `222 12% 22%` | `220 14% 90%` | Hairlines, dividers |
| `--border-strong` | `222 12% 32%` | `220 14% 78%` | Focus rings, emphasized borders |
| `--accent` | `199 89% 60%` | `199 89% 50%` | Brand color (cyan) — used sparingly |
| `--success` | `142 71% 45%` | `142 71% 40%` | Pass status |
| `--warning` | `38 92% 55%` | `38 92% 50%` | In-progress, throttle |
| `--danger` | `0 84% 60%` | `0 84% 50%` | Fail status, errors |
| `--info` | `217 91% 60%` | `217 91% 55%` | Neutral information |
| `--code-bg` | `222 14% 6%` | `222 14% 96%` | Monospace blocks |
| `--diff-add` | `142 60% 20%` | `142 60% 92%` | Diff added lines |
| `--diff-del` | `0 60% 22%` | `0 60% 94%` | Diff removed lines |
| `--diff-add-text` | `142 71% 60%` | `142 60% 28%` | Added diff text |
| `--diff-del-text` | `0 84% 70%` | `0 60% 36%` | Removed diff text |
| `--chart-1..8` | (palette) | (palette) | Run/series colors, colorblind-safe |

Chart series colors are an 8-token Okabe–Ito-derived palette tuned for both themes. Recharts components consume them via `var(--chart-1)` etc., never hardcoded.

`tailwind.config.ts` exposes each as a Tailwind color (`bg-elev-1`, `text-fg-muted`, etc.).

### 3.2 Typography

Font stack:
- **UI:** `"Inter Variable", system-ui, sans-serif`
- **Monospace:** `"JetBrains Mono Variable", ui-monospace, monospace`
- **Display:** UI font, just tighter tracking

Scale (locked, no `text-[Npx]` allowed):

| Token | Size | Line height | Use |
|---|---|---|---|
| `text-xs` | 11 px | 16 px | Metadata, tags, table micro labels |
| `text-sm` | 13 px | 20 px | Default body, table cells |
| `text-base` | 15 px | 22 px | Long-form text |
| `text-md` | 16 px | 24 px | Card titles |
| `text-lg` | 18 px | 26 px | Section headers |
| `text-xl` | 22 px | 30 px | Page headers |
| `text-2xl` | 28 px | 36 px | Hero numbers in dashboards |
| `text-3xl` | 36 px | 44 px | Reserved for empty-state hero |

All sizes ship from Tailwind defaults except we override `xs` and `md`. Line heights are baked into the size token so designers don't have to remember.

Letter spacing: `tracking-tight` (-0.01em) for ≥ `text-lg`, default elsewhere. Headers and numerals use `font-variation-settings: 'opsz' 32` via Inter Variable's optical size axis.

### 3.3 Spacing

Strict 4 px base scale exposed via Tailwind defaults — `0.5` = 2 px, `1` = 4 px, `2` = 8 px, `3` = 12 px, `4` = 16 px, `6` = 24 px, `8` = 32 px, `12` = 48 px. Arbitrary values are linted out (ESLint rule `tailwindcss/no-arbitrary-values` with allowlist for hex colors only on canvas/SVG).

Layout rhythm:
- Page gutter: `px-6` (24 px)
- Card padding: `p-4` (16 px) default, `p-3` for dense cards
- Card → card gap: `gap-3`
- Section → section gap: `gap-8`

### 3.4 Radius, shadow, border

- Radius: `rounded-sm` = 4 px (chips, inputs), `rounded-md` = 6 px (default — cards, buttons), `rounded-lg` = 10 px (modals, side panels), `rounded-full` for pills and status dots.
- Shadow: dark-mode preference is hairline borders, not box shadows. Single `shadow-elev` token for popovers/modals = `0 1px 0 hsla(0,0%,100%,0.06) inset, 0 6px 16px hsla(0,0%,0%,0.4)`.
- Border: 1 px hairlines using `--border`. Focused inputs use `--border-strong` + a 2 px `--accent` outline at `outline-offset: 2px`.

### 3.5 Motion

A small motion vocabulary, no animation libraries:

| Token | Duration | Easing | Use |
|---|---|---|---|
| `motion-fast` | 120 ms | `cubic-bezier(0.4, 0, 0.2, 1)` | Hover, focus, color shifts |
| `motion-default` | 200 ms | `cubic-bezier(0.4, 0, 0.2, 1)` | Open/close panels, dropdowns |
| `motion-slow` | 360 ms | `cubic-bezier(0.16, 1, 0.3, 1)` | Modal enter, drawer slide-in |
| `motion-spring` | — | CSS spring-y bezier | Toast bounce, fork-glyph reveal |

All motion respects `prefers-reduced-motion: reduce` — durations collapse to 0 ms automatically via a media query wrapper.

Defined in `frontend/src/styles/motion.css` and consumed via Tailwind plugin classnames (`transition-motion-fast`).

## 4. Component library extensions

These compose on top of shadcn primitives, living under `frontend/src/components/system/`:

### 4.1 Atomic
- **`StatusDot`** — colored 8 px dot with optional pulse for live; `variant: success | warning | danger | info | neutral`.
- **`TokenChip`** — pill for token counts (`12.4k in / 3.1k out`), monospace numerals.
- **`FilePath`** — truncates middle, monospace, full path on title attr.
- **`Kbd`** — keyboard shortcut pill, used in Cmd-K palette and tooltips.

### 4.2 Composite
- **`TurnCard`** — used by Run Inspector V2 (see [[2026-05-14-run-inspector-v2-design]]). Left-bar color = `block_kind`; header has turn index, role glyph, optional symptom glyph; body collapsible.
- **`DiffBadge`** — `+12 / -3` pill, monospace numbers, semantic colors from `--diff-add-text` / `--diff-del-text`.
- **`ScoreBar`** — segmented horizontal bar for 0–1 dimensions; used for fingerprint dimensions and grade rubrics. Hover reveals exact value.
- **`SymptomGlyph`** — small colored dot + 2-letter code (`HA`, `SD`, `LP`), tooltip with full failure label and quoted rationale.
- **`EmptyState`** — standardized: centered card, optional icon, title (`text-md`), description (`text-sm text-fg-muted`), optional CTA button.
- **`LoadingSkeleton`** — three variants (`card`, `row`, `block`) that match the shape of common containers.
- **`ErrorState`** — title, description, "Try again" CTA, optional "Copy debug info" secondary, optional error code chip.
- **`Toast`** — driven by sonner; only `success | error | info | loading`; auto-dismiss 4 s default; max 3 stacked.

### 4.3 Patterns
- **`TabStack`** — vertical tabs with sticky header; used in Compare V2.
- **`CmdPalette`** — Cmd-K palette wrapping cmdk; standard slots for commands + recent items.
- **`DrawerSheet`** — right-side drawer with breakpoint-aware width (480 / 640 / 800).

## 5. Page chrome

- **Sidebar.** 56 px wide on the rail by default, expandable to 220 px. Nav items use a single-letter glyph collapsed, full label expanded. Active item gets a 2 px left accent bar.
- **Header.** 48 px tall, hairline bottom border, breadcrumb on the left, command-K hint + theme toggle on the right.
- **Theme toggle.** Three states: `system` (default), `dark`, `light`. State stored in localStorage; respects `prefers-color-scheme`.

## 6. Removal list

The following lines/files are deleted or rewritten as part of this work:

- `frontend/src/components/run-monitor/patch-viewer.tsx:28` Turkish text → `"Select a completed run to view its patch."`
- `frontend/src/components/run-monitor/patch-viewer.tsx:37` Turkish text → English equivalent.
- All `text-[10px]` / `text-[11px]` / `text-[Npx]` literals → `text-xs` / `text-sm`.
- All chart color literals (`#2563eb`, `#16a34a`, `#dc2626`, …) → `var(--chart-N)`.
- All `bg-white` / `text-slate-900` outside `theme.css` → token classes.

A CI lint rule will reject re-introductions:

```
- ESLint rule: forbid `\[[\d]+px\]` in className strings
- ESLint rule: forbid `#[0-9a-fA-F]{3,8}` in `.tsx` files (allowlist comment escape `// design-system-allow-hex`)
- Custom regex CI check for non-Latin characters in source files (block reintroduction of placeholder text in any language)
```

## 7. Testing strategy

**Visual regression (Playwright snapshots):**
- A `design-system.spec.ts` page renders every system component in both themes, light and dark. Snapshot diff blocks merges.

**Unit (Vitest + RTL):**
- `Toast` queue dedup behavior.
- `StatusDot` variant → CSS variable mapping.
- `FilePath` middle-truncate cutoff for 80, 120, 200 character paths.

**Accessibility:**
- Axe runs over every page in CI.
- Color contrast checked against WCAG AA: `--fg` on `--bg` ≥ 7.0, `--fg-muted` on `--bg` ≥ 4.5, `--accent` on `--bg-elev-1` ≥ 3.0 (large text).
- `:focus-visible` outline mandatory on every interactive element; reviewed via axe.

**Storybook (optional, behind feature flag):**
- We do *not* introduce Storybook for the MVP. The visual regression page above is the lightweight equivalent. Storybook can be added later if a designer joins.

## 8. Migration plan

1. **Land tokens.** New `tailwind.config.ts` + `frontend/src/styles/tokens.css` + `theme.css`. Theme toggle ships disabled. No visual changes yet because everything still uses old classes.
2. **Migrate shadcn primitives.** `components/ui/*` updated to consume tokens. Visual diff is small (mostly border + bg shifts).
3. **Migrate page chrome.** Sidebar + header rewritten. Theme toggle enabled.
4. **Migrate feature pages.** Dashboard, Experiments, Tasks, Artifacts, Settings, Diagnostic — page by page, each behind no flag (changes are cosmetic).
5. **Ship distinctive components.** `TurnCard`, `DiffBadge`, `ScoreBar`, `SymptomGlyph` — consumed by Run Inspector V2 and Compare V2.
6. **Clean up.** Delete legacy classes, enable lint rules, run visual regression baseline.

Each step is a separate PR so visual changes can be reviewed in isolation.

## 9. Acceptance criteria

- No `text-[Npx]` arbitrary sizes remain in `frontend/src/**`.
- No `#[0-9a-f]{3,8}` literals remain in `frontend/src/**` (outside `theme.css`).
- Dark mode is the default; theme toggle persists across reloads.
- WCAG AA contrast met for all body and metadata text.
- All `EmptyState`, `LoadingSkeleton`, `ErrorState`, and `Toast` usages route through the system components — no ad-hoc empty cards.
- Visual regression snapshots exist for every page in light + dark.
- The Turkish placeholder text is gone from the codebase.

## 10. Out of scope / future

- Custom icon set (we stay on Lucide).
- Storybook.
- Animation library (Framer Motion). Postponed until we have a need beyond Tailwind transitions.
- Branding/marketing site styling — we only style the app.
