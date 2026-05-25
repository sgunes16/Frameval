# Rubric Editor — Design

**Status:** Draft
**Date:** 2026-05-25
**Owner:** sgunes16
**Related:** [[2026-05-25-per-dimension-judge-design]], [[2026-05-23-llm-judge-design]], `grader/llm_judge/prompts.py`, `proto/grader.proto`, `engine/internal/storage/migrations/`

## 1. Motivation

The five judge dimensions and their rubric prompts are currently hardcoded in `grader/llm_judge/prompts.py`. The user cannot:
- See what prompt is shaping the judge's behavior without reading Python source
- Tune a rubric without editing code + restarting the grader
- Add a new dimension to the experiment (e.g., "security_awareness", "test_coverage", "documentation_quality")

For a thesis on context engineering, this is a key constraint: every rubric prompt IS context engineering, and the user needs to iterate on it experimentally. The judge's prompts must be first-class user-editable artifacts, not buried in code.

Second, a real failure surfaced in the latest smoke test: judge `correctness=3.0` on a run with `test_pass_rate=1.0` (3/3 tests passing). The rationale said *"The test_pass_rate is 0.0, indicating the implemented fix does not pass the supplied tests"* — a flat hallucination. The metric was in the prompt; the model ignored it. The user-prompt format (a generic JSON code block buried under "# Code metrics") is too quiet — the model fills in plausible values instead of reading them.

User pain (verbatim):
- *"sidebar'a rubrics'i koy ve oraya tıklanınca dimensionlar ve promptlarını görebilelim ve düzeltebileleim ayrıca ek olarak dimension ekleme özelliği de getir"* — sidebar entry, editable rubrics, add new dimensions.
- *"test rate'i 3/3 niye böyle dedi"* — judge fabricated test failure where there was none.

Both problems need the same architectural foundation: rubrics become data (SQLite-backed) instead of code, and dimensions become extensible (proto + schema use a map, not five fixed columns). Hallucination fix piggybacks via a redesigned user prompt with explicit CRITICAL FACTS framing.

## 2. Goals & non-goals

**Goals.**
- **Rubrics live in SQLite.** New `rubrics` table seeded with the current 5 prompts. Grader reads via engine (engine passes them in `JudgeConfig.rubrics`).
- **Editable from the UI.** New sidebar entry "Rubrics" → page listing all dimensions (5 builtin + any user-added). Click → edit form: display_name + prompt textarea + save. Delete button per row (with confirm prompt for builtin dims).
- **Add new dimensions.** "+ Add dimension" button opens form: key (lowercase, snake_case, e.g. `security`), display_name (e.g. "Security"), starter prompt template. POST creates a row; from then on every grade scores the new dim too.
- **N-dimension judge.** The 5-fixed-column grade storage becomes a JSON map: `grades.judge_scores: Record<string, float>` + `grades.judge_rationales: Record<string, string>`. Proto switches to `map<string, double>` + `map<string, string>`. Frontend `Grade` type drops `judge_correctness`/etc. in favor of `judge_scores`/`judge_rationales` maps.
- **Composite scoring** averages all enabled dimension scores (each weighted equally). Composite formula = `0.3 * code + 0.3 * mean(judge_scores) + 0.2 * process + 0.2 * spec` (when judge is enabled). When no judge dims exist (or all zero), composite falls back to `0.6 * code + 0.4 * process`.
- **Hallucination mitigation.** `render_user_prompt` rewrites the metrics section as a CRITICAL FACTS block that shouts the values (capitalized labels, repeated, with explicit "do not contradict" rider in each rubric).
- **Migration safety.** A new migration backfills `judge_scores` / `judge_rationales` JSON from the existing 5 columns for already-graded runs, then drops the 5 columns. Existing UI keeps working because new columns + new types are populated before old columns disappear.
- **Backend tests cover** the rubrics repo, the HTTP CRUD, the engine→grader proto plumbing, the composite-scoring fallback, and the per-dim async judge code adapted for variable-N dims.
- **Frontend tests cover** the new Rubrics page (smoke render + add-then-edit-then-delete flow) and the updated LLMJudgeCard (renders N rationales).

**Non-goals.**
- Per-dim weighting (each dim contributes equally to composite for now). User-editable weights are a future spec.
- Cross-experiment rubric versioning (everyone shares one rubric set; if you edit, all future grades use the new version). Versioning is a future spec.
- Per-task rubric overrides. Out of scope.
- Streaming the judge rationale. Still deferred from prior specs.
- Multi-judge / cross-model IRR. Still single-judge; `judge_irr_alpha` stays `0.0` (kept as a column for forward compat).
- Reordering dimensions via drag-and-drop in the UI. `sort_order` column exists but UI uses a + / − button pair (or keeps creation order).
- Rich-text editing for rubric prompts. Plain textarea is enough.

## 3. Approach

Six layers of change:

1. **Storage** — new migration adds `rubrics` table; seeds the 5 current rubrics; restructures `grades` table to drop the 5 hardcoded `judge_*` columns and add `judge_scores TEXT` + `judge_rationales TEXT`. New `RubricsRepo` for CRUD.
2. **Proto** — `JudgeGradeResult.{correctness,maintainability,completeness,best_practices,error_handling}` → replaced by `map<string, double> scores` + `map<string, string> rationales`. `JudgeConfig` gains `repeated DimensionRubric rubrics`.
3. **Backend HTTP** — new `GET /api/config/rubrics`, `GET /api/config/rubrics/{key}`, `PUT /api/config/rubrics/{key}`, `POST /api/config/rubrics`, `DELETE /api/config/rubrics/{key}`.
4. **Engine grading path** — `grader_client.go:buildJudgeConfig` reads rubrics from SQLite, populates the new proto `rubrics` field. `gradeFromProto` switches to map-based read + writes new map columns. Composite scoring helper averages the map values.
5. **Grader** — `grader/llm_judge/grader.py:_grade_async` reads `request.judge_config.rubrics` instead of the hardcoded `_DIMENSIONS + DIMENSION_RUBRICS`. If the proto carries no rubrics (legacy path), falls back to the hardcoded defaults in `prompts.py` so headless / dev path still works. `render_user_prompt` rewritten with CRITICAL FACTS framing.
6. **Frontend** — new `/rubrics` route + sidebar entry + page + form components. `Grade` type uses `judge_scores: Record<string, number>` + `judge_rationales: Record<string, string>`. Compare and Grading Inspector consume the new shape. `LLMJudgeCard` iterates `judge_scores` keys instead of hardcoded 5 dims.

All in one PR on the existing `feature/llm-judge` branch.

## 4. Targeted changes

### 4.1 SQLite — new migration `016_rubrics_and_judge_map.sql`

```sql
-- Editable per-dimension judge rubrics.
CREATE TABLE rubrics (
  key          TEXT PRIMARY KEY,
  display_name TEXT NOT NULL,
  prompt       TEXT NOT NULL,
  sort_order   INTEGER NOT NULL DEFAULT 0,
  is_builtin   INTEGER NOT NULL DEFAULT 0,
  created_at   DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
  updated_at   DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Seed the five builtin dimensions. Prompts must match the canonical
-- versions in grader/llm_judge/prompts.py:DIMENSION_RUBRICS at the time
-- of this migration. The grader will use whatever is in this table at
-- run time; defaults are seeded only on first install.
-- (Full prompt text below is the canonical builtin set; truncated here
-- for the spec, see the migration file for the actual SQL.)
INSERT INTO rubrics (key, display_name, prompt, sort_order, is_builtin) VALUES
  ('correctness',    'Correctness',     '<full prompt>', 1, 1),
  ('maintainability','Maintainability', '<full prompt>', 2, 1),
  ('completeness',   'Completeness',    '<full prompt>', 3, 1),
  ('best_practices', 'Best practices',  '<full prompt>', 4, 1),
  ('error_handling', 'Error handling',  '<full prompt>', 5, 1);

-- Move from 5 hardcoded judge columns to JSON maps.
-- Step 1: add new columns alongside the old ones.
ALTER TABLE grades ADD COLUMN judge_scores TEXT;
ALTER TABLE grades ADD COLUMN judge_rationales TEXT;

-- Step 2: backfill existing rows. The old columns are floats; assemble
-- them into a JSON map. Rationale wasn't stored per-dim before — pull
-- from raw_judge_responses_json (tagged 'dim=<name>;...' from the
-- per-dimension PR) when present, else empty rationale per dim.
UPDATE grades
SET
  judge_scores = json_object(
    'correctness',     COALESCE(judge_correctness, 0.0),
    'maintainability', COALESCE(judge_maintainability, 0.0),
    'completeness',    COALESCE(judge_completeness, 0.0),
    'best_practices',  COALESCE(judge_best_practices, 0.0),
    'error_handling',  COALESCE(judge_error_handling, 0.0)
  ),
  judge_rationales = json_object(
    'correctness',     '',
    'maintainability', '',
    'completeness',    '',
    'best_practices',  '',
    'error_handling',  ''
  )
WHERE judge_scores IS NULL;

-- Step 3: drop the old columns via the standard SQLite table-rebuild.
-- (Done in this migration to keep the change atomic. See migration
-- file for the actual rebuild SQL.)
```

Migration is **destructive** for the 5 old judge_* columns — they cease to exist. The data is preserved in `judge_scores` JSON. Tests on the same DB will pass because storage tests use TmpStore which re-runs all migrations from scratch.

### 4.2 `RubricsRepo` — `engine/internal/storage/rubrics_repo.go`

```go
type Rubric struct {
    Key         string
    DisplayName string
    Prompt      string
    SortOrder   int
    IsBuiltin   bool
    CreatedAt   string
    UpdatedAt   string
}

func (s *Store) ListRubrics(ctx context.Context) ([]Rubric, error)
func (s *Store) GetRubric(ctx context.Context, key string) (Rubric, error)
func (s *Store) UpsertRubric(ctx context.Context, r Rubric) error
func (s *Store) DeleteRubric(ctx context.Context, key string) error
```

Validation lives at the HTTP boundary (key shape, prompt non-empty); the repo trusts its inputs. Returns `sql.ErrNoRows` on missing key.

### 4.3 Proto — `proto/grader.proto`

```protobuf
message DimensionRubric {
  string key = 1;
  string prompt = 2;
}

message JudgeConfig {
  string model = 1;
  string provider = 2;
  string api_key = 3;
  repeated RubricDimension rubric = 4;  // existing; unused, leave for compat
  int32 judge_rounds = 5;
  repeated DimensionRubric rubrics = 6; // NEW
}

message JudgeGradeResult {
  // OLD fields removed (correctness, maintainability, completeness,
  // best_practices, error_handling). They were field 1-5.
  // Renumber: scores=1, rationales=2, irr_alpha=3, raw_responses=4.
  map<string, double> scores = 1;
  map<string, string> rationales = 2;
  double irr_alpha = 3;
  repeated string raw_responses = 4;
}
```

**Backward compat note:** removing proto fields is a wire-level break. Acceptable here because engine and grader ship together and there are no external consumers of the proto. Both sides upgrade in the same PR. Re-run `cd proto && buf generate` to refresh stubs.

### 4.4 Backend HTTP — `engine/internal/api/rubrics_handler.go`

```
GET    /api/config/rubrics              → []Rubric
GET    /api/config/rubrics/{key}        → Rubric
POST   /api/config/rubrics              → 201 Rubric (body: key, display_name, prompt)
PUT    /api/config/rubrics/{key}        → 200 Rubric (body: display_name, prompt)
DELETE /api/config/rubrics/{key}        → 204
```

Validation on POST/PUT:
- `key`: `^[a-z][a-z0-9_]{1,40}$` (lowercase snake_case, max 41 chars)
- `display_name`: 1-80 chars
- `prompt`: 50-20000 chars (lower bound prevents accidental empty prompts; upper bound matches LLM context budget)
- POST rejects duplicate key with 409
- DELETE on builtin (`is_builtin=1`) returns 400 ("builtin rubric cannot be deleted") — UI should hide the delete button for these rows but defense in depth at the API too. Edit is still allowed for builtins.

### 4.5 Engine — `grader_client.go` rubric plumbing + map-based judge response

In `buildJudgeConfig`, after the existing app_settings + api_key lookup, fetch all rubrics and convert to proto:

```go
rubricRows, _ := c.settings.ListRubrics(ctx)
rubricsProto := make([]*graderpb.DimensionRubric, 0, len(rubricRows))
for _, r := range rubricRows {
    rubricsProto = append(rubricsProto, &graderpb.DimensionRubric{
        Key:    r.Key,
        Prompt: r.Prompt,
    })
}
// ... existing code populates Provider, Model, ApiKey ...
cfg.Rubrics = rubricsProto
return cfg
```

`SettingsStore` interface gains a `ListRubrics(ctx) ([]Rubric, error)` method.

`gradeFromProto` switches to map reads:

```go
if response.Judge != nil {
    grade.JudgeScores = response.Judge.Scores         // map[string]float64
    grade.JudgeRationales = response.Judge.Rationales // map[string]string
    grade.JudgeIRRAlpha = float64(response.Judge.IrrAlpha)
    grade.RawJudgeResponses = response.Judge.RawResponses
}
```

`models.Grade` struct loses `JudgeCorrectness, JudgeMaintainability, ...` fields and gains `JudgeScores map[string]float64` + `JudgeRationales map[string]string`. Anywhere those old fields were used must be updated (search by symbol).

### 4.6 Composite scoring — `engine/internal/experiment/composite.go` (or wherever it lives, possibly in grader)

In Python grader `composite.py`:

```python
def compute_composite(code_grade, process_grade, judge_grade=None, adherence_grade=None):
    code_score = float(code_grade.get("test_pass_rate", 0.0)) * 10
    process_score = compute_process_score(process_grade)
    if judge_grade is None and adherence_grade is None:
        return round((code_score * 0.6) + (process_score * 0.4), 4)
    scores = (judge_grade or {}).get("scores") or {}
    if scores:
        judge_score = sum(scores.values()) / len(scores)
    else:
        judge_score = 0.0
    adherence_score = float((adherence_grade or {}).get("instruction_compliance", 0.0))
    return round((code_score * 0.3) + (judge_score * 0.3) + (process_score * 0.2) + (adherence_score * 0.2), 4)
```

(The Go-side `gradeFromProto` writes the composite the grader computed; engine doesn't recompute.)

### 4.7 Grader — `grader/llm_judge/grader.py` adapted for variable-N

`_grade_async` reads dim list + prompts from the JudgeConfig proto. If absent (no rubrics passed), falls back to the hardcoded defaults in `prompts.py:DIMENSION_RUBRICS`.

```python
async def _grade_async(cfg, code_grade, process_grade, task, output_files, transcript_json, rubrics):
    """rubrics is a list of (key, prompt) tuples passed in from the engine,
    or None to fall back to defaults."""
    try:
        client = build_client(cfg, async_client=True)
    except Exception as exc:
        logger.warning("judge async client init failed: %s", exc)
        return _all_dims_failed(str(exc), rubrics or _builtin_rubrics())

    effective = rubrics or _builtin_rubrics()
    user_prompt = render_user_prompt(...)
    tasks = [_score_one_dim(client, cfg.model, key, prompt, user_prompt) for key, prompt in effective]
    results = await asyncio.gather(*tasks, return_exceptions=False)

    scores = {effective[i][0]: results[i][0] for i in range(len(effective))}
    rationales = {effective[i][0]: results[i][1] for i in range(len(effective))}
    raw_responses = [results[i][2] for i in range(len(effective))]
    return {
        "scores": scores,
        "rationales": rationales,
        "irr_alpha": 0.0,
        "raw_responses": raw_responses,
    }


def _builtin_rubrics() -> list[tuple[str, str]]:
    from grader.llm_judge.prompts import DIMENSION_RUBRICS
    return list(DIMENSION_RUBRICS.items())
```

`_score_one_dim` now returns `(score, rationale, raw_response)` so the grader can populate the separate `scores` and `rationales` maps.

The grader's `grade()` entry point in `server.py` updates:
- `judge = judge_grade(..., rubrics=_unpack_rubrics(request.judge_config))`
- The proto response uses `scores` and `rationales` maps instead of 5 named fields.

### 4.8 Grader — `render_user_prompt` rewrite for hallucination resistance

New format:

```python
def render_user_prompt(*, code_grade, process_grade, task, output_files, transcript_json) -> str:
    pass_rate = float(code_grade.get("test_pass_rate", 0.0))
    pass_count = int(code_grade.get("test_pass_count", 0))
    fail_count = int(code_grade.get("test_fail_count", 0))
    total = pass_count + fail_count
    type_check = bool(code_grade.get("type_check_pass", False))
    lint = float(code_grade.get("lint_score", 0.0))
    premature = bool(process_grade.get("premature_completion", False))

    facts = (
        "# CRITICAL FACTS — these are authoritative measurements. If your "
        "rationale contradicts a fact below, the fact wins and you must "
        "reflect it in your score.\n\n"
        f"- TESTS: {pass_count} / {total} passed (test_pass_rate = {pass_rate:.2f}).\n"
        f"- TYPE CHECK: {'PASS' if type_check else 'FAIL'}.\n"
        f"- LINT SCORE: {lint:.1f} / 10.\n"
        f"- PREMATURE COMPLETION: {'YES (agent stopped before fully solving)' if premature else 'NO'}.\n"
    )
    files_block = "\n\n".join(
        f"=== {f.get('path', '<unnamed>')} ===\n{_decode_content(f.get('content'))[:4000]}"
        for f in output_files[:10]
    )
    transcript_tail = _decode_content(transcript_json)[-3000:]
    return (
        f"{facts}\n"
        f"# Task\n\n{task.get('prompt', '<no prompt>')}\n\n"
        f"# Output files (truncated)\n\n{files_block or '(no files)'}\n\n"
        f"# Transcript tail\n\n{transcript_tail or '(empty)'}\n"
    )
```

The DIMENSION_RUBRICS' `_SHARED_TAIL` also gains:

```
## Hard rule

If the CRITICAL FACTS section says tests passed (pass_count > 0 with
fail_count = 0), do NOT claim tests failed in your rationale. If
type_check is PASS, do NOT claim it failed. The facts are authoritative.
```

This addresses the smoke-test hallucination without changing any code structure.

### 4.9 Frontend — new `/rubrics` page

Sidebar (existing component in `frontend/src/components/system/` or similar — verify in plan) gains a new "Rubrics" link.

New page `frontend/src/pages/rubrics/index.tsx`:

```
┌────────────────────────────────────────┐
│  Rubrics                  + Add dim    │
├────────────────────────────────────────┤
│  Correctness                [Edit] []  │  ← builtin: delete hidden
│  Maintainability            [Edit] []  │
│  Completeness               [Edit] []  │
│  Best practices             [Edit] []  │
│  Error handling             [Edit] []  │
│  Security        (custom)   [Edit][Del]│  ← user-added shows delete
└────────────────────────────────────────┘
```

Click Edit → modal or inline expand:
- Display name input
- Prompt textarea (monospace, ~30 lines tall, autosize)
- Save / Cancel buttons

"+ Add dim" → modal:
- Key input (lowercase, snake_case, validation feedback)
- Display name input
- Prompt textarea (with a starter template — copy of `correctness` rubric with key/name swapped)
- Create / Cancel

Routes registered in `routes.tsx`. Hooks added: `useRubrics`, `useRubric(key)`, `useUpsertRubric`, `useCreateRubric`, `useDeleteRubric` in `hooks.ts`.

### 4.10 Frontend — `Grade` type + LLMJudgeCard for map shape

`frontend/src/lib/types.ts`:

```typescript
export type Grade = {
  // ... unchanged code/process/spec fields ...
  // OLD: judge_correctness, judge_maintainability, etc. — removed
  judge_scores?: Record<string, number>;
  judge_rationales?: Record<string, string>;
  judge_irr_alpha?: number;
  raw_judge_responses?: string[];
};
```

`LLMJudgeCard.tsx` reads `grade.judge_scores` and `grade.judge_rationales` directly (no more `extractRationalesByDim`), iterates entries:

```tsx
{Object.entries(grade.judge_scores ?? {}).map(([dim, score]) => (
  <DimRow key={dim} label={prettyDim(dim)} value={score} />
))}
// ... below the bars:
{Object.entries(grade.judge_rationales ?? {}).map(([dim, text]) => (
  text ? <PerDimRationale key={dim} dim={dim} text={text} /> : null
))}
```

`raw_judge_responses` debug block stays.

`Compare` page's LLM-as-Judge section: switches from 5 hardcoded BarRow components to a generic `{Object.keys(judge_scores).map(...)}` loop. Visual layout adapts to N dims.

### 4.11 Migration of in-flight integration / Compare assertions

The Compare page currently uses `g.judge_correctness` etc. After the Grade type change, those references must move to `g.judge_scores?.correctness`. Search-and-replace per file. Same for any other site touching the old fields.

### 4.12 Tests

Each layer adds tests:

- **Storage:** `rubrics_repo_test.go` — list (returns 5 seeded), get missing → ErrNoRows, upsert + read back, delete + list (4 left).
- **HTTP:** `rubrics_handler_test.go` — GET list, POST creates, POST duplicate → 409, PUT modifies, DELETE builtin → 400, DELETE custom → 204.
- **Engine grading path:** `grader_client_judge_config_test.go` extends — buildJudgeConfig populates `Rubrics` from store.
- **Grader Python:** `test_judge.py` adapted for variable-N rubrics: tests pass a custom rubrics list, assert scores/rationales maps have those keys.
- **Composite:** new pytest for `compute_composite` with N>5 dims, with 0 dims, and with the old code-only fallback.
- **Frontend:** `rubrics.test.tsx` smoke test of the new page (list render, add-then-edit flow with mocked hooks). `LLMJudgeCard` test updated to provide `judge_scores` map instead of separate fields.

## 5. Testing

Each layer's tests above run as part of standard suites:

- `cd engine && go test ./...` — all packages green.
- `cd grader && uv run pytest` — count grows from 36 to ~40 (composite + rubric variable-N coverage).
- `cd frontend && npm test` — grading test + new rubrics test pass.
- `cd frontend && npm run build` — clean TS build with N-dim Grade type.

**Manual smoke:**
1. Re-run the experiment that produced the 3.0 correctness hallucination.
   - Expect correctness ≥ 7 now that the CRITICAL FACTS block is present.
   - If still wrong, switch judge model (Settings panel) to a stronger free option and re-test.
2. Open the new `/rubrics` page from the sidebar.
3. Edit the correctness rubric (e.g., add "be even stricter on off-by-one errors"), save, run another experiment, verify the new prompt is in effect (check the raw_judge_responses → system prompt should reflect the edit — or verify by reading the row in the rubrics table).
4. Click "+ Add dim", create `security` with display name "Security" and a starter prompt focused on input validation. Run another experiment, verify a 6th bar/rationale appears in the Grading Inspector.
5. Delete the custom `security` dim. Run another experiment, verify it's gone.
6. Attempt to delete a builtin (e.g., correctness) — API rejects with 400, UI hides the button.

## 6. Risks

1. **Schema rebuild dropping `judge_*` columns.** The migration drops 5 columns and replaces with 2 JSON columns. Any code (tests, scripts, SQL queries) referencing the old columns breaks. Mitigation: grep for `judge_correctness` / `judge_maintainability` / etc. across the whole repo before merging, fix every site. Tests are the safety net.
2. **Proto wire break.** Removing proto fields breaks any client built against the old proto. Frameval has only the in-tree engine + grader as clients; both ship together; risk is contained. Document in CLAUDE.md.
3. **Composite-score range shift.** Before: judge was 1 dim (correctness) summed into composite. After: judge is `mean(all dims)`. For an experiment where the 5 builtins all score 8.0, mean is 8.0 (same as before). But if a user adds 5 more dims that score lower, the judge contribution drops and composite drops. Old baselines on the same task become non-comparable. Document; recommend re-running baselines after rubric edits.
4. **Hard-rule prompt may over-correct.** The new "do NOT claim tests failed when they passed" instruction is strong. Edge case: tests technically pass but the implementation cheats (e.g., trivializes the test). The model should still be able to flag that under correctness — verify the prompt allows this nuance ("the metric wins for this specific claim, but you may still point out cheating").
5. **N concurrent calls × free-tier rate limits.** With N user-added dims, free models throttle. Mitigation: keep `asyncio.gather` but add a soft cap (e.g., max 8 concurrent dim calls; queue the rest). Defer the cap to a follow-up if it isn't a problem in practice.
6. **Storage growth.** `judge_scores` + `judge_rationales` JSON columns are larger than 5 floats. With 5 rationales × 600 chars each, ~3.5 KB per grade. Acceptable for thesis scale.
7. **Rubric prompt text length.** Builtin prompts are ~3-4 KB each. Five of them seeded in migration 016 means a one-time ~20 KB SQL insert. SQLite handles this fine.
8. **Failure classifier still uses 12-code taxonomy.** Out of scope here — failure_classifier is a different code path (multi-label classification, not per-dim scoring). The rubric editor does NOT cover the failure-code taxonomy.

## 7. Rollout

Single PR on `feature/llm-judge`, ordered:

1. **Storage** — migration 016, RubricsRepo + tests.
2. **Proto** — edit `proto/grader.proto`, run `buf generate`, commit regenerated stubs.
3. **HTTP** — rubrics handlers + router registration + tests.
4. **Engine grading path** — `Grade` model struct, `gradeFromProto`, `buildJudgeConfig`, tests.
5. **Grader Python** — `_grade_async` reads rubrics list, returns map shape; `render_user_prompt` rewrite with CRITICAL FACTS; `_SHARED_TAIL` hard rule; updated tests + composite test.
6. **Grader server.py** — adapt to new proto shape (`scores`/`rationales` maps instead of named fields).
7. **Frontend types + hooks** — Grade type change, useRubrics / useUpsertRubric / useCreateRubric / useDeleteRubric.
8. **Frontend pages** — `/rubrics` page, sidebar entry, LLMJudgeCard for map shape, Compare page for map shape.
9. **Tests pass through** — fixture data updates as needed.
10. **Docs** — CLAUDE.md note that judge dimensions are now editable from the UI.

Verify the §5 manual smoke gate before opening the PR (existing convention).
