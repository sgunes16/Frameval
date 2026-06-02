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


def test_uses_field_validator_not_legacy_validator():
    tree = ast.parse(MODELS_PATH.read_text())
    imported = _pydantic_imported_names(tree)
    decorators = _decorator_names(tree)

    uses_v2 = "field_validator" in imported or "field_validator" in decorators
    assert uses_v2, "expected Pydantic v2 `field_validator` (import or decorator)"

    uses_v1 = "validator" in imported or "validator" in decorators
    assert not uses_v1, (
        "found legacy v1 `validator` decorator/import; use `field_validator`"
    )
