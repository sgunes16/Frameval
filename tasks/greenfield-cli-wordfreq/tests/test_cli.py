"""End-to-end tests for the wordfreq CLI.

The agent's solution lives in ../workspace; tests invoke it as a subprocess
so they exercise the full CLI surface (arg parsing, exit codes, stderr,
stdout format) rather than poking at internals.

Tests must:
  - run from the workspace directory (eval.sh handles cd)
  - not import from the workspace
  - exit non-zero on failure
"""
from __future__ import annotations

import re
import shutil
import subprocess
import sys
from pathlib import Path

import pytest

WORKSPACE = Path(__file__).resolve().parent.parent / "workspace"
CLI = WORKSPACE / "wordfreq.py"


def run(args, *, expect_exit: int | None = None, sample: str | None = None, cwd: Path = WORKSPACE):
    """Invoke `python wordfreq.py <args>` and capture stdout/stderr.

    `sample` is written to a temp file inside `cwd` and its path is passed as
    the first arg unless the test already supplies a path.
    """
    if not CLI.exists():
        pytest.fail(f"wordfreq.py not found at {CLI}")
    full_args = [sys.executable, str(CLI)]
    if sample is not None:
        sample_path = cwd / "_sample.txt"
        sample_path.write_text(sample)
        full_args.append(str(sample_path))
    full_args.extend(args)
    proc = subprocess.run(full_args, capture_output=True, text=True, cwd=cwd)
    if expect_exit is not None and proc.returncode != expect_exit:
        pytest.fail(
            f"unexpected exit {proc.returncode} (expected {expect_exit})\n"
            f"stdout:\n{proc.stdout}\nstderr:\n{proc.stderr}"
        )
    return proc


@pytest.fixture(autouse=True)
def _cleanup_sample():
    yield
    for stale in WORKSPACE.glob("_sample*.txt"):
        stale.unlink(missing_ok=True)


def test_basic_top_k():
    """Default invocation should return the 10 most common words by count."""
    sample = " ".join(["apple"] * 5 + ["banana"] * 3 + ["cherry"] * 2 + ["d"])
    proc = run([], sample=sample, expect_exit=0)
    lines = [line for line in proc.stdout.strip().splitlines() if line.strip()]
    assert lines, f"no output\n{proc.stdout}"
    first = lines[0]
    assert first.lower().startswith("apple"), f"first word should be 'apple' (5 occurrences); got {first!r}"


def test_k_flag():
    """`-k 3` should return exactly 3 lines.

    Builds 10 distinct purely-alphabetic words ("aa", "ab", ... "aj") each
    repeated with descending frequency so the top-3 is deterministic. Pure
    alphabetic tokens needed because the task spec says tokenization keeps
    only alphabetic characters.
    """
    import string

    sample = " ".join(("a" + ch) * (10 - i) for i, ch in enumerate(string.ascii_lowercase[:10]))
    proc = run(["-k", "3"], sample=sample, expect_exit=0)
    lines = [line for line in proc.stdout.strip().splitlines() if line.strip()]
    assert len(lines) == 3, f"expected 3 lines, got {len(lines)}\n{proc.stdout}"


def test_case_sensitive():
    """`-c` makes 'Hello' and 'hello' distinct words."""
    sample = "Hello hello Hello"
    proc = run(["-c"], sample=sample, expect_exit=0)
    lines = [line for line in proc.stdout.strip().splitlines() if line.strip()]
    # With case-sensitivity, there should be at least 2 distinct entries.
    words = [line.split(":", 1)[0].strip() for line in lines]
    assert "Hello" in words and "hello" in words, (
        f"-c should keep Hello/hello distinct; got words={words}"
    )


def test_missing_file():
    """Invocation against a non-existent path exits 1 with a stderr message.

    Guard against the false-positive case where the agent never wrote
    `wordfreq.py` at all: Python itself errors out with "can't open file"
    which also produces non-zero exit + stderr, but tells us nothing about
    the agent's missing-file handling. Fail explicitly when the CLI is
    absent so a pristine workspace reads as 0/5 rather than 1/5.
    """
    if not CLI.exists():
        pytest.fail(f"wordfreq.py not found at {CLI}")
    proc = subprocess.run(
        [sys.executable, str(CLI), "/nonexistent-path-zzz.txt"],
        capture_output=True,
        text=True,
        cwd=WORKSPACE,
    )
    assert proc.returncode != 0, "expected non-zero exit for missing file"
    assert proc.stderr.strip(), "expected an error message on stderr"
    # Sanity: the stderr must come from the agent's CLI, not Python's "can't
    # open file" wrapper. Reject the latter so we don't accept a stub that
    # crashes on startup as "handled the missing file".
    if "can't open file" in proc.stderr.lower():
        pytest.fail(f"stderr looks like Python's launcher error, not the agent's handling: {proc.stderr!r}")


def test_output_format():
    """Every non-empty output line must match `WORD: COUNT` (regex)."""
    sample = "alpha beta alpha gamma alpha beta"
    proc = run([], sample=sample, expect_exit=0)
    pattern = re.compile(r"^[A-Za-z]+:\s*\d+\s*$")
    for line in proc.stdout.strip().splitlines():
        line = line.strip()
        if not line:
            continue
        assert pattern.match(line), f"line {line!r} does not match WORD: COUNT format"
