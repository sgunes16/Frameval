# Real spec-kit + community extensions on opencode — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make each spec-kit variant run a real spec-kit workflow under the opencode executor by installing the spec-kit CLI in the sandbox, running `specify init` + per-variant `specify extension add --from <url>` in the workspace, and redefining each variant's stages to the extension's real `/speckit.*` commands. The spec-kit CLI version is editable from Settings.

**Architecture:** Sandbox image bakes `specify-cli` (default `v0.9.1`). A new optional `SandboxPreparer` harness capability lets the spec-kit harness declare shell steps the orchestrator runs in the sandbox (via `o.sandbox.RunShell`) before `Invoke` — `specify init` then (if the variant has an extension) `yes | specify extension add <name> --from <url>`. The catalog is rewritten so each variant's stages are the extension's real commands. `speckit.version` lives in `app_settings` + Settings UI; the resolved value is merged into the variant cfg before `Setup`. `.opencode`/`.specify` are scope-excluded.

**Tech Stack:** Go (engine), Docker (sandbox image), React/TS (Settings UI), SQLite (app_settings).

**Design spec:** `docs/superpowers/specs/2026-06-02-speckit-real-extensions-design.md`

---

## Extension reference (verified by live install)

| Variant | Extension id | `--from` URL | Real commands (primary order) | Scripts in sandbox? | Brownfield fit |
|---|---|---|---|---|---|
| canonical | — (core) | none (`specify init` only) | specify → clarify → plan → tasks → analyze → implement | core only | good |
| lite | tinyspec | `https://github.com/Quratulain-bilal/spec-kit-tinyspec/archive/refs/heads/main.zip` (no release; pin by commit SHA) | tinyspec.classify → tinyspec → tinyspec.implement | none | good (writes specs/tiny/) |
| research-first | brownfield | `https://github.com/Quratulain-bilal/spec-kit-brownfield/archive/refs/heads/main.zip` (no release) | brownfield.scan → specify → plan → tasks → implement | none | scan read-only ok; migrate heavy |
| rigorous | red-team | `https://github.com/ashbrener/spec-kit-red-team/archive/refs/tags/v1.0.2.zip` | specify → clarify → red-team.run → plan → tasks → analyze → implement | none | good (arbitrary target path) |
| dual-role | conduct | `https://github.com/twbrandon7/spec-kit-conduct-ext/archive/refs/tags/v1.0.1.zip` | conduct.run specify → conduct.run plan → conduct.run tasks → conduct.run implement | **load.sh/common.sh** | inherits core |
| tdd-first | v-model | `https://github.com/leocamello/spec-kit-v-model/archive/refs/tags/v0.7.2.zip` | v-model.requirements → v-model.acceptance → v-model.plan → v-model.tasks → v-model.implement → v-model.trace | **heavy setup-*.sh + gates** | worst (Status:Approved gates) |

**Reproducibility note:** tinyspec + brownfield have no GitHub releases — pin them by commit SHA (`archive/<sha>.zip`) resolved during Task 2, not the mutable `main` branch.

---

## Phase 0 — Feasibility smoke per extension (do FIRST, gate the rest)

### Task 0: Smoke-test each extension on a reference brownfield task in the sandbox image

**Files:** none (throwaway sandbox runs).

- [ ] **Step 1: Build the sandbox image with specify-cli** (after Task 1 lands, or locally pre-built). For the smoke, install ad-hoc:
```bash
docker run --rm -v "$PWD/tasks/brownfield-hal-api-pydantic-version:/t:ro" frameval-sandbox:local bash -lc '
  mkdir -p /workspace/app && cp /t/workspace/app/* /workspace/app/ && cp /t/workspace/pyproject.toml /workspace/
  uv tool install --quiet specify-cli --from git+https://github.com/github/spec-kit.git@v0.9.1
  cd /workspace
  specify init --here --ai opencode --script sh --no-git --force --ignore-agent-tools
  for E in "tinyspec|https://github.com/Quratulain-bilal/spec-kit-tinyspec/archive/refs/heads/main.zip" \
           "red-team|https://github.com/ashbrener/spec-kit-red-team/archive/refs/tags/v1.0.2.zip" \
           "conduct|https://github.com/twbrandon7/spec-kit-conduct-ext/archive/refs/tags/v1.0.1.zip" \
           "v-model|https://github.com/leocamello/spec-kit-v-model/archive/refs/tags/v0.7.2.zip" \
           "brownfield|https://github.com/Quratulain-bilal/spec-kit-brownfield/archive/refs/heads/main.zip"; do
    name=${E%%|*}; url=${E#*|}
    echo "=== add $name ==="; yes | specify extension add "$name" --from "$url" 2>&1 | tail -3
  done
  ls .opencode/command*/'
```
Expected: each `extension add` succeeds and registers its `speckit.<ext>.*` commands.

- [ ] **Step 2: Record per-extension verdict.** For conduct + v-model confirm their bash scripts are present and executable (`.specify/extensions/<ext>/scripts/bash/*.sh`). Note any command that hard-requires a `specs/<feature>/` tree or `Status: Approved` gates (v-model) — those cannot pass on a single-file brownfield edit.

- [ ] **Step 3: Decision gate.** If v-model's gates can't be satisfied for a one-file edit (likely), set tdd-first to the **hybrid** flow `v-model.requirements → v-model.acceptance → v-model.plan → v-model.tasks → /speckit.implement` (core implement bypasses v-model gates) OR fall back tdd-first to core `specify → plan → tasks → implement` and drop the v-model extension. Record the chosen tdd-first flow; it feeds Task 2. Do the same sanity pass for conduct's scripts.

---

## Phase 1 — Sandbox image

### Task 1: Bake specify-cli into the sandbox image

**Files:** Modify `docker/sandbox/Dockerfile`.

- [ ] **Step 1: Add a build arg + install line.** After the opencode install block (after the `ENV PATH=...` line that already includes `/root/.local/bin`):
```dockerfile
# spec-kit CLI (specify) — default version; overridable at run time via a
# settings-driven `uv tool install --force ...@<ver>` in the spec-kit harness
# pre-step. uv tool installs into /root/.local/bin (already on PATH).
ARG SPECKIT_DEFAULT_VERSION=v0.9.1
RUN uv tool install specify-cli --from "git+https://github.com/github/spec-kit.git@${SPECKIT_DEFAULT_VERSION}"
```
(`uv` is already installed by the existing pip line.)

- [ ] **Step 2: Build and verify.**
Run: `docker build -t frameval-sandbox:local -f docker/sandbox/Dockerfile docker/sandbox && docker run --rm frameval-sandbox:local bash -lc 'specify --version'`
Expected: `specify 0.9.1`.

- [ ] **Step 3: Commit.** `docs: …` → `git commit -m "sandbox: bake specify-cli (spec-kit) into the image"`

---

## Phase 2 — Catalog: real commands + install metadata

### Task 2: Add install metadata to SpecKitExtension and rewrite the 6 entries

**Files:** Modify `engine/internal/builtin/speckit/catalog.go`. Test: `engine/internal/builtin/speckit/catalog_test.go` (create).

- [ ] **Step 1: Extend the struct** (after `SourceURL`):
```go
type SpecKitExtension struct {
	ID          string
	Name        string
	Description string
	Stages      []Stage
	MultiAgent  bool
	SourceURL   string
	// ExtensionName is the `specify extension add <name>` id; empty for core-only (canonical).
	ExtensionName string
	// InstallURL is the `--from` release/zip URL; empty for core-only.
	InstallURL string
}
```

- [ ] **Step 2: Rewrite each entry's Stages to real commands + set ExtensionName/InstallURL.** Use the verified flows from the reference table (and the Task-0 tdd-first decision). Example (canonical + lite + rigorous shown; apply the same shape to research-first, dual-role, tdd-first):
```go
{
	ID: "canonical", Name: "Canonical spec-kit", Description: "Full core SDD flow.",
	SourceURL: "https://github.github.io/spec-kit/",
	Stages: []Stage{
		{Name: "specify",  SlashCommand: "/speckit.specify",  PromptTemplate: "/speckit.specify\n\n{{TASK}}"},
		{Name: "clarify",  SlashCommand: "/speckit.clarify",  PromptTemplate: "/speckit.clarify"},
		{Name: "plan",     SlashCommand: "/speckit.plan",     PromptTemplate: "/speckit.plan\n\n{{TECHNICAL_DETAILS}}"},
		{Name: "tasks",    SlashCommand: "/speckit.tasks",    PromptTemplate: "/speckit.tasks"},
		{Name: "analyze",  SlashCommand: "/speckit.analyze",  PromptTemplate: "/speckit.analyze"},
		{Name: "implement",SlashCommand: "/speckit.implement",PromptTemplate: "/speckit.implement"},
	},
},
{
	ID: "lite", Name: "TinySpec (lightweight)", Description: "Single-file lightweight flow.",
	ExtensionName: "tinyspec",
	InstallURL: "https://github.com/Quratulain-bilal/spec-kit-tinyspec/archive/<PIN_SHA>.zip",
	Stages: []Stage{
		{Name: "tinyspec",          SlashCommand: "/speckit.tinyspec",           PromptTemplate: "/speckit.tinyspec\n\n{{TASK}}"},
		{Name: "tinyspec-implement",SlashCommand: "/speckit.tinyspec.implement", PromptTemplate: "/speckit.tinyspec.implement"},
	},
},
{
	ID: "rigorous", Name: "Red Team review", Description: "Adversarial spec review before planning.",
	ExtensionName: "red-team",
	InstallURL: "https://github.com/ashbrener/spec-kit-red-team/archive/refs/tags/v1.0.2.zip",
	Stages: []Stage{
		{Name: "specify",   SlashCommand: "/speckit.specify",      PromptTemplate: "/speckit.specify\n\n{{TASK}}"},
		{Name: "clarify",   SlashCommand: "/speckit.clarify",      PromptTemplate: "/speckit.clarify"},
		{Name: "red-team",  SlashCommand: "/speckit.red-team.run", PromptTemplate: "/speckit.red-team.run specs"},
		{Name: "plan",      SlashCommand: "/speckit.plan",         PromptTemplate: "/speckit.plan\n\n{{TECHNICAL_DETAILS}}"},
		{Name: "tasks",     SlashCommand: "/speckit.tasks",        PromptTemplate: "/speckit.tasks"},
		{Name: "implement", SlashCommand: "/speckit.implement",    PromptTemplate: "/speckit.implement"},
	},
},
```
Replace the `<PIN_SHA>` placeholders for tinyspec + brownfield with the commit SHA resolved in Task 0 (no mutable `main`). research-first → `brownfield.scan` then core; dual-role → `conduct.run <phase>` per stage keeping `Role` tags; tdd-first → the Task-0 decision.

- [ ] **Step 3: Remove invented commands.** Ensure `/speckit.tests`, `/speckit.verify`, `/speckit.research`, `/speckit.review` no longer appear anywhere in the file.

- [ ] **Step 4: Guard test.** Create `catalog_test.go`:
```go
func TestCatalogUsesOnlyKnownCommands(t *testing.T) {
	known := map[string]bool{ // core + per-extension real prefixes
		"specify": true, "clarify": true, "plan": true, "tasks": true,
		"analyze": true, "checklist": true, "constitution": true, "implement": true,
	}
	prefixes := []string{"tinyspec", "brownfield", "red-team", "conduct", "v-model"}
	for _, ext := range List() {
		for _, s := range ext.Stages {
			cmd := strings.TrimPrefix(s.SlashCommand, "/speckit.")
			ok := known[cmd]
			for _, p := range prefixes { if strings.HasPrefix(cmd, p) { ok = true } }
			if !ok { t.Errorf("%s stage %q uses unknown command %q", ext.ID, s.Name, s.SlashCommand) }
		}
		if ext.ID != "canonical" && (ext.ExtensionName == "" || ext.InstallURL == "") {
			t.Errorf("%s missing ExtensionName/InstallURL", ext.ID)
		}
	}
}
```
Run: `cd engine && go test ./internal/builtin/speckit/` → PASS.

- [ ] **Step 5: Commit.** `git commit -m "speckit: redefine extensions to real spec-kit commands + install metadata"`

---

## Phase 3 — Install pre-step + version wiring

### Task 3: SandboxPreparer capability + spec-kit implementation

**Files:** Modify `engine/pkg/harness/harness.go`, `engine/internal/builtin/harness/speckit.go`. Test: `engine/internal/builtin/harness/speckit_test.go`.

- [ ] **Step 1: Add the optional interface** in `harness.go` (after the `Harness` interface):
```go
// SandboxPreparer is an optional capability: a harness that needs shell setup
// run inside the sandbox (with the workspace mounted) before Invoke implements
// it. The orchestrator runs each returned command via the sandbox manager on
// the run's workspace. Commands run in order; a non-zero exit fails the run.
type SandboxPreparer interface {
	SandboxPrepCommands(run HarnessRun) []string
}
```

- [ ] **Step 2: Stash version + extension on the HarnessRun in spec-kit Setup.** In `speckit.go` Setup, after resolving `ext`, also read the version from cfg (injected by the orchestrator, Task 4) and stash both:
```go
version := "v0.9.1"
if sk, ok := cfg["speckit"].(map[string]any); ok {
	if v, ok := sk["version"].(string); ok && strings.TrimSpace(v) != "" { version = v }
}
// ... in the returned HarnessRun.Metadata:
metadataKeySpecKitVersion: version,
```
(add `metadataKeySpecKitVersion = "speckit.version"` const.)

- [ ] **Step 3: Implement SandboxPrepCommands** on `*SpecKit`:
```go
func (h *SpecKit) SandboxPrepCommands(run harness.HarnessRun) []string {
	ext, _ := run.Metadata[metadataKeySpecKitExtension].(speckit.SpecKitExtension)
	version, _ := run.Metadata[metadataKeySpecKitVersion].(string)
	cmds := []string{}
	// Non-default CLI version → reinstall in-sandbox (needs network).
	if version != "" && version != "v0.9.1" {
		cmds = append(cmds, fmt.Sprintf(
			`uv tool install --force specify-cli --from "git+https://github.com/github/spec-kit.git@%s"`, version))
	}
	cmds = append(cmds, `specify init --here --ai opencode --script sh --no-git --force --ignore-agent-tools`)
	if ext.ExtensionName != "" && ext.InstallURL != "" {
		cmds = append(cmds, fmt.Sprintf(`yes | specify extension add %s --from %q`, ext.ExtensionName, ext.InstallURL))
	}
	return cmds
}
```

- [ ] **Step 4: Unit test** `speckit_test.go`: a `lite` HarnessRun yields 2 commands (`specify init`, `extension add tinyspec`); `canonical` yields 1 (`specify init` only); non-default version prepends the `uv tool install --force`. Run `go test ./internal/builtin/harness/`.

- [ ] **Step 5: Commit.** `git commit -m "harness: SandboxPreparer + spec-kit install pre-step commands"`

### Task 4: Orchestrator — inject version + run prep commands before Invoke

**Files:** Modify `engine/internal/experiment/orchestrator.go`. Test: extend an orchestrator test or add `speckit_prep_test.go`.

- [ ] **Step 1: Resolve speckit.version and merge into the variant cfg** just before `harnessImpl.Setup` (line ~247), gated on harness id:
```go
if harnessID == "speckit" {
	version := speckitVersionOrDefault(ctx, o.store) // GetSetting("speckit.version") w/ env+"v0.9.1" fallback
	if variant.HarnessConfig == nil { variant.HarnessConfig = map[string]any{} }
	sk, _ := variant.HarnessConfig["speckit"].(map[string]any)
	if sk == nil { sk = map[string]any{} }
	sk["version"] = version
	variant.HarnessConfig["speckit"] = sk
}
```
Add helper `speckitVersionOrDefault(ctx, store)` (mirror the judge env-fallback pattern: `store.GetSetting` → `os.Getenv("FRAMEVAL_SPECKIT_RELEASE")` → `"v0.9.1"`).

- [ ] **Step 2: Run prep commands after Setup, before Invoke** (after `hRun.BaseRunConfig = …`, before `invokeWithTimeout`):
```go
if prep, ok := harnessImpl.(pkgharness.SandboxPreparer); ok {
	for _, cmd := range prep.SandboxPrepCommands(hRun) {
		out, perr := o.sandbox.RunShell(ctx, workspace, verificationEnvironment(task.ID, harnessID), cmd)
		o.broadcastRunLog(*experiment, *run, "harness", "$ "+cmd)
		if strings.TrimSpace(out) != "" { o.broadcastRunLog(*experiment, *run, "harness", out) }
		if perr != nil {
			_ = o.store.UpdateRunStatus(ctx, run.ID, "failed", fmt.Sprintf("speckit prep failed: %v", perr))
			return perr
		}
	}
}
```

- [ ] **Step 3: Test.** With `support.FakeExecutor` + a fake sandbox (or testcontainers behind the `integration` tag): assert prep commands run for a speckit variant and a prep failure marks the run failed. At minimum a unit test of `speckitVersionOrDefault` fallback chain. Run `go test ./internal/experiment/`.

- [ ] **Step 4: Commit.** `git commit -m "orchestrator: run spec-kit sandbox prep (init + extension add) before Invoke"`

---

## Phase 4 — Scope excludes

### Task 5: Exclude .opencode from scope-discipline

**Files:** Modify `engine/internal/experiment/orchestrator.go` (`harnessExcludePathspecs`). Test: `engine/internal/experiment/scope_excludes_test.go`.

- [ ] **Step 1:** In the `case "speckit":` arm, add `":!.opencode", ":!.opencode/**"` to the returned slice (alongside the existing `.specify`/`specs`/`memory`).

- [ ] **Step 2: Test.** Extend `TestHarnessExcludePathspecs` so the speckit expectation includes the two `.opencode` pathspecs. Run `go test ./internal/experiment/ -run TestHarnessExcludePathspecs`.

- [ ] **Step 3: Commit.** `git commit -m "orchestrator: scope-exclude .opencode for spec-kit runs"`

---

## Phase 5 — Settings (version editable from the UI)

### Task 6: Backend — speckit.version setting + API

**Files:** Create `engine/internal/storage/migrations/0NN_speckit_version.sql`; modify `engine/internal/api/config_handler.go`, `engine/internal/api/router.go`.

- [ ] **Step 1: Migration** (next number after the latest): seed default.
```sql
INSERT OR IGNORE INTO app_settings(key, value) VALUES ('speckit.version', 'v0.9.1');
```

- [ ] **Step 2: Handlers** mirroring `GetLLMSettings`/`PutLLMSettings`:
```go
type specKitSettingsPayload struct{ Version string `json:"version"` }
func (s *Service) GetSpecKitSettings(w http.ResponseWriter, r *http.Request) {
	v, err := s.store.GetSetting(r.Context(), "speckit.version")
	if err != nil || v == "" { v = "v0.9.1" }
	JSON(w, http.StatusOK, specKitSettingsPayload{Version: v})
}
func (s *Service) PutSpecKitSettings(w http.ResponseWriter, r *http.Request) {
	var p specKitSettingsPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil || strings.TrimSpace(p.Version) == "" {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeValidation, "version required", nil); return
	}
	if err := s.store.SetSetting(r.Context(), "speckit.version", strings.TrimSpace(p.Version)); err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err); return
	}
	s.GetSpecKitSettings(w, r)
}
```

- [ ] **Step 3: Routes** in `router.go` next to the llm-settings routes:
```go
r.Get("/config/speckit-settings", service.GetSpecKitSettings)
r.Put("/config/speckit-settings", service.PutSpecKitSettings)
```

- [ ] **Step 4: Test** `config_handler_test.go`: PUT then GET round-trips the version; empty version → 400. Run `go test ./internal/api/`.

- [ ] **Step 5: Commit.** `git commit -m "engine: speckit.version app_setting + config API"`

### Task 7: Frontend — SpecKit settings panel

**Files:** Create `frontend/src/components/settings/speckit.tsx`; modify `frontend/src/lib/hooks.ts`, `frontend/src/lib/types.ts`, `frontend/src/pages/settings/index.tsx`.

- [ ] **Step 1: Type** in `types.ts`: `export interface SpecKitSettings { version: string }`.

- [ ] **Step 2: Hooks** in `hooks.ts` (mirror `useLLMSettings`/`useSaveLLMSettings`):
```ts
export function useSpecKitSettings() {
  return useQuery({ queryKey: ['config','speckit-settings'], queryFn: () => api.get<SpecKitSettings>('/config/speckit-settings') });
}
export function useSaveSpecKitSettings() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (p: SpecKitSettings) => api.put<SpecKitSettings>('/config/speckit-settings', p),
    onSuccess: () => client.invalidateQueries({ queryKey: ['config','speckit-settings'] }),
  });
}
```

- [ ] **Step 3: Panel** `speckit.tsx` — copy `judge-provider.tsx` shape: load via `useSpecKitSettings`, local `version` state synced in `useEffect`, an `<Input>` for the version + a "Save" `<Button>` calling `save.mutate({version})`, disabled until dirty. Include a one-line helper text: "spec-kit CLI release tag (e.g. v0.9.1). Used to install spec-kit in the sandbox at run time."

- [ ] **Step 4: Mount** `<SpecKitPanel />` in `settings/index.tsx` near `<JudgeProviderPanel />`.

- [ ] **Step 5: Verify.** `cd frontend && npm run lint && npm run build && npm test -- --run`.

- [ ] **Step 6: Commit.** `git commit -m "frontend: SpecKit version settings panel"`

---

## Phase 6 — End-to-end verification

### Task 8: One real run per variant + Compare

- [ ] **Step 1:** Rebuild the sandbox image (Task 1). Restart engine (reseed) + grader.
- [ ] **Step 2:** Launch one experiment on `brownfield-hal-api-pydantic-version`, opencode/deepseek model, all 6 variants.
- [ ] **Step 3:** For each variant, inspect the transcript: confirm the agent executes the extension's real commands (the prep log shows `specify init` + `extension add`, and stages no longer say "command doesn't exist"). Confirm scope passes (no `.opencode`/`.specify` drift).
- [ ] **Step 4:** If v-model/conduct misbehave (gate failures / missing scripts), apply the Task-0 fallback for that variant and note it.

---

## Self-review notes

- **Reproducibility:** tinyspec + brownfield must be pinned by commit SHA (Task 2 Step 2) — `main` is mutable.
- **Network:** `extension add --from` and non-default `uv tool install` need outbound network at run time; the sandbox allows outbound (per CLAUDE.md). Prep failure fails the run loudly (Task 4 Step 2).
- **Risk acknowledged:** v-model (gated, 17 cmds) and conduct (ships scripts) are the misfit-prone variants; Task 0 gates them and Task 8 Step 4 has the fallback.
- **Interface minimality:** `SandboxPreparer` is optional — no change to existing harnesses; orchestrator type-asserts it.
- **No invented commands:** Task 2 Step 3 + the guard test enforce only real commands remain.
