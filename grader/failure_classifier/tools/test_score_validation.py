"""Tests for the calibration scoring helper.

Uses pytest's tmp_path. No external network or LLM calls. Verifies the
core math (TP/FP/FN counting, macro-F1, confusion pairs) on tiny fixture
inputs so the calibration pipeline is reproducible.
"""
from __future__ import annotations

import csv
import json
from pathlib import Path

import pytest

from grader.failure_classifier.tools import score_validation as sv


def write_labels(path: Path, rows: list[dict]) -> None:
    with path.open("w", newline="") as fh:
        writer = csv.DictWriter(
            fh,
            fieldnames=["run_id", "primary", "secondary", "labeler", "notes", "labeled_at"],
        )
        writer.writeheader()
        for row in rows:
            writer.writerow(row)


def write_predictions(path: Path, rows: list[dict]) -> None:
    path.write_text(json.dumps(rows))


def test_perfect_agreement_yields_macro_f1_one(tmp_path: Path):
    labels = tmp_path / "labels.csv"
    preds = tmp_path / "preds.json"
    out = tmp_path / "report.md"

    write_labels(
        labels,
        [
            {"run_id": "r1", "primary": "HAL_API", "secondary": "", "labeler": "x", "notes": "", "labeled_at": ""},
            {"run_id": "r2", "primary": "STOP_EARLY", "secondary": "", "labeler": "x", "notes": "", "labeled_at": ""},
        ],
    )
    write_predictions(
        preds,
        [
            {"run_id": "r1", "classification": {"primary": "HAL_API", "secondary": []}},
            {"run_id": "r2", "classification": {"primary": "STOP_EARLY", "secondary": []}},
        ],
    )
    rc = sv.main(["--labels", str(labels), "--predictions", str(preds), "--out", str(out)])
    assert rc == 0
    report = out.read_text()
    # Only the 2 categories present in the gold set should have F1=1.000;
    # everything else should be 0.000 (no positives to score). Macro
    # averages over all 12 → 2/12 ≈ 0.167.
    assert "Macro-F1 over 12 failure categories: **0.167**" in report


def test_pure_disagreement_yields_zero_f1(tmp_path: Path):
    labels = tmp_path / "labels.csv"
    preds = tmp_path / "preds.json"
    out = tmp_path / "report.md"

    write_labels(
        labels,
        [{"run_id": "r1", "primary": "HAL_API", "secondary": "", "labeler": "", "notes": "", "labeled_at": ""}],
    )
    write_predictions(
        preds,
        [{"run_id": "r1", "classification": {"primary": "STOP_EARLY", "secondary": []}}],
    )
    rc = sv.main(["--labels", str(labels), "--predictions", str(preds), "--out", str(out)])
    assert rc == 0
    report = out.read_text()
    # HAL_API has FN=1, STOP_EARLY has FP=1 → both F1=0 → macro=0
    assert "Macro-F1 over 12 failure categories: **0.000**" in report
    assert "| HAL_API | STOP_EARLY | 1 |" in report  # confusion pair


def test_multi_label_secondaries_counted(tmp_path: Path):
    labels = tmp_path / "labels.csv"
    preds = tmp_path / "preds.json"
    out = tmp_path / "report.md"

    write_labels(
        labels,
        [
            {
                "run_id": "r1",
                "primary": "HAL_API",
                "secondary": "DEP_MISS",  # secondary label
                "labeler": "",
                "notes": "",
                "labeled_at": "",
            },
        ],
    )
    write_predictions(
        preds,
        [
            {
                "run_id": "r1",
                "classification": {
                    "primary": "HAL_API",
                    "secondary": ["DEP_MISS"],
                },
            }
        ],
    )
    rc = sv.main(["--labels", str(labels), "--predictions", str(preds), "--out", str(out)])
    assert rc == 0
    report = out.read_text()
    # Both HAL_API and DEP_MISS should score F1=1 on this single-run set
    assert "| HAL_API | 1 | 0 | 0 | 1.000 | 1.000 | 1.000 |" in report
    assert "| DEP_MISS | 1 | 0 | 0 | 1.000 | 1.000 | 1.000 |" in report


def test_missing_prediction_is_ignored_not_crashed(tmp_path: Path):
    # Gold has r1 + r2; predictions only have r1. Scoring should treat r2
    # as not-graded (no TP/FP/FN contribution) rather than panicking.
    labels = tmp_path / "labels.csv"
    preds = tmp_path / "preds.json"
    out = tmp_path / "report.md"

    write_labels(
        labels,
        [
            {"run_id": "r1", "primary": "HAL_API", "secondary": "", "labeler": "", "notes": "", "labeled_at": ""},
            {"run_id": "r2", "primary": "STOP_EARLY", "secondary": "", "labeler": "", "notes": "", "labeled_at": ""},
        ],
    )
    write_predictions(
        preds,
        [{"run_id": "r1", "classification": {"primary": "HAL_API", "secondary": []}}],
    )
    rc = sv.main(["--labels", str(labels), "--predictions", str(preds), "--out", str(out)])
    assert rc == 0
    report = out.read_text()
    assert "Labeled runs: 2" in report
    assert "Predicted runs: 1" in report
    # Only HAL_API has score 1.0; STOP_EARLY still 0 (not graded)
    assert "| HAL_API | 1 | 0 | 0 | 1.000 | 1.000 | 1.000 |" in report
