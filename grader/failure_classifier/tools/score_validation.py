"""Score the failure classifier against a hand-labeled validation set.

Reads two inputs:
  --labels   CSV with hand-labeled ground truth (one row per run)
  --runs     JSON or DSN providing the classifier's per-run verdict

Emits a markdown report with confusion matrix + per-category P/R/F1 +
macro-F1 over the 12 failure categories (NONE is treated as "no failure"
and excluded from macro averaging — multi-label macro-F1 over 12 codes
is what spec §4.7.5 calls for).

This is the script Story #25's procedure references. Designed to be
runnable offline against any (gold, predicted) pair source so the
calibration study can be re-run as the classifier prompt evolves.
"""
from __future__ import annotations

import argparse
import csv
import json
import sys
from collections import defaultdict
from pathlib import Path
from typing import Iterable

from grader.failure_classifier.taxonomy import FailureCode

# Codes we evaluate per-category. NONE is intentionally excluded — it's
# the absence-of-failure case and counting it as a class would dominate
# macro averages on clean runs.
FAILURE_CATEGORIES: list[FailureCode] = [c for c in FailureCode if c != FailureCode.NONE]


def load_labels(path: Path) -> dict[str, dict]:
    """Parse the hand-labels CSV into {run_id: {primary, secondary_set}}."""
    out: dict[str, dict] = {}
    with path.open() as fh:
        reader = csv.DictReader(fh)
        for row in reader:
            run_id = row["run_id"].strip()
            if not run_id:
                continue
            secondary_raw = (row.get("secondary") or "").strip()
            secondary = {s.strip() for s in secondary_raw.split(";") if s.strip()} | {
                s.strip() for s in secondary_raw.split(",") if s.strip()
            }
            out[run_id] = {
                "primary": row["primary"].strip(),
                "secondary": secondary,
                "labeler": (row.get("labeler") or "").strip(),
                "notes": (row.get("notes") or "").strip(),
            }
    return out


def load_predictions(path: Path) -> dict[str, dict]:
    """Parse the classifier predictions JSON into {run_id: {primary, secondary_set}}.

    Expected JSON shape (matches what the gRPC ClassifyFailure handler
    returns, serialized via Pydantic):

        [
          {"run_id": "abc-123", "classification": {"primary": "HAL_API",
                                                    "secondary": ["DEP_MISS"]}},
          ...
        ]
    """
    out: dict[str, dict] = {}
    with path.open() as fh:
        rows = json.load(fh)
    for row in rows:
        run_id = row["run_id"]
        cls = row["classification"]
        out[run_id] = {
            "primary": cls["primary"],
            "secondary": set(cls.get("secondary", [])),
        }
    return out


def labels_for_run(record: dict) -> set[str]:
    """Multi-label set for one run: primary + all secondaries."""
    s = set(record.get("secondary", set()))
    if record["primary"] != "NONE":
        s.add(record["primary"])
    return s


def compute_per_category(
    gold: dict[str, dict], pred: dict[str, dict]
) -> dict[str, dict[str, float]]:
    """Per-category {tp, fp, fn, precision, recall, f1} for each FailureCode."""
    stats: dict[str, dict[str, float]] = {
        code.value: {"tp": 0, "fp": 0, "fn": 0} for code in FAILURE_CATEGORIES
    }
    for run_id, g in gold.items():
        p = pred.get(run_id)
        if p is None:
            continue
        g_set = labels_for_run(g)
        p_set = labels_for_run(p)
        for code in FAILURE_CATEGORIES:
            v = code.value
            in_g = v in g_set
            in_p = v in p_set
            if in_g and in_p:
                stats[v]["tp"] += 1
            elif in_p and not in_g:
                stats[v]["fp"] += 1
            elif in_g and not in_p:
                stats[v]["fn"] += 1
    for v, s in stats.items():
        tp, fp, fn = s["tp"], s["fp"], s["fn"]
        s["precision"] = tp / (tp + fp) if (tp + fp) > 0 else 0.0
        s["recall"] = tp / (tp + fn) if (tp + fn) > 0 else 0.0
        s["f1"] = (
            2 * s["precision"] * s["recall"] / (s["precision"] + s["recall"])
            if (s["precision"] + s["recall"]) > 0
            else 0.0
        )
    return stats


def macro_f1(per_category: dict[str, dict[str, float]]) -> float:
    f1s = [v["f1"] for v in per_category.values()]
    return sum(f1s) / len(f1s) if f1s else 0.0


def confusion_pairs(
    gold: dict[str, dict], pred: dict[str, dict]
) -> list[tuple[str, str, int]]:
    """Top confusion pairs: (gold_code, predicted_code, count)."""
    pairs: dict[tuple[str, str], int] = defaultdict(int)
    for run_id, g in gold.items():
        p = pred.get(run_id)
        if p is None:
            continue
        g_primary = g["primary"]
        p_primary = p["primary"]
        if g_primary != p_primary:
            pairs[(g_primary, p_primary)] += 1
    return sorted(((g, p, c) for (g, p), c in pairs.items()), key=lambda t: -t[2])


def render_report(
    *,
    gold: dict[str, dict],
    pred: dict[str, dict],
    per_category: dict[str, dict[str, float]],
    macro: float,
    confusions: Iterable[tuple[str, str, int]],
) -> str:
    lines: list[str] = []
    lines.append("# Failure Classifier Validation Report\n")
    lines.append(f"- Labeled runs: {len(gold)}")
    lines.append(f"- Predicted runs: {len(pred)}")
    lines.append(f"- Macro-F1 over 12 failure categories: **{macro:.3f}**\n")

    lines.append("## Per-category precision / recall / F1\n")
    lines.append("| Code | TP | FP | FN | Precision | Recall | F1 |")
    lines.append("|---|---:|---:|---:|---:|---:|---:|")
    for code in FAILURE_CATEGORIES:
        s = per_category[code.value]
        lines.append(
            f"| {code.value} | {s['tp']} | {s['fp']} | {s['fn']} | "
            f"{s['precision']:.3f} | {s['recall']:.3f} | {s['f1']:.3f} |"
        )
    lines.append("")

    lines.append("## Top confusion pairs (primary label disagreements)\n")
    if not confusions:
        lines.append("(no disagreements observed)")
    else:
        lines.append("| Gold | Predicted | Count |")
        lines.append("|---|---|---:|")
        for g, p, c in list(confusions)[:10]:
            lines.append(f"| {g} | {p} | {c} |")
    lines.append("")
    return "\n".join(lines)


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--labels", type=Path, required=True, help="CSV with hand labels")
    parser.add_argument("--predictions", type=Path, required=True, help="JSON with classifier verdicts")
    parser.add_argument("--out", type=Path, required=True, help="Output markdown report path")
    args = parser.parse_args(argv)

    gold = load_labels(args.labels)
    pred = load_predictions(args.predictions)
    per_cat = compute_per_category(gold, pred)
    macro = macro_f1(per_cat)
    confusions = confusion_pairs(gold, pred)

    report = render_report(
        gold=gold, pred=pred, per_category=per_cat, macro=macro, confusions=confusions
    )
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(report)
    print(f"wrote {args.out} (macro-F1 = {macro:.3f})")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
