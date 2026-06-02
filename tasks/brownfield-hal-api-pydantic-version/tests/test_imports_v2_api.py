"""Enforce Pydantic v2 API usage in app/models.py.

Bare agents with stale priors often write the v1 decorator:
    from pydantic import validator
    @validator("email")

v2 wants:
    from pydantic import field_validator
    @field_validator("email")

The check inspects the *parsed code* — imports and decorators — not the
raw file text. An earlier text-grep version produced false positives:
the task docstring legitimately contains the word "validator" while
explaining the task, so any agent that kept the docstring failed even
with correct `field_validator` code. AST inspection ignores comments
and docstrings, so it measures the API actually used.
"""
from __future__ import annotations

import ast
from pathlib import Path


MODELS_PATH = Path(__file__).parent.parent / "app" / "models.py"


def _decorator_base_name(node: ast.expr) -> str | None:
    """Return the trailing identifier of a decorator expression.

    Handles `@validator`, `@validator(...)`, `@pydantic.validator`,
    and `@pydantic.field_validator(...)`.
    """
    if isinstance(node, ast.Call):
        node = node.func
    if isinstance(node, ast.Attribute):
        return node.attr
    if isinstance(node, ast.Name):
        return node.id
    return None


def _pydantic_imported_names(tree: ast.AST) -> set[str]:
    names: set[str] = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.ImportFrom) and node.module:
            if node.module.split(".")[0] == "pydantic":
                for alias in node.names:
                    names.add(alias.name)
    return names


def _decorator_names(tree: ast.AST) -> set[str]:
    names: set[str] = set()
    for node in ast.walk(tree):
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            for dec in node.decorator_list:
                base = _decorator_base_name(dec)
                if base:
                    names.add(base)
    return names


# v2 validator decorators (field-level and whole-model). Either is fine —
# the trap is about v1-vs-v2 API, not field_validator specifically.
V2_VALIDATORS = {"field_validator", "model_validator"}
# v1 decorators that signal the outdated API (HAL_API failure mode).
V1_VALIDATORS = {"validator", "root_validator"}


def test_uses_v2_validator_not_legacy_validator():
    tree = ast.parse(MODELS_PATH.read_text())
    names = _pydantic_imported_names(tree) | _decorator_names(tree)

    assert V2_VALIDATORS & names, (
        "expected a Pydantic v2 validator (field_validator/model_validator)"
    )
    legacy = V1_VALIDATORS & names
    assert not legacy, (
        f"found legacy v1 API {sorted(legacy)}; use the v2 validator API"
    )
