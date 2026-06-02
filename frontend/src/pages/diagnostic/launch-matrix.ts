/**
 * Pure expansion of the launcher's cross-experiment matrix into cells.
 *
 * Cross-experiment axes (each combination → one experiment, batched):
 *   task × executor × model
 *
 * Harnesses and spec-kit extensions are NOT axes here — they are
 * intra-experiment variants (see build-variants.ts). Every cell produces
 * one experiment carrying the same variant list.
 */

export interface LaunchCell {
  taskId: string;
  executorId: string;
  modelId: string;
}

export interface ExpansionInput {
  taskIds: string[];
  executorIds: string[];
  modelIds: string[];
}

/** Number of experiments the current selection will produce. */
export function countExperiments(input: ExpansionInput): number {
  return Math.max(input.taskIds.length, 1)
    * Math.max(input.executorIds.length, 1)
    * Math.max(input.modelIds.length, 1);
}

/**
 * Expand the (task × executor × model) cross-product into one cell per
 * experiment. Order is stable: tasks outermost, models innermost.
 */
export function expandLaunchMatrix(input: ExpansionInput): LaunchCell[] {
  const out: LaunchCell[] = [];
  for (const taskId of input.taskIds) {
    for (const executorId of input.executorIds) {
      for (const modelId of input.modelIds) {
        out.push({ taskId, executorId, modelId });
      }
    }
  }
  return out;
}
