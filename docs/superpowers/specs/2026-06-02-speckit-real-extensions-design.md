# Real spec-kit + community extensions for the opencode executor

**Date:** 2026-06-02
**Status:** Design — pending review

## Problem

The spec-kit harness sends bare `/speckit.*` slash commands (e.g. `/speckit.tasks`) as raw prompts, but opencode has no spec-kit commands registered, so the agent is confused on content-less stages and never runs a real spec-kit workflow (confirmed in experiment 5e6dc7ac: agents did the task on `specify`/`plan`, then on bare `tasks`/`implement` said "this doesn't exist as a file"). The 6 Frameval "extensions" (canonical/lite/tdd-first/research-first/rigorous/dual-role) also use invented commands (`/speckit.tests`, `/speckit.review`, `/speckit.research`) that don't exist in real spec-kit.

## Goal

Make each spec-kit variant run a **real** spec-kit workflow under opencode by:
1. installing the spec-kit CLI in the sandbox image,
2. running `specify init` (offline) in the workspace so opencode registers the real `/speckit.*` commands,
3. installing the per-variant community extension via `specify extension add --from <release-zip-url>`,
4. redefining each variant's stages to the **real** commands its extension provides.

## Feasibility (verified)

- `specify init --here --ai opencode --script sh --no-git --force --ignore-agent-tools` runs **offline** (templates bundled with the CLI) and writes `.opencode/commands/*.md` + `.specify/`. `--no-git` avoids the git extension's branch/commit hooks interfering with Frameval's `baseline` scope diff.
- opencode (cwd = `/workspace`) discovers commands from `<cwd>/.opencode/commands/`. Frameval runs opencode at `/workspace`, so commands must land there.
- Community catalog is **discovery-only** (`specify extension add <name>` is refused), BUT `specify extension add <name> --from <release-zip-url>` installs from a trusted URL. It prompts `[y/N]`; pipe `yes |` for non-interactive. Verified end-to-end: v-model installed and registered `speckit.v-model.*` commands.
- Each `exec.Execute` runs in a fresh container, but the workspace is copied in/out each call, so `.opencode/`/`.specify/` written by the pre-step persist across stages.

## Extension mapping (locked)

| Variant | Extension | Source |
|---|---|---|
| canonical | spec-kit core (no extension) | `specify init` only |
| lite | TinySpec | github.com/Quratulain-bilal/spec-kit-tinyspec (release zip) |
| research-first | Brownfield Bootstrap | github.com/Quratulain-bilal/spec-kit-* (release zip) |
| tdd-first | V-Model | github.com/leocamello/spec-kit-v-model @ v0.7.2 (release zip) — verified |
| rigorous | Red Team | github.com/ashbrener/spec-kit-red-team (release zip) |
| dual-role | Conduct Extension | github.com/twbrandon7/* (release zip) |

Exact release tags + zip URLs per extension are pinned during planning (read each repo's latest release). Each variant's **stage list** is derived from the commands its extension provides (read the installed `.opencode/commands/speckit.<ext>.*.md` + the extension's README/workflow); canonical uses the core flow `specify → clarify → plan → tasks → analyze → implement`.

## Architecture

### 1. Sandbox image (`docker/sandbox/Dockerfile`)
`RUN uv tool install specify-cli --from git+https://github.com/github/spec-kit.git@<DEFAULT_SPECKIT_VERSION>` (default `v0.9.1`). `specify` lands on PATH (`~/.local/bin`, already exported).

### 2. Harness install pre-step (run once, in-sandbox, before stages)
Add an optional capability so a harness can declare shell steps the orchestrator runs in the sandbox (via `o.sandbox.RunShell` on the workspace) before `Invoke`. The spec-kit harness returns:
```
specify init --here --ai opencode --script sh --no-git --force --ignore-agent-tools
# if the variant has an extension:
yes | specify extension add <name> --from <release-zip-url>
```
The version in the `git+...@<ver>` (CLI) and the `--from` tag (extension) come from settings/env (below). Output persists to the workspace, so later stages see the commands.

(Alternative considered: `RunConfig.PreCommand` on stage 0 — rejected because init should run once, not per stage, and the orchestrator pre-step keeps the executor generic.)

### 3. Catalog redefinition (`engine/internal/builtin/speckit/catalog.go`)
Replace the invented commands. Each variant's `Stages` become the real command sequence for its extension (e.g. tdd-first → `speckit.v-model.requirements → .plan → .acceptance → .implement → .trace`; canonical → core flow). Each stage carries the extension id so the pre-step knows what to install. Remove `/speckit.tests`, `/speckit.verify`, `/speckit.research`, `/speckit.review`.

### 4. Version via Settings (`app_settings` + Settings page)
- `app_settings['speckit.version']` — spec-kit CLI version (default from `FRAMEVAL_SPECKIT_RELEASE` env, fallback `v0.9.1`). Editable on the Settings page (same pattern as `judge.provider`/`judge.model`).
- Extension release tags pinned in `catalog.go` initially (changeable by PR); a follow-up may surface them in Settings.
- The engine passes the resolved version to the harness, which bakes it into the `git+...@<ver>` install in the pre-step (a non-default version triggers a runtime `uv tool install --force` — needs network).

### 5. Scope excludes (`engine/internal/experiment/orchestrator.go`)
`harnessExcludePathspecs("speckit")` adds `:!.opencode`, `:!.opencode/**` (keeps existing `.specify`, `specs`, `memory`). So spec-kit scaffolding never reads as agent scope drift.

## Risks

| Risk | Mitigation |
|---|---|
| Network needed for `extension add --from` each run | Accept (version-from-settings requires runtime install); cache/bake default core in image so only the extension fetch hits the network. Surface install failures as a clear run error. |
| Community extensions are greenfield-feature-oriented (branches, specs dirs) — may misfit brownfield micro-tasks & inflate scope | `.opencode`/`.specify` excluded; `--no-git`; evaluate per-extension flow during planning and trim stages that create unrelated artifacts. |
| Extension `--from` install prompts interactively | `yes |` pipe; verified non-interactive. |
| Extension repo/tag drift breaks installs | Pin exact release tags; `extension add` failure → run fails loudly, not silently. |
| Per-run latency (init + fetch) | Acceptable for eval; bounded by the per-run wall timeout (#157). |

## Testing

- Go: catalog uses only real command names (guard test); `harnessExcludePathspecs("speckit")` includes `.opencode`.
- In-sandbox smoke (per extension, during planning): `specify init` + `extension add --from` succeeds and registers the expected commands; a reference-solution run passes the task tests for at least canonical + one extension variant.
- Manual: launch one experiment per variant on opencode; confirm the transcript shows the agent executing the extension's real commands (not "command doesn't exist").

## Out of scope

- Surfacing per-extension versions in Settings (follow-up).
- Non-opencode executors (Claude Code already has spec-kit support natively).
- Re-running historical experiments.
