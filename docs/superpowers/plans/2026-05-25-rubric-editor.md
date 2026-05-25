# Rubric Editor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Move judge rubrics from hardcoded Python into SQLite, expose them via a `/rubrics` editor page with add/edit/delete (including new user-defined dimensions). Switch the grade storage + proto + frontend Grade type from 5 fixed `judge_*` fields to `judge_scores: map<string,double>` + `judge_rationales: map<string,string>`. Also rewrite `render_user_prompt` with a CRITICAL FACTS block to stop the hallucination-of-metric-values failure mode.

**Architecture:** New `rubrics` table seeded with the 5 builtins; `RubricsRepo` for CRUD; `/api/config/rubrics` REST endpoints; engine populates new `JudgeConfig.rubrics` proto field per `GradeRun`; grader loops over received rubrics (falls back to defaults if none). New frontend route + sidebar entry; map-based Grade type; LLMJudgeCard and Compare adapt to iterate the maps.

**Tech Stack:** Go (engine), Python 3.11 (grader), React + TS (frontend). protobuf wire change (no external consumers).

**Spec:** [`docs/superpowers/specs/2026-05-25-rubric-editor-design.md`](../specs/2026-05-25-rubric-editor-design.md)

---

## File Structure

**Create (Go):**
- `engine/internal/storage/migrations/016_rubrics_and_judge_map.sql`
- `engine/internal/storage/rubrics_repo.go`
- `engine/internal/storage/rubrics_repo_test.go`
- `engine/internal/api/rubrics_handler.go`
- `engine/internal/api/rubrics_handler_test.go`

**Modify (Go):**
- `engine/internal/models/grade.go` (or wherever Grade is defined) — drop 5 fields, add 2 maps
- `engine/internal/experiment/grader_client.go` — populate JudgeConfig.Rubrics; gradeFromProto map read
- `engine/internal/experiment/grader_client_judge_config_test.go` — assert Rubrics populated
- `engine/internal/api/router.go` — register rubrics routes
- `engine/internal/storage/repo_test.go` or similar — anywhere old judge_* fields are used

**Modify (proto):**
- `proto/grader.proto` — add DimensionRubric; modify JudgeConfig + JudgeGradeResult
- Regenerated stubs: `engine/proto/grader_pb.go` (or wherever), `grader/proto/grader_pb2.py`, `grader/proto/grader_pb2_grpc.py`

**Modify (Python):**
- `grader/llm_judge/prompts.py` — render_user_prompt rewrite + _SHARED_TAIL hard rule
- `grader/llm_judge/grader.py` — variable-N rubrics from proto + maps return shape
- `grader/llm_judge/tests/test_judge.py` — variable-N test cases
- `grader/server.py` — adapt to new proto shape
- `grader/composite.py` — average scores map
- `grader/tests/test_composite.py` (new) — composite with N dims

**Create (Frontend):**
- `frontend/src/pages/rubrics/index.tsx`
- `frontend/src/pages/rubrics/RubricEditor.tsx`
- `frontend/src/pages/rubrics/AddRubricDialog.tsx`
- `frontend/src/pages/rubrics/rubrics.test.tsx`

**Modify (Frontend):**
- `frontend/src/lib/types.ts` — Grade map fields; new Rubric type
- `frontend/src/lib/hooks.ts` — useRubrics, useRubric, useCreateRubric, useUpsertRubric, useDeleteRubric
- `frontend/src/routes.tsx` — register `/rubrics`
- `frontend/src/components/grading-inspector/LLMJudgeCard.tsx` — iterate map
- `frontend/src/pages/diagnostic/compare.tsx` — iterate map
- `frontend/src/pages/runs/grading.test.tsx` — update mock to map shape
- Sidebar component (locate via grep) — add Rubrics entry

**Modify (docs):**
- `CLAUDE.md` — note that judge dimensions are SQLite-stored + editable from `/rubrics`

---

## Task 1: Migration 016 + RubricsRepo + tests

**Files:**
- Create: `engine/internal/storage/migrations/016_rubrics_and_judge_map.sql`
- Create: `engine/internal/storage/rubrics_repo.go`
- Create: `engine/internal/storage/rubrics_repo_test.go`

- [ ] **Step 1: Write the migration**

Create `engine/internal/storage/migrations/016_rubrics_and_judge_map.sql`. The full prompt text for the 5 builtin rubrics must come from `grader/llm_judge/prompts.py`'s current `DIMENSION_RUBRICS` dict — copy each value EXACTLY into the SQL (escape single quotes by doubling them).

```sql
CREATE TABLE rubrics (
  key          TEXT PRIMARY KEY,
  display_name TEXT NOT NULL,
  prompt       TEXT NOT NULL,
  sort_order   INTEGER NOT NULL DEFAULT 0,
  is_builtin   INTEGER NOT NULL DEFAULT 0,
  created_at   DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
  updated_at   DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

INSERT INTO rubrics (key, display_name, prompt, sort_order, is_builtin) VALUES
  ('correctness',     'Correctness',     '<paste full prompt from DIMENSION_RUBRICS["correctness"]>',     1, 1),
  ('maintainability', 'Maintainability', '<paste full prompt from DIMENSION_RUBRICS["maintainability"]>', 2, 1),
  ('completeness',    'Completeness',    '<paste full prompt from DIMENSION_RUBRICS["completeness"]>',    3, 1),
  ('best_practices',  'Best practices',  '<paste full prompt from DIMENSION_RUBRICS["best_practices"]>',  4, 1),
  ('error_handling',  'Error handling',  '<paste full prompt from DIMENSION_RUBRICS["error_handling"]>',  5, 1);

-- Move judge storage from 5 hardcoded columns to JSON maps.
ALTER TABLE grades ADD COLUMN judge_scores TEXT;
ALTER TABLE grades ADD COLUMN judge_rationales TEXT;

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

-- Drop the 5 old columns via standard SQLite table rebuild.
-- READ the existing CREATE TABLE grades from migration 001 first;
-- the rebuild must preserve EVERY other column. Steps:
-- 1) CREATE TABLE grades_new (...) without the 5 old judge_* cols
-- 2) INSERT INTO grades_new SELECT <all other cols> FROM grades
-- 3) DROP TABLE grades
-- 4) ALTER TABLE grades_new RENAME TO grades
-- 5) Recreate any indexes that existed on grades (grep migration 001)
```

When pasting the rubric prompt text, escape single quotes by doubling them (`'` → `''`). SQLite is strict.

- [ ] **Step 2: Confirm migration applies cleanly**

`cd engine && go test ./internal/storage/... 2>&1 | tail -10`
Expected: all existing tests still PASS. The grade-row tests will surface any reference to dropped columns; fix the test refs in those files.

- [ ] **Step 3: Write the failing repo tests**

`engine/internal/storage/rubrics_repo_test.go`:

```go
package storage

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestRubrics_ListSeedHas5Builtins(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	rows, err := store.ListRubrics(ctx)
	if err != nil { t.Fatalf("ListRubrics: %v", err) }
	if len(rows) != 5 {
		t.Errorf("got %d rubrics, want 5 seeded", len(rows))
	}
	keys := map[string]bool{}
	for _, r := range rows {
		keys[r.Key] = true
		if !r.IsBuiltin { t.Errorf("rubric %q should be builtin", r.Key) }
	}
	for _, want := range []string{"correctness", "maintainability", "completeness", "best_practices", "error_handling"} {
		if !keys[want] { t.Errorf("missing seeded rubric %q", want) }
	}
}

func TestRubrics_GetMissingReturnsErrNoRows(t *testing.T) {
	store := newTestStore(t)
	_, err := store.GetRubric(context.Background(), "nope")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("want sql.ErrNoRows, got %v", err)
	}
}

func TestRubrics_UpsertNewThenReadBack(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	r := Rubric{Key: "security", DisplayName: "Security", Prompt: "score security only", SortOrder: 99, IsBuiltin: false}
	if err := store.UpsertRubric(ctx, r); err != nil { t.Fatalf("upsert: %v", err) }
	got, err := store.GetRubric(ctx, "security")
	if err != nil { t.Fatalf("get: %v", err) }
	if got.DisplayName != "Security" || got.Prompt != "score security only" || got.IsBuiltin {
		t.Errorf("roundtrip mismatch: %+v", got)
	}
}

func TestRubrics_UpsertExistingPreservesIsBuiltin(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Edit a builtin rubric — display_name change should stick, is_builtin must remain 1.
	r := Rubric{Key: "correctness", DisplayName: "Correctness (edited)", Prompt: "new prompt body", IsBuiltin: false}
	if err := store.UpsertRubric(ctx, r); err != nil { t.Fatalf("upsert: %v", err) }
	got, err := store.GetRubric(ctx, "correctness")
	if err != nil { t.Fatalf("get: %v", err) }
	if !got.IsBuiltin {
		t.Errorf("is_builtin must be preserved as true on upsert of a builtin row")
	}
	if got.Prompt != "new prompt body" {
		t.Errorf("prompt not updated")
	}
}

func TestRubrics_DeleteRemovesRow(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	_ = store.UpsertRubric(ctx, Rubric{Key: "tmp", DisplayName: "Tmp", Prompt: "x", IsBuiltin: false})
	if err := store.DeleteRubric(ctx, "tmp"); err != nil { t.Fatalf("delete: %v", err) }
	_, err := store.GetRubric(ctx, "tmp")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected ErrNoRows after delete, got %v", err)
	}
}
```

Run: `cd engine && go test ./internal/storage/ -run TestRubrics -v` → expect FAIL with "undefined: ListRubrics".

- [ ] **Step 4: Implement `rubrics_repo.go`**

```go
package storage

import (
	"context"
	"database/sql"
	"fmt"
)

type Rubric struct {
	Key         string
	DisplayName string
	Prompt      string
	SortOrder   int
	IsBuiltin   bool
	CreatedAt   string
	UpdatedAt   string
}

func (s *Store) ListRubrics(ctx context.Context) ([]Rubric, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT key, display_name, prompt, sort_order, is_builtin, created_at, updated_at
		FROM rubrics ORDER BY sort_order, key`)
	if err != nil { return nil, fmt.Errorf("list rubrics: %w", err) }
	defer rows.Close()
	out := make([]Rubric, 0)
	for rows.Next() {
		var r Rubric
		var isB int
		if err := rows.Scan(&r.Key, &r.DisplayName, &r.Prompt, &r.SortOrder, &isB, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan rubric: %w", err)
		}
		r.IsBuiltin = isB == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetRubric(ctx context.Context, key string) (Rubric, error) {
	var r Rubric
	var isB int
	err := s.DB.QueryRowContext(ctx, `
		SELECT key, display_name, prompt, sort_order, is_builtin, created_at, updated_at
		FROM rubrics WHERE key = ?`, key).
		Scan(&r.Key, &r.DisplayName, &r.Prompt, &r.SortOrder, &isB, &r.CreatedAt, &r.UpdatedAt)
	if err != nil { return Rubric{}, err }
	r.IsBuiltin = isB == 1
	return r, nil
}

// UpsertRubric inserts or updates by key. is_builtin from the input is
// IGNORED on update — once a row is builtin it stays builtin (only a
// new INSERT can set is_builtin=true, and the API never does that).
func (s *Store) UpsertRubric(ctx context.Context, r Rubric) error {
	isB := 0
	if r.IsBuiltin { isB = 1 }
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO rubrics (key, display_name, prompt, sort_order, is_builtin)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			display_name = excluded.display_name,
			prompt = excluded.prompt,
			sort_order = excluded.sort_order,
			-- is_builtin is intentionally NOT updated; preserves existing value
			updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
	`, r.Key, r.DisplayName, r.Prompt, r.SortOrder, isB)
	if err != nil { return fmt.Errorf("upsert rubric %q: %w", r.Key, err) }
	return nil
}

func (s *Store) DeleteRubric(ctx context.Context, key string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM rubrics WHERE key = ?`, key)
	if err != nil { return fmt.Errorf("delete rubric %q: %w", key, err) }
	return nil
}

var _ = sql.ErrNoRows
```

- [ ] **Step 5: Run tests**

`cd engine && go test ./internal/storage/ -run TestRubrics -v` → all 5 PASS.

- [ ] **Step 6: Fix any storage references to dropped judge_* columns**

`cd engine && go build ./... 2>&1 | tail -30`

Expected build errors: anywhere the old `judge_correctness` etc. fields on `models.Grade` were read or written. We'll handle the Grade model change in Task 4; for now, get the storage layer compiling. If `grade_repo.go` or similar references `JudgeCorrectness`, comment them out with `// TODO: rubric-editor PR` and fix in Task 4.

- [ ] **Step 7: Commit**

```bash
git add engine/internal/storage/migrations/016_rubrics_and_judge_map.sql \
        engine/internal/storage/rubrics_repo.go \
        engine/internal/storage/rubrics_repo_test.go
# Plus any storage files you commented out above.
git commit -m "Add rubrics table + RubricsRepo; drop hardcoded judge_* columns on grades"
```

---

## Task 2: Proto edit + regen stubs

**Files:**
- Modify: `proto/grader.proto`
- Regenerate: `engine/proto/grader*.go`, `grader/proto/grader_pb2.py`, `grader/proto/grader_pb2_grpc.py`

- [ ] **Step 1: Edit `proto/grader.proto`**

Find `message JudgeGradeResult { ... }`. Replace with:

```protobuf
message JudgeGradeResult {
  map<string, double> scores = 1;
  map<string, string> rationales = 2;
  double irr_alpha = 3;
  repeated string raw_responses = 4;
}
```

(Removes the 5 named fields. Field numbers are renumbered; this is a wire break but we ship engine+grader together.)

Find `message JudgeConfig { ... }`. Add a new field:

```protobuf
message DimensionRubric {
  string key = 1;
  string prompt = 2;
}

message JudgeConfig {
  string model = 1;
  string provider = 2;
  string api_key = 3;
  repeated RubricDimension rubric = 4;  // pre-existing legacy field; leave as-is
  int32 judge_rounds = 5;
  repeated DimensionRubric rubrics = 6; // NEW
}
```

(`RubricDimension` is the old unused name; keep it for proto-tag stability. `DimensionRubric` is the new message we'll actually use.)

- [ ] **Step 2: Regenerate stubs**

Per CLAUDE.md: `cd proto && buf generate`

Verify both sides updated:
- `engine/proto/grader.pb.go` — should have a `Rubrics []*DimensionRubric` field on JudgeConfig and `Scores map[string]float64` + `Rationales map[string]string` on JudgeGradeResult.
- `grader/proto/grader_pb2.py` — should have these too (verify by importing in a quick `python -c`).

- [ ] **Step 3: Build engine — expect many errors**

`cd engine && go build ./... 2>&1 | head -40`

The errors point at every call site that used the old proto fields. These are fixed in Tasks 4-6. For now just commit the proto change.

- [ ] **Step 4: Commit**

```bash
git add proto/grader.proto engine/proto/ grader/proto/
git commit -m "Switch JudgeGradeResult to map shape; add DimensionRubric to JudgeConfig"
```

---

## Task 3: Rubrics HTTP CRUD + tests

**Files:**
- Create: `engine/internal/api/rubrics_handler.go`
- Create: `engine/internal/api/rubrics_handler_test.go`
- Modify: `engine/internal/api/router.go`

- [ ] **Step 1: Write the failing handler tests**

Tests cover: GET list, POST creates new, POST duplicate key → 409, PUT modifies, DELETE builtin → 400, DELETE custom → 204, key validation.

`engine/internal/api/rubrics_handler_test.go`:

```go
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRubrics_GetListIncludes5Seeded(t *testing.T) {
	srv := newTestServer(t)
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/config/rubrics", nil))
	if w.Code != 200 { t.Fatalf("status %d body %s", w.Code, w.Body.String()) }
	var got []map[string]any
	_ = json.NewDecoder(w.Body).Decode(&got)
	if len(got) != 5 { t.Errorf("got %d, want 5", len(got)) }
}

func TestRubrics_PostCreatesNew(t *testing.T) {
	srv := newTestServer(t)
	body, _ := json.Marshal(map[string]string{
		"key": "security", "display_name": "Security",
		"prompt": "score security: input validation, secrets, dep risks. " + strings.Repeat("x", 100),
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/config/rubrics", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	srv.Handler.ServeHTTP(w, r)
	if w.Code != 201 { t.Fatalf("status %d body %s", w.Code, w.Body.String()) }
}

func TestRubrics_PostDuplicateReturns409(t *testing.T) {
	srv := newTestServer(t)
	body, _ := json.Marshal(map[string]string{
		"key": "correctness", "display_name": "x",
		"prompt": strings.Repeat("p", 100),
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/config/rubrics", bytes.NewReader(body))
	srv.Handler.ServeHTTP(w, r)
	if w.Code != 409 { t.Errorf("want 409, got %d", w.Code) }
}

func TestRubrics_DeleteBuiltinReturns400(t *testing.T) {
	srv := newTestServer(t)
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/config/rubrics/correctness", nil))
	if w.Code != 400 { t.Errorf("want 400, got %d", w.Code) }
}

func TestRubrics_DeleteCustomReturns204(t *testing.T) {
	srv := newTestServer(t)
	// First POST a custom rubric.
	body, _ := json.Marshal(map[string]string{
		"key": "tmp", "display_name": "Tmp", "prompt": strings.Repeat("p", 100),
	})
	r := httptest.NewRequest(http.MethodPost, "/api/config/rubrics", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, r)
	if w.Code != 201 { t.Fatalf("create failed: %d %s", w.Code, w.Body.String()) }
	// Then DELETE.
	w = httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/config/rubrics/tmp", nil))
	if w.Code != 204 { t.Errorf("want 204, got %d", w.Code) }
}

func TestRubrics_PostInvalidKeyReturns400(t *testing.T) {
	srv := newTestServer(t)
	body, _ := json.Marshal(map[string]string{
		"key": "Bad-Key!", "display_name": "x", "prompt": strings.Repeat("p", 100),
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/config/rubrics", bytes.NewReader(body))
	srv.Handler.ServeHTTP(w, r)
	if w.Code != 400 { t.Errorf("want 400, got %d", w.Code) }
}
```

Need `import "strings"` — add it.

`cd engine && go test ./internal/api/ -run TestRubrics -v` → FAIL (handlers don't exist yet).

- [ ] **Step 2: Implement handlers**

Append to a new file `engine/internal/api/rubrics_handler.go`:

```go
package api

import (
	"errors"
	"net/http"
	"regexp"

	"database/sql"

	"github.com/go-chi/chi/v5"
	"github.com/mustafaselman/frameval/engine/internal/storage"
)

var rubricKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,40}$`)

type rubricPayload struct {
	Key         string `json:"key"`
	DisplayName string `json:"display_name"`
	Prompt      string `json:"prompt"`
	SortOrder   int    `json:"sort_order,omitempty"`
	IsBuiltin   bool   `json:"is_builtin,omitempty"`
}

func (s *Service) ListRubrics(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.ListRubrics(r.Context())
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusOK, rows)
}

func (s *Service) GetRubric(w http.ResponseWriter, r *http.Request) {
	got, err := s.store.GetRubric(r.Context(), chi.URLParam(r, "key"))
	if errors.Is(err, sql.ErrNoRows) {
		renderError(w, r.Context(), http.StatusNotFound, ErrCodeNotFound, "rubric not found", err)
		return
	}
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusOK, got)
}

func (s *Service) CreateRubric(w http.ResponseWriter, r *http.Request) {
	var p rubricPayload
	if err := decodeJSON(r, &p); err != nil {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "invalid request", err)
		return
	}
	if !rubricKeyPattern.MatchString(p.Key) {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "invalid key (lowercase snake_case, 2-41 chars)", nil)
		return
	}
	if len(p.DisplayName) < 1 || len(p.DisplayName) > 80 {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "display_name must be 1-80 chars", nil)
		return
	}
	if len(p.Prompt) < 50 || len(p.Prompt) > 20000 {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "prompt must be 50-20000 chars", nil)
		return
	}
	if _, err := s.store.GetRubric(r.Context(), p.Key); err == nil {
		renderError(w, r.Context(), http.StatusConflict, ErrCodeConflict, "rubric with this key already exists", nil)
		return
	}
	row := storage.Rubric{
		Key: p.Key, DisplayName: p.DisplayName, Prompt: p.Prompt,
		SortOrder: p.SortOrder, IsBuiltin: false, // user-created rubrics never builtin
	}
	if err := s.store.UpsertRubric(r.Context(), row); err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusCreated, row)
}

func (s *Service) UpdateRubric(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	existing, err := s.store.GetRubric(r.Context(), key)
	if errors.Is(err, sql.ErrNoRows) {
		renderError(w, r.Context(), http.StatusNotFound, ErrCodeNotFound, "rubric not found", err)
		return
	}
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	var p rubricPayload
	if err := decodeJSON(r, &p); err != nil {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "invalid request", err)
		return
	}
	if len(p.DisplayName) >= 1 && len(p.DisplayName) <= 80 {
		existing.DisplayName = p.DisplayName
	}
	if len(p.Prompt) >= 50 && len(p.Prompt) <= 20000 {
		existing.Prompt = p.Prompt
	}
	if p.SortOrder > 0 {
		existing.SortOrder = p.SortOrder
	}
	if err := s.store.UpsertRubric(r.Context(), existing); err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusOK, existing)
}

func (s *Service) DeleteRubricHandler(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	existing, err := s.store.GetRubric(r.Context(), key)
	if errors.Is(err, sql.ErrNoRows) {
		w.WriteHeader(http.StatusNoContent) // idempotent delete
		return
	}
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	if existing.IsBuiltin {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "builtin rubric cannot be deleted", nil)
		return
	}
	if err := s.store.DeleteRubric(r.Context(), key); err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

If `ErrCodeConflict` or `ErrCodeNotFound` aren't already defined, add them to the existing error-codes file (find via `grep -rn "ErrCodeBadRequest" engine/internal/api/`).

- [ ] **Step 3: Register routes in `router.go`**

After the existing `/config/llm-settings` lines:
```go
r.Get("/config/rubrics", service.ListRubrics)
r.Post("/config/rubrics", service.CreateRubric)
r.Get("/config/rubrics/{key}", service.GetRubric)
r.Put("/config/rubrics/{key}", service.UpdateRubric)
r.Delete("/config/rubrics/{key}", service.DeleteRubricHandler)
```

- [ ] **Step 4: Run tests**

`cd engine && go test ./internal/api/ -run TestRubrics -v` → all 6 PASS.

- [ ] **Step 5: Commit**

```bash
git add engine/internal/api/rubrics_handler.go engine/internal/api/rubrics_handler_test.go engine/internal/api/router.go engine/internal/api/errors.go # if errors file updated
git commit -m "Add /api/config/rubrics CRUD handlers with key validation"
```

---

## Task 4: Engine Grade model + grader_client map handling

**Files:**
- Modify: `engine/internal/models/grade.go` (find via grep)
- Modify: `engine/internal/experiment/grader_client.go`
- Modify: `engine/internal/experiment/grader_client_judge_config_test.go`
- Modify: any storage / repo files that read/write the old judge_* fields

- [ ] **Step 1: Update the Grade struct**

`grep -rn "JudgeCorrectness" engine/` to find the Grade struct's location.

Drop these 5 fields:
- `JudgeCorrectness float64`
- `JudgeMaintainability float64`
- `JudgeCompleteness float64`
- `JudgeBestPractices float64`
- `JudgeErrorHandling float64`

Add:
- `JudgeScores map[string]float64 \`json:"judge_scores,omitempty"\``
- `JudgeRationales map[string]string \`json:"judge_rationales,omitempty"\``

Keep `JudgeIRRAlpha float64` and `RawJudgeResponses []string`.

- [ ] **Step 2: Fix all readers/writers**

`grep -rn "JudgeCorrectness\|JudgeMaintainability\|JudgeCompleteness\|JudgeBestPractices\|JudgeErrorHandling" engine/ --include="*.go"` — every reference must be replaced with:

For reads: `grade.JudgeScores["correctness"]` etc.
For writes (storage): persist `JudgeScores` and `JudgeRationales` as JSON via `json.Marshal`. The SQLite columns now hold JSON strings.

For the `engine/internal/experiment/grader_client.go:gradeFromProto` function:

```go
if response.Judge != nil {
    grade.JudgeScores = make(map[string]float64, len(response.Judge.Scores))
    for k, v := range response.Judge.Scores {
        grade.JudgeScores[k] = v
    }
    grade.JudgeRationales = make(map[string]string, len(response.Judge.Rationales))
    for k, v := range response.Judge.Rationales {
        grade.JudgeRationales[k] = v
    }
    grade.JudgeIRRAlpha = float64(response.Judge.IrrAlpha)
    grade.RawJudgeResponses = response.Judge.RawResponses
}
```

For `fallbackGrade`: drop the per-dim zero assignments, leave the JudgeScores nil (gets serialized as `null` JSON, which matches the spec's empty-grade contract).

- [ ] **Step 3: Populate JudgeConfig.Rubrics**

In `buildJudgeConfig`:

```go
rubricRows, _ := c.settings.ListRubrics(ctx)
rubricsProto := make([]*graderpb.DimensionRubric, 0, len(rubricRows))
for _, r := range rubricRows {
    rubricsProto = append(rubricsProto, &graderpb.DimensionRubric{
        Key: r.Key, Prompt: r.Prompt,
    })
}
return &graderpb.JudgeConfig{
    Provider: provider, Model: settings["judge.model"], ApiKey: apiKey,
    JudgeRounds: 1, Rubrics: rubricsProto,
}
```

Extend the `SettingsStore` interface in the same file with `ListRubrics(ctx context.Context) ([]Rubric, error)` — and import the `Rubric` type. (May need an alias since `Rubric` lives in the `storage` package — use a type re-export, OR define a minimal `SettingsRubric struct{Key, Prompt string}` in the experiment package and convert.)

Choose: define a local `type RubricRow struct { Key, Prompt string }` in `grader_client.go` and have `SettingsStore.ListRubrics` return `[]RubricRow`. Adapt `*storage.Store` to satisfy this via a thin wrapper, OR add another method on Store that returns the narrower shape. Either works; pick the simplest.

- [ ] **Step 4: Extend `grader_client_judge_config_test.go`**

Add a test:

```go
func TestBuildJudgeConfig_PopulatesRubrics(t *testing.T) {
	store := support.TmpStore(t)
	ctx := context.Background()
	_ = store.SetSetting(ctx, "judge.enabled", "true")
	_ = store.SetSetting(ctx, "judge.provider", "openrouter")
	_ = store.SetSetting(ctx, "judge.model", "x")

	cfg := BuildJudgeConfigForTest(ctx, store)
	if cfg == nil { t.Fatal("expected non-nil cfg") }
	if len(cfg.Rubrics) != 5 {
		t.Errorf("want 5 seeded rubrics, got %d", len(cfg.Rubrics))
	}
	// Check the correctness rubric is present with a non-empty prompt.
	found := false
	for _, r := range cfg.Rubrics {
		if r.Key == "correctness" && len(r.Prompt) > 50 {
			found = true; break
		}
	}
	if !found { t.Errorf("correctness rubric missing or empty prompt") }
}
```

- [ ] **Step 5: Build clean + tests pass**

```bash
cd engine && go build ./... 2>&1 | tail -10
cd engine && go test ./... 2>&1 | tail -10
```

Both should be clean. If a storage repo serializing the Grade needs updating, that's expected; fix.

- [ ] **Step 6: Commit**

```bash
git add -A engine/
git commit -m "Grade uses map shape; grader_client populates JudgeConfig.Rubrics from SQLite"
```

---

## Task 5: Grader Python — composite + render_user_prompt rewrite + _SHARED_TAIL hard rule

**Files:**
- Modify: `grader/composite.py`
- Modify: `grader/llm_judge/prompts.py`
- Create: `grader/tests/test_composite.py`

- [ ] **Step 1: Update `compute_composite` for map-based judge**

In `grader/composite.py`, replace `compute_composite`:

```python
def compute_composite(code_grade, process_grade, judge_grade=None, adherence_grade=None):
    code_score = float(code_grade.get("test_pass_rate", 0.0)) * 10
    process_score = compute_process_score(process_grade)
    if judge_grade is None and adherence_grade is None:
        return round((code_score * 0.6) + (process_score * 0.4), 4)
    scores = (judge_grade or {}).get("scores") or {}
    judge_score = (sum(scores.values()) / len(scores)) if scores else 0.0
    adherence_score = float((adherence_grade or {}).get("instruction_compliance", 0.0))
    return round((code_score * 0.3) + (judge_score * 0.3) + (process_score * 0.2) + (adherence_score * 0.2), 4)
```

- [ ] **Step 2: Create `grader/tests/test_composite.py`**

```python
from grader.composite import compute_composite

def test_no_judge_no_adherence_uses_60_40():
    out = compute_composite({"test_pass_rate": 1.0}, {})
    # code=10 * 0.6 + process_score * 0.4 (process_score=0 here)
    assert out == 6.0

def test_judge_with_one_dim_averages_correctly():
    out = compute_composite(
        {"test_pass_rate": 1.0},
        {"self_validation_rate": 0.5, "token_efficiency": 0.4, "context_utilization": 0.5},
        judge_grade={"scores": {"correctness": 8.0}},
    )
    # mean({correctness: 8}) = 8; 10*0.3 + 8*0.3 + process_score*0.2 + 0*0.2
    assert out > 0

def test_judge_with_five_dims_averages_them():
    judge = {"scores": {"correctness": 8.0, "maintainability": 7.0, "completeness": 9.0, "best_practices": 6.0, "error_handling": 5.0}}
    out = compute_composite({"test_pass_rate": 1.0}, {}, judge_grade=judge)
    # mean = 7.0; 10*0.3 + 7*0.3 + 0 + 0 = 5.1
    assert abs(out - 5.1) < 0.01

def test_judge_with_n_dims_averages_them():
    judge = {"scores": {f"dim{i}": 10.0 for i in range(20)}}
    out = compute_composite({"test_pass_rate": 0.0}, {}, judge_grade=judge)
    # 0 + 10 * 0.3 + 0 + 0 = 3.0
    assert abs(out - 3.0) < 0.01

def test_judge_empty_scores_treated_as_zero():
    out = compute_composite({"test_pass_rate": 0.0}, {}, judge_grade={"scores": {}})
    assert out == 0.0
```

- [ ] **Step 3: Run composite tests**

`cd grader && uv run pytest tests/test_composite.py -v` → 5 PASS.

- [ ] **Step 4: Rewrite `render_user_prompt` with CRITICAL FACTS**

In `grader/llm_judge/prompts.py`, replace the existing `render_user_prompt` function:

```python
def render_user_prompt(
    *,
    code_grade,
    process_grade,
    task,
    output_files,
    transcript_json,
) -> str:
    pass_rate = float(code_grade.get("test_pass_rate", 0.0))
    pass_count = int(code_grade.get("test_pass_count", 0))
    fail_count = int(code_grade.get("test_fail_count", 0))
    total = pass_count + fail_count
    type_check = bool(code_grade.get("type_check_pass", False))
    lint = float(code_grade.get("lint_score", 0.0))
    premature = bool(process_grade.get("premature_completion", False))

    facts = (
        "# CRITICAL FACTS\n\n"
        "These are authoritative measurements taken from the actual run. "
        "If your rationale contradicts any fact below, the fact wins — "
        "you must reflect it in your score and rationale.\n\n"
        f"- TESTS: {pass_count} / {total if total else '?'} passed (test_pass_rate = {pass_rate:.2f}).\n"
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

- [ ] **Step 5: Add the hard rule to `_SHARED_TAIL`**

Append to `_SHARED_TAIL`:

```python
_SHARED_TAIL = """## Output format

...existing text...

## Score anchors (use these to calibrate)

...existing text...

## Hard rule on facts

If the CRITICAL FACTS section of the user prompt says tests passed
(pass_count > 0 with fail_count = 0), you MUST NOT claim tests failed in
your rationale. If TYPE CHECK is PASS, you MUST NOT claim it failed. If
PREMATURE COMPLETION is NO, you MUST NOT claim the agent stopped early.
The facts are authoritative — your rationale must reflect them. You may
still penalize on substance (e.g., trivial tests that pass via a cheat),
but you may not invent contrary metric values."""
```

- [ ] **Step 6: Commit**

```bash
git add grader/composite.py grader/llm_judge/prompts.py grader/tests/test_composite.py
git commit -m "Composite averages judge scores map; render_user_prompt shouts CRITICAL FACTS"
```

---

## Task 6: Grader Python — judge.grade reads rubrics from proto + maps return shape

**Files:**
- Modify: `grader/llm_judge/grader.py`
- Modify: `grader/llm_judge/tests/test_judge.py`
- Modify: `grader/server.py`

- [ ] **Step 1: Update `grade()` signature + `_grade_async`**

Edit `grader/llm_judge/grader.py`. The current `grade()` signature stays the same EXCEPT it must now accept `rubrics` either through `config_override.rubrics` (a list of proto DimensionRubric) or fall back to defaults from `DIMENSION_RUBRICS`.

Replace the function body's "tasks" construction to use received rubrics:

```python
def grade(code_grade, process_grade, task=None, output_files=None,
          transcript_json=None, config_override=None) -> dict:
    try:
        cfg = load_config(config_override)
    except Exception as exc:
        logger.warning("judge config load failed: %s", exc)
        return _all_dims_failed(str(exc), _builtin_rubrics())
    rubrics_from_request = _extract_rubrics(config_override)
    effective = rubrics_from_request or _builtin_rubrics()
    return asyncio.run(_grade_async(cfg, code_grade, process_grade, task, output_files, transcript_json, effective))


def _extract_rubrics(config_override) -> list[tuple[str, str]] | None:
    """Pull (key, prompt) pairs out of a JudgeConfig proto's rubrics field.
    Returns None if config_override is missing or has no rubrics."""
    if config_override is None:
        return None
    rubrics_attr = getattr(config_override, "rubrics", None)
    if not rubrics_attr:
        return None
    return [(r.key, r.prompt) for r in rubrics_attr if r.key and r.prompt]


def _builtin_rubrics() -> list[tuple[str, str]]:
    from grader.llm_judge.prompts import DIMENSION_RUBRICS
    return list(DIMENSION_RUBRICS.items())


async def _grade_async(cfg, code_grade, process_grade, task, output_files, transcript_json, rubrics):
    try:
        client = build_client(cfg, async_client=True)
    except Exception as exc:
        logger.warning("judge async client init failed: %s", exc)
        return _all_dims_failed(str(exc), rubrics)

    user_prompt = render_user_prompt(
        code_grade=code_grade, process_grade=process_grade,
        task=task or {}, output_files=output_files or [],
        transcript_json=transcript_json or b"",
    )
    tasks = [_score_one_dim(client, cfg.model, key, prompt, user_prompt) for key, prompt in rubrics]
    results = await asyncio.gather(*tasks, return_exceptions=False)

    scores = {rubrics[i][0]: results[i][0] for i in range(len(rubrics))}
    rationales = {rubrics[i][0]: results[i][1] for i in range(len(rubrics))}
    raw_responses = [results[i][2] for i in range(len(rubrics))]
    return {
        "scores": scores,
        "rationales": rationales,
        "irr_alpha": 0.0,
        "raw_responses": raw_responses,
    }


async def _score_one_dim(client, model: str, key: str, prompt: str, user_prompt: str) -> tuple[float, str, str]:
    """Returns (score, rationale, raw_response_string).
    Never raises — failures yield 0.0 + sentinel rationale + sentinel raw_response."""
    try:
        verdict: DimensionVerdict = await client.create(
            model=model,
            response_model=DimensionVerdict,
            max_retries=2,
            max_tokens=512,
            messages=[
                {"role": "system", "content": prompt},
                {"role": "user", "content": user_prompt},
            ],
        )
        return verdict.score, verdict.rationale, _tag_response(key, verdict.model_dump_json())
    except Exception as exc:
        logger.warning("judge dim=%s call failed: %s", key, exc)
        sentinel = f"judge_unavailable: {str(exc)[:300]}"
        return 0.0, sentinel, _tag_response(key, sentinel)


def _all_dims_failed(reason: str, rubrics: list[tuple[str, str]]) -> dict:
    short = reason[:300]
    return {
        "scores": {key: 0.0 for key, _ in rubrics},
        "rationales": {key: f"judge_unavailable: {short}" for key, _ in rubrics},
        "irr_alpha": 0.0,
        "raw_responses": [f"dim={key};judge_unavailable: {short}" for key, _ in rubrics],
    }
```

The old `_DIMENSIONS` constant is GONE.

- [ ] **Step 2: Update `server.py`**

The grader's `GradeRun` handler must:
- Continue to call `judge_grade(...)` with the same args (the function pulls rubrics out of config_override)
- Build the `JudgeGradeResult` from the new `scores` + `rationales` maps:

```python
judge = judge_grade(code, process, task=task, output_files=output_files,
                    transcript_json=request.transcript_json.encode(),
                    config_override=judge_cfg)
# ... in the response build:
judge_pb = grader_pb2.JudgeGradeResult(
    scores=judge["scores"],
    rationales=judge["rationales"],
    irr_alpha=judge["irr_alpha"],
    raw_responses=judge["raw_responses"],
)
```

`disabled_judge_result()` likewise returns the new dict shape:

```python
def disabled_judge_result() -> dict:
    return {
        "scores": {},
        "rationales": {},
        "irr_alpha": 0.0,
        "raw_responses": ["llm_judge_disabled"],
    }
```

- [ ] **Step 3: Update `test_judge.py`**

Replace the existing tests with variable-N versions:

```python
import json
from types import SimpleNamespace
from unittest.mock import patch

import pytest

from grader.llm_judge.grader import DimensionVerdict, grade


class _AsyncStub:
    def __init__(self, by_key): self.by_key = by_key
    async def create(self, *, model, response_model, max_retries, max_tokens, messages):
        sys = messages[0]["content"]
        for key, result in self.by_key.items():
            tag = f"**{key.upper().replace('_', ' ')}**"
            if tag in sys:
                if isinstance(result, Exception): raise result
                return result
        raise AssertionError(f"no key matched in system prompt: {sys[:80]}")


def _v(score, rationale="ok"): return DimensionVerdict(score=score, rationale=rationale)


def _proto_rubrics(*keys_and_prompts):
    return SimpleNamespace(
        provider="", model="", api_key="",
        rubrics=[SimpleNamespace(key=k, prompt=p) for k, p in keys_and_prompts],
    )


def test_grade_uses_rubrics_from_proto():
    stub = _AsyncStub({"foo": _v(8.0, "rfoo"), "bar": _v(7.0, "rbar")})
    cfg = _proto_rubrics(("foo", "scoring **FOO**"), ("bar", "scoring **BAR**"))
    with patch("grader.llm_judge.grader.build_client", return_value=stub):
        out = grade({}, {}, task={}, output_files=[], transcript_json=b"", config_override=cfg)
    assert out["scores"] == {"foo": 8.0, "bar": 7.0}
    assert out["rationales"] == {"foo": "rfoo", "bar": "rbar"}
    assert len(out["raw_responses"]) == 2


def test_grade_falls_back_to_builtin_rubrics_when_none_supplied():
    stub = _AsyncStub({
        "correctness": _v(8.0), "maintainability": _v(7.0), "completeness": _v(9.0),
        "best_practices": _v(6.0), "error_handling": _v(5.0),
    })
    with patch("grader.llm_judge.grader.build_client", return_value=stub):
        out = grade({}, {}, task={}, output_files=[], transcript_json=b"", config_override=None)
    assert len(out["scores"]) == 5
    assert "correctness" in out["scores"]


def test_grade_one_dim_fails_others_succeed():
    stub = _AsyncStub({
        "correctness": RuntimeError("boom"),
        "maintainability": _v(7.0), "completeness": _v(8.0),
        "best_practices": _v(6.0), "error_handling": _v(5.0),
    })
    with patch("grader.llm_judge.grader.build_client", return_value=stub):
        out = grade({}, {}, task={}, output_files=[], transcript_json=b"", config_override=None)
    assert out["scores"]["correctness"] == 0.0
    assert "judge_unavailable: boom" in out["rationales"]["correctness"]
    assert out["scores"]["maintainability"] == 7.0


def test_grade_client_init_failure_returns_all_failed():
    cfg = _proto_rubrics(("alpha", "**ALPHA**"), ("beta", "**BETA**"))
    with patch("grader.llm_judge.grader.build_client", side_effect=RuntimeError("no key")):
        out = grade({}, {}, task={}, output_files=[], transcript_json=b"", config_override=cfg)
    assert out["scores"] == {"alpha": 0.0, "beta": 0.0}
    assert all("judge_unavailable: no key" in r for r in out["rationales"].values())
```

- [ ] **Step 4: Run grader suite**

`cd grader && uv run pytest 2>&1 | tail -10` → all PASS (count rises again).

- [ ] **Step 5: Commit**

```bash
git add grader/llm_judge/grader.py grader/llm_judge/tests/test_judge.py grader/server.py
git commit -m "Grader judge reads rubrics from proto; returns scores/rationales maps"
```

---

## Task 7: Frontend types + new rubrics hooks

**Files:**
- Modify: `frontend/src/lib/types.ts`
- Modify: `frontend/src/lib/hooks.ts`

- [ ] **Step 1: Update Grade type**

In `frontend/src/lib/types.ts`, find the `Grade` type. Remove:
- `judge_correctness`, `judge_maintainability`, `judge_completeness`, `judge_best_practices`, `judge_error_handling`

Add:
- `judge_scores?: Record<string, number>;`
- `judge_rationales?: Record<string, string>;`

Keep `judge_irr_alpha` and `raw_judge_responses`.

Also add the Rubric type:
```typescript
export type Rubric = {
  key: string;
  display_name: string;
  prompt: string;
  sort_order: number;
  is_builtin: boolean;
  created_at?: string;
  updated_at?: string;
};
```

- [ ] **Step 2: Add hooks**

Append to `frontend/src/lib/hooks.ts`:

```typescript
export function useRubrics() {
  return useQuery({
    queryKey: ['config', 'rubrics'],
    queryFn: () => api.get<Rubric[]>('/config/rubrics'),
  });
}

export function useRubric(key?: string) {
  return useQuery({
    queryKey: ['config', 'rubrics', key],
    enabled: Boolean(key),
    queryFn: () => api.get<Rubric>(`/config/rubrics/${key}`),
  });
}

export function useCreateRubric() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (payload: Pick<Rubric, 'key' | 'display_name' | 'prompt'>) =>
      api.post<Rubric>('/config/rubrics', payload),
    onSuccess: () => client.invalidateQueries({ queryKey: ['config', 'rubrics'] }),
  });
}

export function useUpdateRubric() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ key, ...payload }: Pick<Rubric, 'key' | 'display_name' | 'prompt'>) =>
      api.put<Rubric>(`/config/rubrics/${key}`, payload),
    onSuccess: () => client.invalidateQueries({ queryKey: ['config', 'rubrics'] }),
  });
}

export function useDeleteRubric() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (key: string) => api.delete<void>(`/config/rubrics/${key}`),
    onSuccess: () => client.invalidateQueries({ queryKey: ['config', 'rubrics'] }),
  });
}
```

- [ ] **Step 3: Verify build**

`cd frontend && npm run build 2>&1 | tail -10`

Expected errors: anywhere `grade.judge_correctness` etc. are read. We fix those in Tasks 8 and 9. For now, get types.ts + hooks.ts compiling on their own — they may need a top-level `import type { Rubric }` in hooks.ts.

- [ ] **Step 4: Commit (build may have downstream errors from old field refs — that's fine for this commit)**

```bash
git add frontend/src/lib/types.ts frontend/src/lib/hooks.ts
git commit -m "Update Grade type to map shape; add Rubric type and CRUD hooks"
```

---

## Task 8: Frontend Compare + LLMJudgeCard adapt to map shape

**Files:**
- Modify: `frontend/src/components/grading-inspector/LLMJudgeCard.tsx`
- Modify: `frontend/src/pages/diagnostic/compare.tsx`
- Modify: `frontend/src/pages/runs/grading.test.tsx`

- [ ] **Step 1: LLMJudgeCard reads `grade.judge_scores` + `grade.judge_rationales`**

Replace the `dims` array computed from hardcoded fields with:

```tsx
const scores = grade.judge_scores ?? {};
const rationales = grade.judge_rationales ?? {};
const dims = Object.entries(scores).map(([key, value]) => ({ key, value }));
```

Render bars by iterating `dims`:

```tsx
{dims.map(d => <DimRow key={d.key} label={prettyDim(d.key)} value={d.value} />)}
```

Where `prettyDim(key)` is `key.replace(/_/g, ' ').replace(/^\w/, c => c.toUpperCase())`.

Per-dim rationale section iterates `rationales`:

```tsx
{Object.keys(rationales).length > 0 && (
  <div className="mt-3 space-y-2 border-t border-border pt-3">
    <div className="text-xs uppercase tracking-wider text-fg-muted">Per-dimension rationale</div>
    {Object.entries(rationales).filter(([_, t]) => t).map(([dim, text]) => (
      <div key={dim}>
        <div className="text-xs font-medium capitalize text-fg-muted">{prettyDim(dim)}</div>
        <p className="text-sm text-fg">{text}</p>
      </div>
    ))}
  </div>
)}
```

The `extractRationalesByDim` helper (which parsed `dim=<name>;<json>` from raw_responses) is no longer needed for rationales — they come straight from the grade. The raw-debug `<pre>` block still uses `raw_judge_responses` as-is.

- [ ] **Step 2: Update Compare page**

In `frontend/src/pages/diagnostic/compare.tsx`, the LLM-as-Judge section currently has 5 hardcoded `<BarRow>`s referencing `g.judge_correctness` etc. Replace with a dynamic loop:

```tsx
{/* Build the union of dim keys across all selected grades. */}
{Array.from(new Set(grades.flatMap(g => Object.keys(g?.judge_scores ?? {})))).map(dim => (
  <BarRow key={dim} label={prettyDim(dim)} headers={headers}
          grades={grades} pick={(g) => g?.judge_scores?.[dim] ?? 0} />
))}
```

Move `prettyDim` to a shared util (or duplicate inline; YAGNI rules say duplicate if only two sites need it).

- [ ] **Step 3: Update `grading.test.tsx` mock**

Replace the `raw_judge_responses` and dim score fields with the new map shape:

```tsx
judge_scores: {
  correctness: 8.0,
  maintainability: 7.0,
  completeness: 9.0,
  best_practices: 6.0,
  error_handling: 5.0,
},
judge_rationales: {
  correctness: "solid solution on correctness",
  maintainability: "clean names",
  completeness: "all requirements covered",
  best_practices: "sync lock in async code",
  error_handling: "happy path only",
},
```

The rationale assertion stays as `expect(screen.getByText(/solid solution on correctness/))`.

- [ ] **Step 4: Verify build + test**

```bash
cd frontend && npm run build 2>&1 | tail -8
cd frontend && npm test -- grading.test 2>&1 | tail -6
```

Both should pass.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/grading-inspector/LLMJudgeCard.tsx \
        frontend/src/pages/diagnostic/compare.tsx \
        frontend/src/pages/runs/grading.test.tsx
git commit -m "LLMJudgeCard + Compare iterate judge_scores map (N dimensions)"
```

---

## Task 9: Frontend `/rubrics` page + sidebar entry

**Files:**
- Create: `frontend/src/pages/rubrics/index.tsx`
- Create: `frontend/src/pages/rubrics/RubricEditor.tsx`
- Create: `frontend/src/pages/rubrics/AddRubricDialog.tsx`
- Modify: `frontend/src/routes.tsx`
- Modify: sidebar component (locate via grep)

- [ ] **Step 1: Locate sidebar**

`grep -rn "Settings\|Sidebar\|NavLink" frontend/src/components/system/ frontend/src/components/layout/ 2>/dev/null | head -10`

The sidebar likely lives in `frontend/src/App.tsx` or in a layout component. Find the file with the existing nav entries (Experiments, Settings, etc.).

- [ ] **Step 2: Add Rubrics nav entry**

Add an entry alongside Settings:

```tsx
<NavLink to="/rubrics" className={...}>Rubrics</NavLink>
```

(Match the existing nav-item pattern.)

- [ ] **Step 3: Create the page `frontend/src/pages/rubrics/index.tsx`**

```tsx
import { useState } from 'react';
import type { Rubric } from '../../lib/types';
import { useDeleteRubric, useRubrics } from '../../lib/hooks';
import { Button } from '../../components/ui/button';
import { Card, CardHeader } from '../../components/ui/card';
import { ErrorState, LoadingSkeleton } from '../../components/system';
import { AddRubricDialog } from './AddRubricDialog';
import { RubricEditor } from './RubricEditor';

export function RubricsPage() {
  const { data: rubrics, isLoading, isError, refetch } = useRubrics();
  const del = useDeleteRubric();
  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [showAdd, setShowAdd] = useState(false);

  if (isLoading) return <LoadingSkeleton variant="row" count={5} />;
  if (isError) return <ErrorState title="Could not load rubrics" onRetry={() => refetch()} />;
  const list = rubrics ?? [];
  const editing = editingKey ? list.find((r) => r.key === editingKey) ?? null : null;

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader
          title="Rubrics"
          description="Per-dimension system prompts the LLM judge uses. Changes take effect on the next experiment."
        />
        <div className="mb-3 flex justify-end">
          <Button onClick={() => setShowAdd(true)}>+ Add dimension</Button>
        </div>
        <ul className="space-y-1">
          {list.map((r) => (
            <li key={r.key}
                className="flex items-center justify-between rounded-lg border border-border bg-bg-elev-1 px-3 py-2 text-sm">
              <div className="flex-1">
                <div className="font-medium text-fg">{r.display_name}</div>
                <div className="text-xs text-fg-muted font-mono">{r.key}{r.is_builtin && ' (builtin)'}</div>
              </div>
              <div className="flex gap-1">
                <Button variant="ghost" onClick={() => setEditingKey(r.key)}>Edit</Button>
                {!r.is_builtin && (
                  <Button variant="ghost"
                          onClick={() => { if (confirm(`Delete ${r.display_name}?`)) del.mutate(r.key); }}
                          disabled={del.isPending}>Delete</Button>
                )}
              </div>
            </li>
          ))}
        </ul>
      </Card>

      {editing && <RubricEditor rubric={editing} onClose={() => setEditingKey(null)} />}
      {showAdd && <AddRubricDialog onClose={() => setShowAdd(false)} />}
    </div>
  );
}
```

- [ ] **Step 4: Create RubricEditor.tsx**

```tsx
import { useState } from 'react';
import type { Rubric } from '../../lib/types';
import { useUpdateRubric } from '../../lib/hooks';
import { Button } from '../../components/ui/button';
import { Card, CardHeader } from '../../components/ui/card';
import { Input } from '../../components/ui/input';

export function RubricEditor({ rubric, onClose }: { rubric: Rubric; onClose: () => void }) {
  const [displayName, setDisplayName] = useState(rubric.display_name);
  const [prompt, setPrompt] = useState(rubric.prompt);
  const update = useUpdateRubric();

  const onSave = () => {
    update.mutate(
      { key: rubric.key, display_name: displayName, prompt },
      { onSuccess: onClose },
    );
  };

  return (
    <Card>
      <CardHeader title={`Edit ${rubric.key}`} description={rubric.is_builtin ? 'Builtin rubric — edits persist; cannot delete.' : 'Custom rubric.'} />
      <div className="space-y-3">
        <div>
          <label className="mb-1 block text-xs text-fg-muted">Display name</label>
          <Input value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
        </div>
        <div>
          <label className="mb-1 block text-xs text-fg-muted">Prompt (system prompt for this dim's LLM call)</label>
          <textarea
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            rows={24}
            className="w-full rounded-md border border-border bg-bg-elev-1 p-2 font-mono text-xs text-fg"
          />
        </div>
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>Cancel</Button>
          <Button onClick={onSave} disabled={update.isPending}>{update.isPending ? 'Saving…' : 'Save'}</Button>
        </div>
      </div>
    </Card>
  );
}
```

- [ ] **Step 5: Create AddRubricDialog.tsx**

```tsx
import { useState } from 'react';
import { useCreateRubric } from '../../lib/hooks';
import { Button } from '../../components/ui/button';
import { Card, CardHeader } from '../../components/ui/card';
import { Input } from '../../components/ui/input';

const STARTER_PROMPT = `You are a strict senior code reviewer scoring ONE
dimension of an AI coding agent's output: **REPLACE_ME**.

Describe what this dimension measures and what to look for. Reference
specific evidence from the output files, test results, or transcript.

## Output format
Return a JSON object with:
- score: float in [0.0, 10.0]
- rationale: string up to 600 chars

## Calibration
Most outputs score 4-7. Reserve 8-10 for production-ready work.
Reserve 0-2 for output that does not address this dimension.`;

export function AddRubricDialog({ onClose }: { onClose: () => void }) {
  const [key, setKey] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [prompt, setPrompt] = useState(STARTER_PROMPT);
  const create = useCreateRubric();
  const [error, setError] = useState<string | null>(null);

  const onCreate = () => {
    if (!/^[a-z][a-z0-9_]{1,40}$/.test(key)) {
      setError('Key must be lowercase snake_case, 2-41 chars.'); return;
    }
    if (!displayName.trim()) { setError('Display name required.'); return; }
    if (prompt.length < 50) { setError('Prompt must be at least 50 chars.'); return; }
    create.mutate(
      { key, display_name: displayName, prompt },
      {
        onSuccess: onClose,
        onError: (err) => setError(String(err)),
      },
    );
  };

  return (
    <Card>
      <CardHeader title="Add dimension" description="Define a new judge dimension. The LLM will score every grade against it." />
      <div className="space-y-3">
        <div>
          <label className="mb-1 block text-xs text-fg-muted">Key (lowercase snake_case)</label>
          <Input value={key} onChange={(e) => setKey(e.target.value)} placeholder="e.g., security" />
        </div>
        <div>
          <label className="mb-1 block text-xs text-fg-muted">Display name</label>
          <Input value={displayName} onChange={(e) => setDisplayName(e.target.value)} placeholder="Security" />
        </div>
        <div>
          <label className="mb-1 block text-xs text-fg-muted">Prompt</label>
          <textarea
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            rows={20}
            className="w-full rounded-md border border-border bg-bg-elev-1 p-2 font-mono text-xs text-fg"
          />
        </div>
        {error && <div className="text-xs text-danger">{error}</div>}
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>Cancel</Button>
          <Button onClick={onCreate} disabled={create.isPending}>{create.isPending ? 'Creating…' : 'Create'}</Button>
        </div>
      </div>
    </Card>
  );
}
```

- [ ] **Step 6: Register the route**

In `frontend/src/routes.tsx`:

```tsx
import { RubricsPage } from './pages/rubrics';
// ...
<Route path="/rubrics" element={<RubricsPage />} />
```

- [ ] **Step 7: Verify build**

`cd frontend && npm run build 2>&1 | tail -8` → clean.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/pages/rubrics/ frontend/src/routes.tsx <sidebar-file>
git commit -m "Add /rubrics page with editor + add-dimension dialog; sidebar entry"
```

---

## Task 10: Smoke test for /rubrics + docs

**Files:**
- Create: `frontend/src/pages/rubrics/rubrics.test.tsx`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Write smoke test**

```tsx
import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';

import { RubricsPage } from './index';

vi.mock('../../lib/hooks', () => ({
  useRubrics: () => ({
    data: [
      { key: 'correctness', display_name: 'Correctness', prompt: '...', sort_order: 1, is_builtin: true },
      { key: 'security', display_name: 'Security', prompt: '...', sort_order: 99, is_builtin: false },
    ],
    isLoading: false, isError: false, refetch: vi.fn(),
  }),
  useDeleteRubric: () => ({ mutate: vi.fn(), isPending: false }),
}));

describe('RubricsPage', () => {
  it('renders builtin + custom rubrics with appropriate actions', () => {
    const qc = new QueryClient();
    render(
      <QueryClientProvider client={qc}>
        <MemoryRouter initialEntries={['/rubrics']}>
          <Routes>
            <Route path="/rubrics" element={<RubricsPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );
    expect(screen.getByText('Correctness')).toBeInTheDocument();
    expect(screen.getByText('Security')).toBeInTheDocument();
    expect(screen.getAllByText('Edit')).toHaveLength(2);
    expect(screen.getAllByText('Delete')).toHaveLength(1); // only Security has Delete
  });
});
```

`cd frontend && npm test -- rubrics.test 2>&1 | tail -6` → 1 PASS.

- [ ] **Step 2: Update CLAUDE.md**

Add to the "## Important Constraints" section:

> - Judge dimensions (rubrics) are SQLite-stored in the `rubrics` table and editable from the `/rubrics` page. The 5 builtin dimensions can be edited but not deleted; user-added dimensions can be deleted. The grader uses whatever is in the table at run time, falling back to hardcoded defaults in `grader/llm_judge/prompts.py` only when the engine sends no rubrics (headless path).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/rubrics/rubrics.test.tsx CLAUDE.md
git commit -m "Add Rubrics page smoke test + CLAUDE.md note on editable dimensions"
```

---

## Self-review (run before declaring plan done)

**Spec coverage:**
- §4.1 (migration + JSON cols) → Task 1
- §4.2 (RubricsRepo) → Task 1
- §4.3 (proto) → Task 2
- §4.4 (HTTP) → Task 3
- §4.5 (engine grade + buildJudgeConfig) → Task 4
- §4.6 (composite) → Task 5
- §4.7 (grader.py variable-N) → Task 6
- §4.8 (CRITICAL FACTS render_user_prompt) → Task 5
- §4.9 (rubrics page) → Task 9
- §4.10 (LLMJudgeCard + Compare map iter) → Task 8
- §4.11 (test mocks update) → Task 8 + 10
- §4.12 (tests) → spread across 1, 3, 4, 5, 6, 10

All covered.

**Placeholder check:** No TBD. Migration step calls out paste-prompts-from-prompts.py explicitly. Sidebar file is "find via grep" — that's instruction, not placeholder.

**Type consistency:**
- `Rubric` Go struct (storage) ↔ `Rubric` TS type (frontend): same field names. ✓
- `Grade.JudgeScores` Go map ↔ `Grade.judge_scores` TS Record: serialized as JSON object. ✓
- `JudgeGradeResult.scores` proto map ↔ `grade["scores"]` Python dict ↔ `Grade.JudgeScores` Go map: all the same shape. ✓
- `DimensionRubric` proto ↔ `(key, prompt)` Python tuple list: matches via `_extract_rubrics`. ✓

Plan ready.
