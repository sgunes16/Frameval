# Failure Classifier Calibration

The AgentDx failure classifier (Story #22) is a Pydantic-typed LLM judge.
On its own, it gives plausible-looking categorical verdicts on every run.
To turn those verdicts from "the classifier said HAL_API" into the
defendable "the classifier said HAL_API and it agrees with the human
ground truth 78 % of the time on this validation set", you need a
calibration study.

This directory is the home for that study: procedure docs, the
labeled validation dataset, and the accuracy numbers downstream papers
will cite.

## Procedure (per spec §4.7.5)

1. **Generate ~100 runs across the harness × task grid.** Target
   distribution: 5 harnesses × 3 tasks × ~7 replicas, balanced to ≈100
   runs total. Use the existing `/api/experiments` UI or a script.

2. **Hand-label each run.** For every run, read the transcript tail + the
   test results and assign one primary FailureCode + 0–3 secondary codes
   from the 13-value taxonomy (see Appendix A of the design spec).

   Use the CSV schema at `labels-template.csv` in this directory. Each row:

   ```
   run_id,primary,secondary,labeler,notes,labeled_at
   abc-123,HAL_API,DEP_MISS,mustafa,"agent imported fastapi_limiter without installing it",2026-06-XX
   ```

   2-hour budget per the project plan; ~1.2 min/run if you skim the
   transcript tail and tests-failed count.

3. **Run the classifier over the same runs.** Each run already has a
   `diagnostic.failure_label` row populated by the gRPC ClassifyFailure
   call after the orchestrator integration lands. Until that integration
   ships, export the classifier predictions to a JSON file (one record
   per run with `{"run_id": "...", "classification": {...}}`) and feed
   it to the scorer:

   ```bash
   cd grader
   PYTHONPATH=.. uv run python -m grader.failure_classifier.tools.score_validation \
       --labels ../docs/calibration/labels-template.csv \
       --predictions <path-to-predictions.json> \
       --out ../docs/calibration/2026-06-validation.md
   ```

   The `score_validation` script (shipped in
   `grader/failure_classifier/tools/`) accepts `--predictions` as a JSON
   file matching the gRPC `ClassifyFailureResponse` shape.

4. **Per-category accuracy.** The script outputs:
   - Confusion matrix (predicted vs gold)
   - Per-category precision / recall / F1
   - Macro-F1 across the 12 failure categories
   - Top-3 most-confused pairs (HAL_API ↔ DEP_MISS, etc.)

5. **Threshold gate.** Per Story #25 acceptance: macro-F1 ≥ 0.60. If the
   bar isn't hit, the classifier prompts in
   `grader/failure_classifier/prompts.py` need iteration before the
   thesis chapter is defensible.

## Files in this directory

| File | What |
|---|---|
| `labels-template.csv` | Empty CSV with the column schema. Copy + fill. |
| `2026-06-validation.md` | (TBD) Generated accuracy report; lands when the study runs. |

## Why this matters for the thesis

Without this study, the thesis says "we built a classifier". With it,
the thesis says "we built a classifier and validated its per-category
accuracy on 100 hand-labeled runs". That's the difference between a
demo paper and a defendable methodology contribution. Spec §9 captures
the full thesis contribution chain.

## Out of scope for MVP

- Multi-rater inter-annotator agreement (Cohen's κ): planned for the
  thesis writeup phase; needs a second labeler.
- Local-model classifier comparison (Qwen-14B vs Haiku): planned
  ablation; requires the calibration set to be locked first.
