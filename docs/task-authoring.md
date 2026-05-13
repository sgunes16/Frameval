# Authoring AgentDx tasks

AgentDx tasks are the **stress fixtures** that the harness runs against. A
good task gives the agent a non-trivial job, has tests that catch realistic
failure modes, and produces a 30–80 % pass rate on the baseline `bare`
harness — that judgment-rich middle ground is where diagnostic signal lives.

This guide collects the conventions established by the 3 MVP tasks under
`tasks/`. Use it when authoring a 4th task or porting an existing
benchmark into the AgentDx format.

---

## Anatomy of a task

```
tasks/<task-id>/
├── task.yaml                  Metadata + agent prompt + test_cases
├── workspace/                 Initial files the agent sees (empty for greenfield)
├── tests/                     OUTSIDE workspace; agent never sees these
│   ├── __init__.py
│   ├── conftest.py            (optional; shared fixtures)
│   └── test_*.py
├── harness_context/           (optional) per-harness instruction files
│   ├── claudemd.md            picked up by the `claudemd` harness
│   └── constitution.md        picked up by the `speckit` harness
├── reference/                 Canonical solution; passes all tests
├── setup.sh                   Runs in sandbox before agent invocation
├── eval.sh                    Runs in sandbox after agent invocation
└── pyproject.toml             (optional) pytest config, e.g., asyncio_mode
```

**Mount story.** The orchestrator mounts `workspace/` read-write into the
sandbox at `/workspace`, and `tests/` read-only at a sibling path the agent
cannot reach. `setup.sh` and `eval.sh` are run BY the orchestrator INSIDE
the sandbox — they have access to both.

---

## The four test recipes

Almost every realistic AgentDx test is one of these shapes.

### Recipe 1 — Detect the fix (brownfield bug-fix)

The bug exists in the starter workspace; the test fails on the broken code
and passes once the agent fixes it. Use this when the task description is
"X is broken; fix it without breaking the API contract".

```python
# tests/test_race_fixed.py
import asyncio, httpx, pytest

@pytest.mark.asyncio
async def test_concurrent_add_credits_no_lost_update():
    transport = httpx.ASGITransport(app=fresh_app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        await asyncio.gather(*(client.post("/users/1/credits", json={"amount": 1}) for _ in range(100)))
        body = (await client.get("/users/1")).json()
        assert body["credits"] == 100, f"race not fixed; got {body['credits']}"
```

See `tasks/brownfield-fix-async-race/tests/test_race_fixed.py` for the
full pattern with a `fresh_app` fixture that reloads the agent's modules
between tests.

### Recipe 2 — No regression (brownfield API contract)

The agent must NOT change the public contract. Snapshot the existing
response shape and assert it stays equal.

```python
# tests/test_api_unchanged.py
async def test_get_user_schema_unchanged(fresh_app):
    transport = httpx.ASGITransport(app=fresh_app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        body = (await client.get("/users/1")).json()
        assert set(body.keys()) == {"id", "name", "credits"}, f"unexpected keys: {sorted(body.keys())}"
        assert isinstance(body["id"], int)
        assert isinstance(body["credits"], int)
```

### Recipe 3 — Scope discipline (`git diff` test)

For brownfield tasks, the agent must NOT touch unrelated files.
`setup.sh` `git init`s the workspace and tags `baseline`; a shell test
asserts `git diff baseline` lists only expected paths.

```bash
# tests/test_scope.sh
#!/usr/bin/env bash
set -euo pipefail
WORKSPACE="$(dirname "$0")/../workspace"
cd "$WORKSPACE"
if ! git rev-parse --verify --quiet baseline >/dev/null; then
    echo "scope check: baseline tag missing; setup.sh did not complete" >&2
    exit 2
fi
CHANGED=$(git diff --name-only baseline -- 2>/dev/null || true)
UNTRACKED=$(git ls-files --others --exclude-standard 2>/dev/null || true)
ALL_TOUCHED=$(printf "%s\n%s\n" "$CHANGED" "$UNTRACKED" | sed '/^$/d' | sort -u)
EXPECTED="app/user_service.py"
UNEXPECTED=$(echo "$ALL_TOUCHED" | grep -vxF "$EXPECTED" || true)
if [[ -n "$UNEXPECTED" ]]; then
    echo "scope check: agent modified unexpected files:" >&2
    echo "$UNEXPECTED" >&2
    exit 1
fi
if [[ -z "$ALL_TOUCHED" ]]; then
    echo "scope check: no files modified (did the agent run?)" >&2
    exit 1
fi
```

### Recipe 4 — Behavior battery (greenfield)

For greenfield tasks where the agent writes everything from scratch, split
the contract into 3–5 behavioral assertions. Use a fail-fast guard so a
pristine workspace produces 0/N, not 1/N.

```python
# tests/test_cli.py
WORKSPACE = Path(__file__).resolve().parent.parent / "workspace"
CLI = WORKSPACE / "wordfreq.py"

def run(args, *, sample=None):
    if not CLI.exists():
        pytest.fail(f"wordfreq.py not found at {CLI}")  # fail-fast guard
    # ... subprocess invocation ...

def test_basic_top_k():
    sample = " ".join(["apple"] * 5 + ["banana"] * 3 + ["cherry"] * 2)
    proc = run([], sample=sample)
    assert proc.returncode == 0
    # ... assertions on output ...
```

See `tasks/greenfield-cli-wordfreq/tests/test_cli.py` for the full pattern.

---

## The four anti-patterns

### Anti-pattern 1 — Tests inside `workspace/`

The agent will see test files and reverse-engineer them. Always put tests
**outside** `workspace/` (typically in a sibling `tests/` dir).

### Anti-pattern 2 — Testing code patterns instead of behavior

```python
# DON'T:
assert "asyncio.Lock" in workspace_file_content  # brittle

# DO:
# Trigger the race condition concurrently and assert observable behavior
assert body["credits"] == 100
```

The agent might use a different (correct) approach. Test the outcome, not
the code shape.

### Anti-pattern 3 — Network-dependent `setup.sh`

`pip install -r requirements.txt` is fine because it's bounded and cacheable.
Calling external APIs during setup is not — it makes runs flaky and binds
your diagnostic signal to network conditions. If the task needs external
state, vendor it into `workspace/` as fixture data.

### Anti-pattern 4 — All-or-nothing tests

A single mega-test that asserts the entire contract gives the diagnostic
pipeline only 1 bit of signal (pass/fail). Three smaller tests give 8 states
and let the failure classifier latch onto specific failure modes (e.g.,
"test_basic passes but test_window_resets fails → likely STOP_EARLY").

---

## task.yaml schema

The minimal task.yaml looks like:

```yaml
id: my-task
name: Short human-readable title
description: |
  One paragraph explaining the task and what failure modes it stresses.
category: greenfield   # or: brownfield, bug-fix, refactor
template_kind: builtin
workspace_mode: empty  # or: local, git
complexity_score: 5.0  # 1.0–10.0
codebase_type: python  # or: javascript, go, …

task_prompt: |
  The prompt the agent sees verbatim.

technical_details: |
  Hidden notes for the test author / reviewer; not shown to the agent.

test_cases:
  - name: First test name (human-readable)
    test_command: pytest -q tests/test_my.py::test_first
    expected_result: exit 0
    ordering: 0
  - name: Second test
    test_command: pytest -q tests/test_my.py::test_second
    expected_result: exit 0
    ordering: 1
```

Brownfield tasks also set:

```yaml
workspace_mode: local                              # snapshot workspace/ at setup
# or
workspace_mode: git
workspace_git_url: https://github.com/example/repo.git
workspace_git_ref: main
```

---

## Calibrating difficulty

After writing a task, run it through `bare + aider + Qwen2.5-Coder-7B` ×
5 replicas. Target: 30–80 % pass rate.

| Pass rate | What to change |
|---|---|
| 0% | Task or tests broken. Apply your `reference/` solution and ensure 100%. |
| 0–20% | Too hard. Soften prompt, add hints, or expand allowed dependencies. |
| 30–80% | Sweet spot. Keep. |
| 80–100% | Too easy. Tighten tests (add an edge case, require a specific flag). |

The judgment-rich middle ground is where AgentDx differentiates harnesses —
near-100 % runs all look the same on the radar.

---

## Cross-references

- 3 reference tasks under `tasks/` for full working examples
- Spec §4.5 (`docs/superpowers/specs/2026-05-12-agentdx-design.md`) for the
  authoritative format
- `examples/03-add-your-own-harness/` (TODO post-MVP) for the harness side
