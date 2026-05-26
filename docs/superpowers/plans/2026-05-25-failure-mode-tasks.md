# Failure-Mode-Calibrated Tasks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Add 5 brownfield tasks under `tasks/`, each calibrated to a single failure mode from the 12-code taxonomy, designed so the `bare` harness lands ~40-60% pass rate. Each task is a self-contained directory matching the existing `brownfield-fix-async-race` layout.

**Architecture:** Each task is a directory with `task.yaml`, `setup.sh`, `eval.sh`, `workspace/` (agent-visible source), `tests/` (pytest + scope-shell), and optional `reference/` (canonical solution, hidden from agent). The `primary_failure_mode` metadata is stored in the existing `metadata` map field on Task — no engine code change needed.

**Tech Stack:** Python 3.10+, FastAPI / SQLAlchemy / Alembic / Pydantic v2 (one stack per task), pytest, pytest-asyncio, httpx for async test clients.

**Spec:** [`docs/superpowers/specs/2026-05-25-failure-mode-tasks-design.md`](../specs/2026-05-25-failure-mode-tasks-design.md)

---

## File Structure

Each new task directory mirrors `tasks/brownfield-fix-async-race/`:

```
tasks/<task-id>/
  task.yaml          # id, prompt, test_cases, metadata.primary_failure_mode
  setup.sh           # pip install + git init + tag baseline
  eval.sh            # runs pytest + shell scope test
  pyproject.toml     # when packaging matters
  workspace/         # what the agent sees at /workspace in sandbox
    requirements.txt
    README.md
    app/...          # source files
  tests/             # eval-time tests
    __init__.py
    conftest.py
    test_<scenario>.py
    test_scope.sh
  reference/         # canonical solution (hidden from agent)
    <fixed-file>.py
```

**5 task directories to create:**
- `tasks/brownfield-misread-hidden-contract/` (MISREAD)
- `tasks/brownfield-scope-drift-tempting-cleanup/` (SCOPE_DRIFT)
- `tasks/brownfield-wrong-abs-async-throttle/` (WRONG_ABS)
- `tasks/brownfield-stop-early-multi-step-migration/` (STOP_EARLY)
- `tasks/brownfield-hal-api-pydantic-version/` (HAL_API)

**No engine code changes.** `primary_failure_mode` rides on the existing `metadata` map.

---

## Conventions (apply to every task)

### setup.sh template (every task uses this shape)

```bash
#!/usr/bin/env bash
# Sandbox-side setup for <task-id>.
# Same shape as brownfield-fix-async-race/setup.sh. Cwd is /workspace
# (flattened from tasks/<id>/workspace/) inside the sandbox container.
set -euo pipefail
pip install --no-cache-dir -r requirements.txt
if [[ ! -d .git ]]; then
    git init -q
    git config user.email "agentdx@frameval.local"
    git config user.name "AgentDx"
    git add -A
    git commit -q -m "AgentDx baseline (<task-id>)"
    git tag baseline
fi
```

Replace `<task-id>` per task. Some tasks (the alembic one) need extra setup steps; those are called out in their task section.

### eval.sh template

```bash
#!/usr/bin/env bash
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/workspace"
pytest -q --tb=short ../tests/test_<a>.py ../tests/test_<b>.py
bash "$SCRIPT_DIR/tests/test_scope.sh"
```

Each task substitutes its actual test files.

### task.yaml header pattern

```yaml
id: <task-id>
name: <human-readable name>
description: |
  <2-3 sentence summary of what the task tests>
category: brownfield
template_kind: builtin
workspace_mode: local
complexity_score: 6.5
codebase_type: python
metadata:
  primary_failure_mode: <CODE>
task_prompt: |
  <agent-facing prompt>
technical_details: |
  <notes for the human reading the task; not shown to agent>
test_cases:
  - name: <human-readable test description>
    test_command: pytest -q tests/<file>.py
    expected_result: exit 0
    ordering: 0
  ...
```

The `metadata.primary_failure_mode` value must be one of the 13 enum values from `grader/failure_classifier/taxonomy.py:FailureCode`: NONE, HAL_API, HAL_FILE, DEP_MISS, STOP_EARLY, STOP_GIVEUP, LOOP_INF, WRONG_ABS, MISREAD, ENV_ERR, SCOPE_DRIFT, TIMEOUT, SILENT_SKIP.

### Scope-shell test template (`tests/test_scope.sh`)

```bash
#!/usr/bin/env bash
# Scope-discipline check: agent may only modify files in the allow-list.
set -e
cd "$(dirname "$0")/../workspace"
CHANGED=$(git diff --name-only baseline; git ls-files --others --exclude-standard)
EXTRA=$(echo "$CHANGED" | sort -u | grep -v -E '^(<allowed-file-1>|<allowed-file-2>)$' || true)
if [[ -n "$EXTRA" ]]; then
    echo "Scope drift detected. Disallowed changes:" >&2
    echo "$EXTRA" >&2
    exit 1
fi
```

Per task, the allow-list regex changes. Untracked files count as drift.

---

## Task 1: brownfield-misread-hidden-contract (MISREAD)

**Files to create under** `tasks/brownfield-misread-hidden-contract/`:
- `task.yaml`
- `setup.sh`
- `eval.sh`
- `workspace/requirements.txt`
- `workspace/README.md`
- `workspace/app/__init__.py`
- `workspace/app/main.py`
- `workspace/app/users.py`
- `workspace/openapi.yaml`
- `tests/__init__.py`
- `tests/conftest.py`
- `tests/test_users_basic.py`
- `tests/test_spec_compliance.py`
- `tests/test_scope.sh`
- `reference/users.py` (canonical fix; hidden from agent)
- `reference/openapi.yaml` (canonical spec update)

- [ ] **Step 1: Create task.yaml**

```yaml
id: brownfield-misread-hidden-contract
name: Add created_at to user response (and the OpenAPI spec)
description: |
  Brownfield task. FastAPI endpoint GET /users/{id} returns user data;
  the request adds a `created_at` field. An openapi.yaml in the
  workspace pins the response schema and a spec-compliance test
  validates real responses against it. The task prompt mentions the
  endpoint but NOT the spec — agents that miss reading openapi.yaml
  fail the spec test. Stresses MISREAD (implicit contract in a
  sibling file).
category: brownfield
template_kind: builtin
workspace_mode: local
complexity_score: 5.5
codebase_type: python
metadata:
  primary_failure_mode: MISREAD
task_prompt: |
  Add a `created_at` field to the response of GET /users/{id}. The
  field should be an ISO 8601 timestamp string. For the seeded users
  (id 1 and id 2), return "2024-01-01T00:00:00Z".

  Constraints:
    - Function signature of get_user() may not change.
    - Existing response fields (id, name, email) remain.
technical_details: |
  - workspace/openapi.yaml defines the canonical response schema.
  - tests/test_spec_compliance.py loads openapi.yaml and validates
    real responses against it via openapi-spec-validator + jsonschema.
  - The agent must update BOTH workspace/app/users.py and
    workspace/openapi.yaml.
  - test_scope.sh allows only those two files in the diff.
test_cases:
  - name: GET /users/{id} returns created_at
    test_command: pytest -q tests/test_users_basic.py
    expected_result: exit 0
    ordering: 0
  - name: Response matches openapi.yaml schema
    test_command: pytest -q tests/test_spec_compliance.py
    expected_result: exit 0
    ordering: 1
  - name: Only users.py and openapi.yaml are modified
    test_command: bash tests/test_scope.sh
    expected_result: exit 0
    ordering: 2
```

- [ ] **Step 2: Create setup.sh**

```bash
#!/usr/bin/env bash
set -euo pipefail
pip install --no-cache-dir -r requirements.txt
if [[ ! -d .git ]]; then
    git init -q
    git config user.email "agentdx@frameval.local"
    git config user.name "AgentDx"
    git add -A
    git commit -q -m "AgentDx baseline (brownfield-misread-hidden-contract)"
    git tag baseline
fi
```

Mark executable: `chmod +x setup.sh`.

- [ ] **Step 3: Create eval.sh**

```bash
#!/usr/bin/env bash
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/workspace"
pytest -q --tb=short ../tests/test_users_basic.py ../tests/test_spec_compliance.py
bash "$SCRIPT_DIR/tests/test_scope.sh"
```

`chmod +x eval.sh`.

- [ ] **Step 4: Create workspace files**

`workspace/requirements.txt`:
```
fastapi>=0.110
httpx>=0.27
pytest>=8.0
pytest-asyncio>=0.23
pyyaml>=6.0
jsonschema>=4.0
openapi-spec-validator>=0.7
```

`workspace/README.md`:
```markdown
# user-service

Tiny FastAPI app exposing `GET /users/{id}`.

The response schema lives in `openapi.yaml`. Spec/impl alignment is
enforced by `tests/test_spec_compliance.py` so any field added to the
response must also be reflected in the spec.

Seeded users: id 1 (Alice), id 2 (Bob).
```

`workspace/app/__init__.py`: empty file.

`workspace/app/main.py`:
```python
"""FastAPI entry point — mounts the users router."""
from __future__ import annotations

from fastapi import FastAPI

from app import users

app = FastAPI(title="user-service")
app.include_router(users.router)
```

`workspace/app/users.py`:
```python
"""User read endpoint.

The agent must extend the response shape to include created_at while
keeping the function signature and the existing fields untouched.
"""
from __future__ import annotations

from fastapi import APIRouter, HTTPException

router = APIRouter()

_SEED_USERS = {
    1: {"id": 1, "name": "Alice", "email": "alice@example.com"},
    2: {"id": 2, "name": "Bob", "email": "bob@example.com"},
}


@router.get("/users/{user_id}")
async def get_user(user_id: int) -> dict:
    user = _SEED_USERS.get(user_id)
    if user is None:
        raise HTTPException(status_code=404, detail="not found")
    return user
```

`workspace/openapi.yaml`:
```yaml
openapi: 3.0.3
info:
  title: user-service
  version: 0.1.0
paths:
  /users/{user_id}:
    get:
      operationId: get_user
      parameters:
        - in: path
          name: user_id
          required: true
          schema: {type: integer}
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                required: [id, name, email]
                properties:
                  id: {type: integer}
                  name: {type: string}
                  email: {type: string}
```

- [ ] **Step 5: Create tests**

`tests/__init__.py`: empty file.

`tests/conftest.py`:
```python
"""Pytest fixtures shared by the user-service tests."""
from __future__ import annotations

import sys
from pathlib import Path

import pytest
from httpx import ASGITransport, AsyncClient

# Make the workspace's app/ importable without installing.
sys.path.insert(0, str(Path(__file__).parent.parent / "workspace"))

from app.main import app  # noqa: E402  (path manipulated above)


@pytest.fixture
def client() -> AsyncClient:
    return AsyncClient(transport=ASGITransport(app=app), base_url="http://test")
```

`tests/test_users_basic.py`:
```python
"""Functional test: the new created_at field exists and matches the
seeded timestamp."""
from __future__ import annotations

import pytest


@pytest.mark.asyncio
async def test_user_response_includes_created_at(client):
    response = await client.get("/users/1")
    assert response.status_code == 200
    body = response.json()
    # Existing fields stay.
    assert body["id"] == 1
    assert body["name"] == "Alice"
    assert body["email"] == "alice@example.com"
    # New field present and matches the seeded fixed timestamp.
    assert body["created_at"] == "2024-01-01T00:00:00Z"
```

`tests/test_spec_compliance.py`:
```python
"""Contract test: the live response must validate against openapi.yaml.

This is the trap test for MISREAD. An agent that updates only users.py
and not openapi.yaml will see this test fail because jsonschema rejects
the unknown `created_at` field (the spec's `additionalProperties` is
implicit-false by default for explicit type=object schemas).
"""
from __future__ import annotations

from pathlib import Path

import pytest
import yaml
from jsonschema import validate, Draft202012Validator


SPEC_PATH = Path(__file__).parent.parent / "workspace" / "openapi.yaml"


def _user_schema() -> dict:
    spec = yaml.safe_load(SPEC_PATH.read_text())
    schema = spec["paths"]["/users/{user_id}"]["get"]["responses"]["200"]["content"]["application/json"]["schema"]
    # Default to additionalProperties=False so any drift between handler
    # and spec is caught. (OpenAPI's default is True; we tighten here.)
    schema.setdefault("additionalProperties", False)
    return schema


@pytest.mark.asyncio
async def test_response_matches_openapi_spec(client):
    response = await client.get("/users/1")
    assert response.status_code == 200
    body = response.json()
    schema = _user_schema()
    Draft202012Validator.check_schema(schema)
    validate(instance=body, schema=schema)
```

`tests/test_scope.sh`:
```bash
#!/usr/bin/env bash
set -e
cd "$(dirname "$0")/../workspace"
CHANGED=$( { git diff --name-only baseline; git ls-files --others --exclude-standard; } | sort -u )
EXTRA=$(echo "$CHANGED" | grep -v -E '^(app/users\.py|openapi\.yaml)$' || true)
if [[ -n "$EXTRA" ]]; then
    echo "Scope drift detected. Disallowed changes:" >&2
    echo "$EXTRA" >&2
    exit 1
fi
```

`chmod +x tests/test_scope.sh`.

- [ ] **Step 6: Create reference solution** (hidden from agent; in `reference/`)

`reference/users.py` — the canonical post-fix version:
```python
"""Reference solution (for the human reviewer)."""
from __future__ import annotations

from fastapi import APIRouter, HTTPException

router = APIRouter()

_SEED_USERS = {
    1: {"id": 1, "name": "Alice", "email": "alice@example.com", "created_at": "2024-01-01T00:00:00Z"},
    2: {"id": 2, "name": "Bob",   "email": "bob@example.com",   "created_at": "2024-01-01T00:00:00Z"},
}


@router.get("/users/{user_id}")
async def get_user(user_id: int) -> dict:
    user = _SEED_USERS.get(user_id)
    if user is None:
        raise HTTPException(status_code=404, detail="not found")
    return user
```

`reference/openapi.yaml` — canonical spec with created_at added under properties + required.

- [ ] **Step 7: Smoke-run the reference solution locally**

This validates the test suite actually catches the trap. From the repo root:

```bash
cd /Users/mustafaselmangunes/Desktop/Frameval/tasks/brownfield-misread-hidden-contract
python -m venv .venv && source .venv/bin/activate
cd workspace
pip install -q -r requirements.txt
git init -q && git config user.email t@t && git config user.name t && git add -A && git commit -q -m baseline && git tag baseline
# Run tests against the UNFIXED workspace — basic + scope pass, spec FAILS.
pytest -q ../tests/test_users_basic.py  # expect FAIL (no created_at)
pytest -q ../tests/test_spec_compliance.py  # expect FAIL (no created_at)
# Apply reference fix.
cp ../reference/users.py app/users.py
cp ../reference/openapi.yaml openapi.yaml
# Now all three pass.
pytest -q ../tests/test_users_basic.py  # expect PASS
pytest -q ../tests/test_spec_compliance.py  # expect PASS
bash ../tests/test_scope.sh  # expect PASS
deactivate && cd ../.. && rm -rf .venv workspace/.git
```

Expected: with the reference applied, all three tests pass. Without the reference (raw workspace), the basic + spec tests fail.

- [ ] **Step 8: Commit**

```bash
cd /Users/mustafaselmangunes/Desktop/Frameval
git add tasks/brownfield-misread-hidden-contract/
git commit -m "Add brownfield-misread-hidden-contract task (MISREAD)"
```

---

## Task 2: brownfield-scope-drift-tempting-cleanup (SCOPE_DRIFT)

**Files to create under** `tasks/brownfield-scope-drift-tempting-cleanup/`:
- `task.yaml`
- `setup.sh`
- `eval.sh`
- `workspace/requirements.txt`
- `workspace/README.md`
- `workspace/app/__init__.py`
- `workspace/app/pricing.py`
- `workspace/app/models.py`
- `workspace/app/storage.py`
- `tests/__init__.py`
- `tests/test_charge_correctness.py`
- `tests/test_existing_behavior.py`
- `tests/test_scope.sh`
- `reference/pricing.py`

- [ ] **Step 1: Create task.yaml**

```yaml
id: brownfield-scope-drift-tempting-cleanup
name: Fix negative-total bug in pricing.compute_total
description: |
  Brownfield task. A 200-line Python module has a one-line bug
  (negative total when discount exceeds subtotal) surrounded by
  tempting cleanup opportunities — 3 functions marked DEPRECATED, a
  copy-paste-heavy helper, a "TODO: refactor this mess" comment. The
  task asks the agent to fix the bug only. Stresses SCOPE_DRIFT:
  agents that try to clean while they're there break unrelated tests
  and trip the scope-discipline check.
category: brownfield
template_kind: builtin
workspace_mode: local
complexity_score: 5.0
codebase_type: python
metadata:
  primary_failure_mode: SCOPE_DRIFT
task_prompt: |
  `app/pricing.py::compute_total` returns a negative total when the
  discount exceeds the subtotal. It should clamp to 0 instead.

  Fix the bug.

  Constraints:
    - Modify only `app/pricing.py`.
    - Do not refactor, rename, or remove unrelated code in the same
      file, including the DEPRECATED functions and the _apply_loyalty_tier
      helper.
    - All existing tests must continue to pass.
technical_details: |
  - workspace/app/pricing.py has 4 DEPRECATED functions, copy-paste
    branches in _apply_loyalty_tier, and a "TODO: refactor" comment
    that lures agents.
  - test_existing_behavior.py exercises 6 pricing scenarios that pass
    on the baseline; if the agent refactors away helpers, behavior
    drifts and these fail.
  - test_scope.sh requires the diff to list ONLY pricing.py AND the
    count of "# DEPRECATED" markers in the post-fix file equals the
    baseline count (4). This catches "agent silently removed the
    deprecated funcs while fixing the bug."
test_cases:
  - name: Negative total clamps to 0
    test_command: pytest -q tests/test_charge_correctness.py
    expected_result: exit 0
    ordering: 0
  - name: Six existing pricing scenarios still pass
    test_command: pytest -q tests/test_existing_behavior.py
    expected_result: exit 0
    ordering: 1
  - name: Scope discipline (single file, DEPRECATED count unchanged)
    test_command: bash tests/test_scope.sh
    expected_result: exit 0
    ordering: 2
```

- [ ] **Step 2: setup.sh and eval.sh**

Identical to Task 1's pattern, substituting the task id. setup.sh's git commit message: `"AgentDx baseline (brownfield-scope-drift-tempting-cleanup)"`.

eval.sh:
```bash
#!/usr/bin/env bash
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/workspace"
pytest -q --tb=short ../tests/test_charge_correctness.py ../tests/test_existing_behavior.py
bash "$SCRIPT_DIR/tests/test_scope.sh"
```

Both executable.

- [ ] **Step 3: workspace files**

`workspace/requirements.txt`:
```
pytest>=8.0
```

`workspace/README.md`:
```markdown
# pricing-engine

Toy pricing/discount module used by the checkout flow.

`compute_total` has a known bug: when a discount exceeds the subtotal
it returns a negative total. The fix should clamp at 0.

The module also contains DEPRECATED helpers and rough code marked
TODO. Do NOT touch them in this fix — separate cleanup PRs handle
deprecations.
```

`workspace/app/__init__.py`: empty.

`workspace/app/models.py`:
```python
"""Domain types used by pricing.py."""
from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class LineItem:
    sku: str
    unit_price: float
    quantity: int


@dataclass(frozen=True)
class Customer:
    id: int
    loyalty_tier: str  # "bronze" | "silver" | "gold" | "platinum"
```

`workspace/app/storage.py`:
```python
"""In-memory line-item store for tests."""
from __future__ import annotations

from app.models import LineItem

_ITEMS: dict[str, list[LineItem]] = {}


def add(order_id: str, item: LineItem) -> None:
    _ITEMS.setdefault(order_id, []).append(item)


def items_for(order_id: str) -> list[LineItem]:
    return list(_ITEMS.get(order_id, []))


def reset() -> None:
    _ITEMS.clear()
```

`workspace/app/pricing.py`:

The pricing.py file MUST contain exactly 4 `# DEPRECATED` markers in deprecated function docstrings/headers, a 40-line `_apply_loyalty_tier` helper with visible copy-paste branches, a `# TODO: refactor this mess` comment near the bug, and the buggy `compute_total` function. Approximate skeleton (subagent fills in the verbose body to reach ~200 lines):

```python
"""Pricing rules for the checkout flow.

# TODO: refactor this mess  ← intentional bait
"""
from __future__ import annotations

from app.models import Customer, LineItem


# DEPRECATED: legacy promo lookup; remove once migrate-promos lands.
def lookup_promo_legacy(sku: str) -> float | None:
    table = {"sku-1": 0.10, "sku-2": 0.15}
    return table.get(sku)


# DEPRECATED: replaced by lookup_promo_v2; kept for backward compat.
def lookup_promo_v1(sku: str) -> float:
    val = lookup_promo_legacy(sku)
    return val if val is not None else 0.0


# DEPRECATED: pricing.discount was inlined into compute_total in 2023.
def discount(subtotal: float, pct: float) -> float:
    return subtotal * pct


# DEPRECATED: shipping was extracted to shipping.py last quarter.
def shipping_for(subtotal: float) -> float:
    if subtotal < 50:
        return 9.99
    return 0.0


def _apply_loyalty_tier(subtotal: float, customer: Customer) -> float:
    """Apply tier-based discount.

    Note the copy-paste in each branch — refactor target, but out of
    scope for the current bug fix.
    """
    if customer.loyalty_tier == "bronze":
        # Bronze gets nothing.
        adjustment = 0.0
        floor = 0.0
        cap = 0.0
        adjusted = max(adjustment, floor)
        adjusted = min(adjusted, cap)
        return subtotal - adjusted
    if customer.loyalty_tier == "silver":
        # Silver: 2% off, floor 0, cap 5.
        adjustment = subtotal * 0.02
        floor = 0.0
        cap = 5.0
        adjusted = max(adjustment, floor)
        adjusted = min(adjusted, cap)
        return subtotal - adjusted
    if customer.loyalty_tier == "gold":
        # Gold: 5% off, floor 0, cap 25.
        adjustment = subtotal * 0.05
        floor = 0.0
        cap = 25.0
        adjusted = max(adjustment, floor)
        adjusted = min(adjusted, cap)
        return subtotal - adjusted
    if customer.loyalty_tier == "platinum":
        # Platinum: 10% off, floor 0, cap 100.
        adjustment = subtotal * 0.10
        floor = 0.0
        cap = 100.0
        adjusted = max(adjustment, floor)
        adjusted = min(adjusted, cap)
        return subtotal - adjusted
    return subtotal


def compute_total(
    items: list[LineItem],
    customer: Customer,
    coupon_amount: float = 0.0,
) -> float:
    """Compute order total.

    BUG: when coupon_amount > subtotal, returns a negative number.
    Fix: clamp at 0.
    """
    subtotal = sum(item.unit_price * item.quantity for item in items)
    after_loyalty = _apply_loyalty_tier(subtotal, customer)
    # TODO: refactor this mess
    total = after_loyalty - coupon_amount
    return total
```

- [ ] **Step 4: tests**

`tests/__init__.py`: empty.

`tests/test_charge_correctness.py`:
```python
"""Functional test: the bug is fixed (negative total → 0)."""
from __future__ import annotations

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent / "workspace"))

from app.models import Customer, LineItem
from app.pricing import compute_total


def test_total_clamps_at_zero_when_coupon_exceeds_subtotal():
    items = [LineItem(sku="s", unit_price=10.0, quantity=1)]
    customer = Customer(id=1, loyalty_tier="bronze")
    total = compute_total(items, customer, coupon_amount=50.0)
    assert total == 0.0, f"expected clamped 0, got {total}"
```

`tests/test_existing_behavior.py`:
```python
"""Regression: existing pricing scenarios still pass."""
from __future__ import annotations

import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).parent.parent / "workspace"))

from app.models import Customer, LineItem
from app.pricing import compute_total


@pytest.mark.parametrize(
    "tier, items, coupon, expected",
    [
        ("bronze",   [LineItem("a", 10.0, 1)], 0.0, 10.0),
        ("silver",   [LineItem("a", 10.0, 1)], 0.0, 9.8),    # 2% off
        ("gold",     [LineItem("a", 10.0, 1)], 0.0, 9.5),    # 5% off
        ("platinum", [LineItem("a", 10.0, 1)], 0.0, 9.0),    # 10% off
        ("silver",   [LineItem("a", 1000.0, 1)], 0.0, 995.0),  # cap=5 binds
        ("bronze",   [LineItem("a", 100.0, 2)], 10.0, 190.0),  # subtotal=200 - coupon
    ],
)
def test_existing_pricing_unchanged(tier, items, coupon, expected):
    total = compute_total(items, Customer(id=1, loyalty_tier=tier), coupon_amount=coupon)
    assert total == pytest.approx(expected, abs=0.01)
```

`tests/test_scope.sh`:
```bash
#!/usr/bin/env bash
set -e
cd "$(dirname "$0")/../workspace"
CHANGED=$( { git diff --name-only baseline; git ls-files --others --exclude-standard; } | sort -u )
EXTRA=$(echo "$CHANGED" | grep -v -E '^app/pricing\.py$' || true)
if [[ -n "$EXTRA" ]]; then
    echo "Scope drift: changes outside pricing.py:" >&2
    echo "$EXTRA" >&2
    exit 1
fi
# DEPRECATED count must remain at the baseline (4).
COUNT=$(grep -c '# DEPRECATED' app/pricing.py || true)
if [[ "$COUNT" -ne 4 ]]; then
    echo "Scope drift: DEPRECATED markers changed from 4 to $COUNT" >&2
    exit 1
fi
```

`chmod +x tests/test_scope.sh`.

- [ ] **Step 5: Reference solution**

`reference/pricing.py`: same as workspace/app/pricing.py but with the one-line fix on the last line of compute_total:
```python
total = after_loyalty - coupon_amount
return max(total, 0.0)
```

- [ ] **Step 6: Smoke verify and commit**

Same smoke pattern as Task 1: with reference applied, all 3 tests pass; without, the correctness test fails.

```bash
git add tasks/brownfield-scope-drift-tempting-cleanup/
git commit -m "Add brownfield-scope-drift-tempting-cleanup task (SCOPE_DRIFT)"
```

---

## Task 3: brownfield-wrong-abs-async-throttle (WRONG_ABS)

**Files to create under** `tasks/brownfield-wrong-abs-async-throttle/`:
- `task.yaml`
- `setup.sh`, `eval.sh`
- `workspace/requirements.txt`, `workspace/README.md`
- `workspace/app/__init__.py`, `workspace/app/main.py`, `workspace/app/search.py`
- `tests/__init__.py`, `tests/conftest.py`
- `tests/test_search_correctness.py`, `tests/test_throttle_under_load.py`, `tests/test_scope.sh`
- `reference/search.py`

- [ ] **Step 1: task.yaml**

```yaml
id: brownfield-wrong-abs-async-throttle
name: Rate-limit an async FastAPI endpoint without blocking the loop
description: |
  Brownfield task. Async FastAPI endpoint needs rate limiting to 10
  req/s. Naive bare agents reach for time.sleep, which blocks the
  event loop and collapses throughput. Correct fix uses
  asyncio.sleep, asyncio.Lock + a counter, or aiolimiter. Stresses
  WRONG_ABS: picking sync sleep in an async context.
category: brownfield
template_kind: builtin
workspace_mode: local
complexity_score: 6.0
codebase_type: python
metadata:
  primary_failure_mode: WRONG_ABS
task_prompt: |
  Add rate limiting to `app/search.py::search` so it processes at most
  10 requests per second. Excess requests should wait, not be rejected.

  Constraints:
    - The endpoint must remain async-friendly. The function signature
      stays `async def search(...) -> dict`.
    - Single requests should complete in <100ms (no artificial delay
      for low load).
    - Tests fire 30 concurrent requests at a steady 10 req/s arrival
      rate. Throughput must not collapse below 8 req/s under that
      sustained load.
technical_details: |
  - Naive time.sleep in async code blocks the loop, queues stack up,
    test_throttle_under_load.py times out or asserts throughput floor.
  - Reference solution uses asyncio.Lock + monotonic timer for a
    simple token-bucket; aiolimiter is also acceptable (added to
    requirements.txt).
  - Tests use httpx.AsyncClient + ASGITransport (in-process, no
    uvicorn).
test_cases:
  - name: Single request still returns expected shape
    test_command: pytest -q tests/test_search_correctness.py
    expected_result: exit 0
    ordering: 0
  - name: Throughput holds under concurrent load
    test_command: pytest -q tests/test_throttle_under_load.py
    expected_result: exit 0
    ordering: 1
  - name: Only app/search.py modified
    test_command: bash tests/test_scope.sh
    expected_result: exit 0
    ordering: 2
```

- [ ] **Step 2: setup.sh, eval.sh** (same pattern; substitute task id)

`eval.sh`:
```bash
#!/usr/bin/env bash
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/workspace"
pytest -q --tb=short ../tests/test_search_correctness.py ../tests/test_throttle_under_load.py
bash "$SCRIPT_DIR/tests/test_scope.sh"
```

- [ ] **Step 3: workspace files**

`workspace/requirements.txt`:
```
fastapi>=0.110
httpx>=0.27
pytest>=8.0
pytest-asyncio>=0.23
aiolimiter>=1.1
```

`workspace/README.md`:
```markdown
# search-service

Async FastAPI endpoint `GET /search?q=...`. Currently unrestricted.

We need to throttle to 10 req/s under concurrent load. The
implementation must remain async-friendly — blocking the event loop
(`time.sleep`, sync HTTP calls) will collapse throughput and fail
`tests/test_throttle_under_load.py`.
```

`workspace/app/__init__.py`: empty.

`workspace/app/main.py`:
```python
from __future__ import annotations

from fastapi import FastAPI

from app.search import search

app = FastAPI(title="search-service")


@app.get("/search")
async def search_endpoint(q: str = "") -> dict:
    return await search(q)
```

`workspace/app/search.py`:
```python
"""Search handler.

Currently does no rate limiting — under sustained load the upstream
service it would talk to will throttle US instead, which is worse.
Agent task: add a 10 req/s cap that's async-friendly.
"""
from __future__ import annotations


async def search(q: str) -> dict:
    # Simulated lookup; in reality this would hit a vector DB.
    return {"query": q, "hits": [f"result-{i}-for-{q}" for i in range(3)]}
```

- [ ] **Step 4: tests**

`tests/__init__.py`: empty.

`tests/conftest.py`:
```python
from __future__ import annotations

import sys
from pathlib import Path

import pytest
from httpx import ASGITransport, AsyncClient

sys.path.insert(0, str(Path(__file__).parent.parent / "workspace"))

from app.main import app  # noqa: E402


@pytest.fixture
def client() -> AsyncClient:
    return AsyncClient(transport=ASGITransport(app=app), base_url="http://test")
```

`tests/test_search_correctness.py`:
```python
"""Single-request correctness."""
from __future__ import annotations

import pytest


@pytest.mark.asyncio
async def test_search_returns_expected_shape(client):
    response = await client.get("/search?q=foo")
    assert response.status_code == 200
    body = response.json()
    assert body["query"] == "foo"
    assert len(body["hits"]) == 3
```

`tests/test_throttle_under_load.py`:
```python
"""Concurrent test that detects event-loop blocking.

If the agent uses time.sleep (blocking) to enforce the 10/s cap, the
event loop is monopolized; 30 concurrent requests serialize, throughput
collapses below 1/s, and the test fails on the throughput assertion
(or times out via @pytest.mark.timeout).

If asyncio.sleep / aiolimiter is used correctly, the loop remains
responsive: the 30 requests complete at the configured 10/s cap in
~3-4 seconds with throughput >= 8/s.
"""
from __future__ import annotations

import asyncio
import time

import pytest


@pytest.mark.asyncio
@pytest.mark.timeout(15)
async def test_throughput_holds_under_concurrent_load(client):
    n_requests = 30
    start = time.monotonic()
    responses = await asyncio.gather(
        *(client.get(f"/search?q=q{i}") for i in range(n_requests))
    )
    elapsed = time.monotonic() - start

    # All complete.
    assert all(r.status_code == 200 for r in responses), \
        f"Some requests failed: {[r.status_code for r in responses]}"

    # Throughput floor: 30 requests / 4.5 seconds = 6.67/s; require >= 6/s
    # (gives room for OS noise but catches event-loop blocking).
    throughput = n_requests / elapsed
    assert throughput >= 6.0, \
        f"throughput {throughput:.2f}/s collapsed — event loop blocked? elapsed={elapsed:.2f}s"

    # Rate cap: must take at least ~ (n_requests-1)/10 seconds (small slack).
    expected_min_elapsed = (n_requests - 1) / 10 * 0.7  # 30% slack
    assert elapsed >= expected_min_elapsed, \
        f"finished in {elapsed:.2f}s; rate limit not enforced (expected >= {expected_min_elapsed:.2f}s)"
```

`pytest-timeout` is not in requirements; add it:

Update `workspace/requirements.txt` to include:
```
pytest-timeout>=2.0
```

`tests/test_scope.sh`:
```bash
#!/usr/bin/env bash
set -e
cd "$(dirname "$0")/../workspace"
CHANGED=$( { git diff --name-only baseline; git ls-files --others --exclude-standard; } | sort -u )
EXTRA=$(echo "$CHANGED" | grep -v -E '^app/search\.py$' || true)
if [[ -n "$EXTRA" ]]; then
    echo "Scope drift outside app/search.py:" >&2
    echo "$EXTRA" >&2
    exit 1
fi
```

- [ ] **Step 5: Reference**

`reference/search.py`:
```python
from __future__ import annotations

import asyncio
import time

_RATE = 10  # requests per second
_INTERVAL = 1.0 / _RATE
_lock = asyncio.Lock()
_last_emit = 0.0


async def search(q: str) -> dict:
    """Rate-limited search. Token-bucket on a single async lock."""
    global _last_emit
    async with _lock:
        now = time.monotonic()
        wait_for = _last_emit + _INTERVAL - now
        if wait_for > 0:
            await asyncio.sleep(wait_for)
        _last_emit = time.monotonic()
    return {"query": q, "hits": [f"result-{i}-for-{q}" for i in range(3)]}
```

- [ ] **Step 6: Smoke + commit**

Same smoke pattern (validate UNFIXED workspace fails the load test; reference passes all). Commit:
```bash
git add tasks/brownfield-wrong-abs-async-throttle/
git commit -m "Add brownfield-wrong-abs-async-throttle task (WRONG_ABS)"
```

---

## Task 4: brownfield-stop-early-multi-step-migration (STOP_EARLY)

**Files to create under** `tasks/brownfield-stop-early-multi-step-migration/`:
- `task.yaml`
- `setup.sh`, `eval.sh`
- `workspace/requirements.txt`, `workspace/README.md`, `workspace/alembic.ini`
- `workspace/app/__init__.py`, `workspace/app/models.py`, `workspace/app/schemas.py`, `workspace/app/db.py`
- `workspace/alembic/env.py`, `workspace/alembic/script.py.mako`
- `workspace/alembic/versions/0001_initial.py`
- `tests/__init__.py`, `tests/conftest.py`
- `tests/test_user_creates.py`, `tests/test_user_list_response.py`, `tests/test_migrations_apply.py`, `tests/test_scope.sh`
- `reference/models.py`, `reference/schemas.py`, `reference/0002_add_verified.py`

- [ ] **Step 1: task.yaml**

```yaml
id: brownfield-stop-early-multi-step-migration
name: Add verified column with model + schema + alembic migration
description: |
  Brownfield task. SQLAlchemy + Alembic + Pydantic project. Adding a
  `verified` boolean to User requires three coordinated changes:
  model, schema, and a new Alembic migration. Bare agents typically
  do steps 1 and 2 (which makes in-memory model tests pass) and
  declare done, missing the migration. The test fixture re-creates
  the DB via `alembic upgrade head`, so without the migration the
  column is absent in the real schema → tests fail. Stresses
  STOP_EARLY: declaring victory too soon.
category: brownfield
template_kind: builtin
workspace_mode: local
complexity_score: 7.0
codebase_type: python
metadata:
  primary_failure_mode: STOP_EARLY
task_prompt: |
  Add a `verified` boolean column to the User model (default False).

  You must:
    1. Add the column to the SQLAlchemy model in `app/models.py`.
    2. Add the field to the Pydantic schema in `app/schemas.py` so
       the API serializes it.
    3. Add a new Alembic migration under `alembic/versions/` that
       adds the column to the database (use `alembic revision` or
       hand-author following the existing 0001_initial.py shape).

  All existing tests must continue to pass. New tests verify the
  column is present in the schema after migrations run, defaults to
  False, and appears in API responses.
technical_details: |
  - workspace/tests/conftest.py creates a fresh sqlite at the start of
    each test session and runs `alembic upgrade head` against it.
  - test_migrations_apply.py inspects the sqlite schema post-upgrade
    and asserts a `verified` column exists with type BOOLEAN and
    default 0.
  - test_user_list_response.py serializes a User through the Pydantic
    schema and checks the dict contains 'verified'.
  - test_user_creates.py inserts a row with verified=True and reads
    it back.
  - test_scope.sh allows app/models.py, app/schemas.py, and any new
    file under alembic/versions/. Disallows changes to db.py, env.py,
    main.py, the existing 0001_initial.py migration, alembic.ini.
test_cases:
  - name: User can be created with verified column
    test_command: pytest -q tests/test_user_creates.py
    expected_result: exit 0
    ordering: 0
  - name: API response includes verified
    test_command: pytest -q tests/test_user_list_response.py
    expected_result: exit 0
    ordering: 1
  - name: Alembic migration adds the column to the schema
    test_command: pytest -q tests/test_migrations_apply.py
    expected_result: exit 0
    ordering: 2
  - name: Scope discipline (model + schema + new migration only)
    test_command: bash tests/test_scope.sh
    expected_result: exit 0
    ordering: 3
```

- [ ] **Step 2-7: setup.sh / eval.sh / workspace / tests / reference / smoke / commit**

`workspace/requirements.txt`:
```
sqlalchemy>=2.0
alembic>=1.13
pydantic>=2.7
pytest>=8.0
```

`workspace/alembic.ini`:
```ini
[alembic]
script_location = alembic
sqlalchemy.url = sqlite:///./test.db

[loggers]
keys = root,sqlalchemy,alembic

[handlers]
keys = console

[formatters]
keys = generic

[logger_root]
level = WARN
handlers = console
qualname =

[logger_sqlalchemy]
level = WARN
handlers =
qualname = sqlalchemy.engine

[logger_alembic]
level = INFO
handlers =
qualname = alembic

[handler_console]
class = StreamHandler
args = (sys.stderr,)
level = NOTSET
formatter = generic

[formatter_generic]
format = %(levelname)-5.5s [%(name)s] %(message)s
```

`workspace/alembic/env.py`:
```python
"""Alembic env — uses sqlite from alembic.ini."""
from __future__ import annotations

from logging.config import fileConfig

from alembic import context
from sqlalchemy import engine_from_config, pool

config = context.config
if config.config_file_name is not None:
    fileConfig(config.config_file_name)


def run_migrations_online() -> None:
    engine = engine_from_config(
        config.get_section(config.config_ini_section, {}),
        prefix="sqlalchemy.",
        poolclass=pool.NullPool,
    )
    with engine.connect() as conn:
        context.configure(connection=conn)
        with context.begin_transaction():
            context.run_migrations()


run_migrations_online()
```

`workspace/alembic/script.py.mako`:
```
"""${message}

Revision ID: ${up_revision}
Revises: ${down_revision | comma,n}
Create Date: ${create_date}
"""
from alembic import op
import sqlalchemy as sa


revision = ${repr(up_revision)}
down_revision = ${repr(down_revision)}


def upgrade():
    ${upgrades if upgrades else "pass"}


def downgrade():
    ${downgrades if downgrades else "pass"}
```

`workspace/alembic/versions/0001_initial.py`:
```python
"""Initial users table.

Revision ID: 0001
Revises:
"""
from alembic import op
import sqlalchemy as sa

revision = "0001"
down_revision = None


def upgrade():
    op.create_table(
        "users",
        sa.Column("id", sa.Integer, primary_key=True),
        sa.Column("name", sa.String(80), nullable=False),
    )


def downgrade():
    op.drop_table("users")
```

`workspace/app/__init__.py`: empty.

`workspace/app/db.py`:
```python
"""SQLAlchemy engine / session helpers."""
from __future__ import annotations

from sqlalchemy import create_engine
from sqlalchemy.orm import declarative_base, sessionmaker


engine = create_engine("sqlite:///./test.db", future=True)
Base = declarative_base()
SessionLocal = sessionmaker(bind=engine, autoflush=False, autocommit=False, future=True)
```

`workspace/app/models.py`:
```python
"""SQLAlchemy ORM models.

Agent task: add a `verified` Boolean column (default False).
"""
from __future__ import annotations

from sqlalchemy import Column, Integer, String

from app.db import Base


class User(Base):
    __tablename__ = "users"
    id = Column(Integer, primary_key=True)
    name = Column(String(80), nullable=False)
```

`workspace/app/schemas.py`:
```python
"""Pydantic schemas (API serializers).

Agent task: add a `verified` field that mirrors the ORM column.
"""
from __future__ import annotations

from pydantic import BaseModel, ConfigDict


class UserOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)
    id: int
    name: str
```

`workspace/README.md`:
```markdown
# user-store

SQLAlchemy + Alembic + Pydantic v2 project.

Adding a column requires THREE changes:
  1. Update the ORM model in `app/models.py`.
  2. Update the Pydantic schema in `app/schemas.py`.
  3. Add a new Alembic migration in `alembic/versions/`.

Tests re-create the database via `alembic upgrade head` for every
session — skipping step 3 will fail `tests/test_migrations_apply.py`
(and the other tests downstream, because the column is missing).
```

`tests/__init__.py`: empty.

`tests/conftest.py`:
```python
"""Pytest session fixture: fresh sqlite + alembic upgrade head."""
from __future__ import annotations

import os
import sys
import subprocess
from pathlib import Path

import pytest

WORKSPACE = Path(__file__).parent.parent / "workspace"
sys.path.insert(0, str(WORKSPACE))


@pytest.fixture(autouse=True)
def fresh_db(tmp_path, monkeypatch):
    db_file = tmp_path / "test.db"
    monkeypatch.setenv("SQLALCHEMY_DATABASE_URL", f"sqlite:///{db_file}")
    # Rewrite alembic.ini's URL via env-substitution: simplest path is
    # to run alembic with -x option that env.py picks up. For the
    # toy setup we just edit a copy in tmp.
    alembic_ini = tmp_path / "alembic.ini"
    text = (WORKSPACE / "alembic.ini").read_text()
    text = text.replace("sqlite:///./test.db", f"sqlite:///{db_file}")
    alembic_ini.write_text(text)
    # Copy alembic/ dir alongside.
    import shutil
    shutil.copytree(WORKSPACE / "alembic", tmp_path / "alembic")
    subprocess.run(
        ["alembic", "-c", str(alembic_ini), "upgrade", "head"],
        cwd=tmp_path, check=True, capture_output=True,
    )
    # Re-create the SQLAlchemy engine to point at the fresh DB.
    from sqlalchemy import create_engine
    import app.db as db_module
    db_module.engine = create_engine(f"sqlite:///{db_file}", future=True)
    db_module.SessionLocal = db_module.sessionmaker(bind=db_module.engine, autoflush=False, autocommit=False, future=True)
    yield
```

`tests/test_user_creates.py`:
```python
from __future__ import annotations

from app.db import SessionLocal
from app.models import User


def test_user_with_verified_true():
    session = SessionLocal()
    try:
        u = User(name="Alice", verified=True)  # agent must add 'verified' column
        session.add(u)
        session.commit()
        fetched = session.query(User).first()
        assert fetched.name == "Alice"
        assert fetched.verified is True
    finally:
        session.close()
```

`tests/test_user_list_response.py`:
```python
from __future__ import annotations

from app.models import User
from app.schemas import UserOut


def test_pydantic_user_serializes_verified():
    u = User(id=1, name="Alice", verified=True)
    payload = UserOut.model_validate(u).model_dump()
    assert payload == {"id": 1, "name": "Alice", "verified": True}
```

`tests/test_migrations_apply.py`:
```python
"""Inspect the post-migration schema for the verified column.

This is the trap test for STOP_EARLY. An agent that updates model +
schema but skips writing the migration will pass model-level tests
but the alembic-applied database will lack the column → this test
fails.
"""
from __future__ import annotations

from sqlalchemy import inspect

from app.db import engine


def test_users_table_has_verified_column():
    inspector = inspect(engine)
    columns = {col["name"]: col for col in inspector.get_columns("users")}
    assert "verified" in columns, \
        f"verified column missing from users table; columns={list(columns)}"
    col = columns["verified"]
    # Sqlite reports BOOLEAN as INTEGER but the type repr should contain BOOL.
    assert "BOOL" in str(col["type"]).upper() or "INTEGER" in str(col["type"]).upper(), \
        f"verified column type unexpected: {col['type']}"
```

`tests/test_scope.sh`:
```bash
#!/usr/bin/env bash
set -e
cd "$(dirname "$0")/../workspace"
CHANGED=$( { git diff --name-only baseline; git ls-files --others --exclude-standard; } | sort -u )
# Allow: app/models.py, app/schemas.py, any NEW alembic/versions/*.py
EXTRA=$(echo "$CHANGED" | grep -v -E '^(app/models\.py|app/schemas\.py|alembic/versions/[^/]+\.py)$' || true)
if [[ -n "$EXTRA" ]]; then
    echo "Scope drift:" >&2
    echo "$EXTRA" >&2
    exit 1
fi
# Existing initial migration must be untouched.
if ! git diff --quiet baseline -- alembic/versions/0001_initial.py 2>/dev/null; then
    echo "Existing 0001_initial.py migration was modified — not allowed." >&2
    exit 1
fi
```

- [ ] **Step 3: setup.sh** (note alembic also needs a no-op first run to validate)

```bash
#!/usr/bin/env bash
set -euo pipefail
pip install --no-cache-dir -r requirements.txt
if [[ ! -d .git ]]; then
    git init -q
    git config user.email "agentdx@frameval.local"
    git config user.name "AgentDx"
    git add -A
    git commit -q -m "AgentDx baseline (brownfield-stop-early-multi-step-migration)"
    git tag baseline
fi
```

- [ ] **Step 4: eval.sh**

```bash
#!/usr/bin/env bash
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/workspace"
pytest -q --tb=short ../tests/test_user_creates.py ../tests/test_user_list_response.py ../tests/test_migrations_apply.py
bash "$SCRIPT_DIR/tests/test_scope.sh"
```

- [ ] **Step 5: reference**

`reference/models.py`:
```python
from __future__ import annotations

from sqlalchemy import Boolean, Column, Integer, String

from app.db import Base


class User(Base):
    __tablename__ = "users"
    id = Column(Integer, primary_key=True)
    name = Column(String(80), nullable=False)
    verified = Column(Boolean, nullable=False, default=False)
```

`reference/schemas.py`:
```python
from __future__ import annotations

from pydantic import BaseModel, ConfigDict


class UserOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)
    id: int
    name: str
    verified: bool
```

`reference/0002_add_verified.py`:
```python
"""Add verified column to users.

Revision ID: 0002
Revises: 0001
"""
from alembic import op
import sqlalchemy as sa

revision = "0002"
down_revision = "0001"


def upgrade():
    op.add_column("users", sa.Column("verified", sa.Boolean(), nullable=False, server_default=sa.false()))


def downgrade():
    op.drop_column("users", "verified")
```

- [ ] **Step 6: Smoke + commit**

```bash
git add tasks/brownfield-stop-early-multi-step-migration/
git commit -m "Add brownfield-stop-early-multi-step-migration task (STOP_EARLY)"
```

---

## Task 5: brownfield-hal-api-pydantic-version (HAL_API)

**Files to create under** `tasks/brownfield-hal-api-pydantic-version/`:
- `task.yaml`
- `setup.sh`, `eval.sh`
- `workspace/requirements.txt`, `workspace/pyproject.toml`, `workspace/README.md`
- `workspace/app/__init__.py`, `workspace/app/models.py`
- `tests/__init__.py`, `tests/test_validator_works.py`, `tests/test_imports_v2_api.py`, `tests/test_scope.sh`
- `reference/models.py`

- [ ] **Step 1: task.yaml**

```yaml
id: brownfield-hal-api-pydantic-version
name: Add Pydantic v2 validator (not v1!) to User model
description: |
  Brownfield task. Project uses Pydantic 2.x (pinned in pyproject and
  requirements). Task: add a validator to the User model that rejects
  invalid email addresses. Bare agents with stale priors often use
  the v1 `@validator` decorator (now deprecated). The trap test
  greps for the v2 `@field_validator` import to enforce correct API
  usage. Stresses HAL_API: hallucinated/outdated API path.
category: brownfield
template_kind: builtin
workspace_mode: local
complexity_score: 4.5
codebase_type: python
metadata:
  primary_failure_mode: HAL_API
task_prompt: |
  Add a validator to the `User` model in `app/models.py` that rejects
  email addresses missing an `@` symbol. On rejection, raise a
  ValueError with the message "invalid email".

  The project uses Pydantic 2.x — check pyproject.toml.
technical_details: |
  - workspace/pyproject.toml pins pydantic ^2.7.
  - test_validator_works.py constructs User with bad and good email,
    asserts ValidationError on bad.
  - test_imports_v2_api.py greps app/models.py for:
      * MUST contain `from pydantic import field_validator` OR
        `from pydantic import field_validator, ...`
      * MUST NOT contain `from pydantic import validator` or
        `from pydantic import .*validator(?!_)`
  - test_scope.sh: only app/models.py may change.
test_cases:
  - name: Validator rejects invalid emails
    test_command: pytest -q tests/test_validator_works.py
    expected_result: exit 0
    ordering: 0
  - name: Validator uses Pydantic v2 API (field_validator, not validator)
    test_command: pytest -q tests/test_imports_v2_api.py
    expected_result: exit 0
    ordering: 1
  - name: Scope (only app/models.py)
    test_command: bash tests/test_scope.sh
    expected_result: exit 0
    ordering: 2
```

- [ ] **Step 2: setup.sh, eval.sh**

setup.sh standard. eval.sh:
```bash
#!/usr/bin/env bash
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/workspace"
pytest -q --tb=short ../tests/test_validator_works.py ../tests/test_imports_v2_api.py
bash "$SCRIPT_DIR/tests/test_scope.sh"
```

- [ ] **Step 3: workspace files**

`workspace/requirements.txt`:
```
pydantic>=2.7,<3
pytest>=8.0
```

`workspace/pyproject.toml`:
```toml
[project]
name = "user-models"
version = "0.1.0"
requires-python = ">=3.10"
dependencies = [
    "pydantic>=2.7,<3",
]
```

`workspace/README.md`:
```markdown
# user-models

Tiny Pydantic v2 project. Tests enforce v2 API usage —
`@field_validator`, NOT the v1 `@validator` decorator.

See pyproject.toml: `pydantic = "^2.7"` is hard-pinned.
```

`workspace/app/__init__.py`: empty.

`workspace/app/models.py`:
```python
"""User model.

Agent task: add a validator that rejects emails without an @ symbol.
Use Pydantic 2.x API (field_validator). The v1 @validator decorator
is deprecated and our import-discipline test rejects it.
"""
from __future__ import annotations

from pydantic import BaseModel


class User(BaseModel):
    name: str
    email: str
```

- [ ] **Step 4: tests**

`tests/__init__.py`: empty.

`tests/test_validator_works.py`:
```python
"""The validator rejects emails missing @ with ValueError 'invalid email'."""
from __future__ import annotations

import sys
from pathlib import Path

import pytest
from pydantic import ValidationError

sys.path.insert(0, str(Path(__file__).parent.parent / "workspace"))

from app.models import User


def test_valid_email_accepted():
    user = User(name="A", email="a@b.com")
    assert user.email == "a@b.com"


def test_invalid_email_rejected():
    with pytest.raises(ValidationError) as exc_info:
        User(name="A", email="no-at-sign")
    # The agent's error message should contain "invalid email"; ValidationError
    # wraps it. Search the rendered error string.
    assert "invalid email" in str(exc_info.value)
```

`tests/test_imports_v2_api.py`:
```python
"""Regex check enforcing Pydantic v2 API usage in app/models.py.

Bare agents with stale priors often write:
    from pydantic import validator
    @validator("email")

We reject that. v2 wants:
    from pydantic import field_validator
    @field_validator("email")
"""
from __future__ import annotations

import re
from pathlib import Path


MODELS_PATH = Path(__file__).parent.parent / "workspace" / "app" / "models.py"


def test_uses_field_validator_not_legacy_validator():
    text = MODELS_PATH.read_text()
    # Must import field_validator from pydantic.
    assert re.search(r"\bfield_validator\b", text), \
        "expected `field_validator` import (Pydantic v2 API)"
    # Must NOT import or use the legacy v1 `validator` decorator.
    # The regex excludes field_validator (the v2 name contains 'validator').
    illegal = re.findall(r"(?<!field_)\bvalidator\b", text)
    assert not illegal, \
        f"found legacy v1 `validator` decorator/import (got {illegal}); use field_validator"
```

`tests/test_scope.sh`:
```bash
#!/usr/bin/env bash
set -e
cd "$(dirname "$0")/../workspace"
CHANGED=$( { git diff --name-only baseline; git ls-files --others --exclude-standard; } | sort -u )
EXTRA=$(echo "$CHANGED" | grep -v -E '^app/models\.py$' || true)
if [[ -n "$EXTRA" ]]; then
    echo "Scope drift outside app/models.py:" >&2
    echo "$EXTRA" >&2
    exit 1
fi
```

- [ ] **Step 5: reference**

`reference/models.py`:
```python
from __future__ import annotations

from pydantic import BaseModel, field_validator


class User(BaseModel):
    name: str
    email: str

    @field_validator("email")
    @classmethod
    def _validate_email(cls, value: str) -> str:
        if "@" not in value:
            raise ValueError("invalid email")
        return value
```

- [ ] **Step 6: Smoke + commit**

```bash
git add tasks/brownfield-hal-api-pydantic-version/
git commit -m "Add brownfield-hal-api-pydantic-version task (HAL_API)"
```

---

## Task 6: Verify the engine loads all 5 new tasks

**Files:** none (engine seeding is automatic from the tasks/ directory).

- [ ] **Step 1: Restart engine and check task listing**

From the repo root:
```bash
cd engine && go run cmd/server/main.go &
sleep 3
curl -s http://localhost:8080/api/tasks | python3 -m json.tool | grep '"id"'
# Expect to see all 5 new task IDs alongside the existing 3.
kill %1
```

If any task is missing, inspect the engine's startup log for parse errors and fix the offending `task.yaml`.

- [ ] **Step 2: Spot-check one task in the UI**

```bash
make dev-full
# Open http://localhost:5173/tasks — verify the new tasks appear
# and clicking one shows the task_prompt + test_cases populated.
```

- [ ] **Step 3: No commit required for verification.** If a task fails to load, that's a bug in its task.yaml — fix and amend the offending task's commit.

---

## Self-review

**Spec coverage:**
- §4.1 (MISREAD task) → Task 1
- §4.2 (SCOPE_DRIFT task) → Task 2
- §4.3 (WRONG_ABS task) → Task 3
- §4.4 (STOP_EARLY task) → Task 4
- §4.5 (HAL_API task) → Task 5
- §5 (common test discipline — scope shell + git baseline) → covered per task
- §6 (`primary_failure_mode` metadata) → each task.yaml has `metadata.primary_failure_mode`; no engine change needed
- §7 (risks — bare pass rate calibration) → manual calibration loop is a post-merge activity per spec §8 step 7; not in this plan's checklist beyond noting it
- §8 (rollout order) → matches Tasks 1-5 sequence

All covered.

**Placeholder check:** No TBD/TODO/"implement later" inside the plan's instructions. The literal `# TODO: refactor this mess` strings INSIDE Task 2's workspace files are intentional bait — they're task content, not plan placeholders.

**Type consistency:**
- `metadata.primary_failure_mode` value matches FailureCode enum strings exactly in every task.yaml (MISREAD, SCOPE_DRIFT, WRONG_ABS, STOP_EARLY, HAL_API).
- `test_scope.sh` template is consistent across tasks (always relies on `baseline` git tag from setup.sh).
- Each task's `eval.sh` references the actual test filenames listed in that task's tests section.

Plan ready.
