import type { FailureCode } from './types';

/**
 * Human-readable descriptions for FailureCode enum values.
 *
 * Mirrors grader/failure_classifier/taxonomy.py:FAILURE_DESCRIPTIONS.
 * Keep in sync — when adding/changing a code there, update here too.
 */
export const FAILURE_DESCRIPTIONS: Record<FailureCode, string> = {
  NONE: 'No failure detected.',
  HAL_API: 'Hallucinated API — used a function/method/parameter that does not exist.',
  HAL_FILE: 'Phantom file — referenced a file that was never created or wrong location.',
  DEP_MISS: 'Missing dependency — used package without installing or declaring it.',
  STOP_EARLY: 'Premature completion — declared task done while tests still failing.',
  STOP_GIVEUP: 'Surrender — declared inability to proceed without exhausting options.',
  LOOP_INF: 'Infinite loop / no progress — repeated same action with no state change.',
  WRONG_ABS: "Wrong abstraction — solution structure doesn't match task (sync vs async).",
  MISREAD: 'Spec misread — solution targets wrong requirement (broke contract).',
  ENV_ERR: 'Environment failure — failure caused by sandbox/tool, not the agent.',
  SCOPE_DRIFT: 'Scope drift — modified files outside expected scope for brownfield task.',
  TIMEOUT: 'Wall-clock timeout — run exceeded time budget before completion.',
  SILENT_SKIP: 'Silent failure — agent encountered error and ignored it in subsequent turns.',
};
