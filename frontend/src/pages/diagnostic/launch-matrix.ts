/**
 * Pure expansion of the launcher's variant matrix into experiment cells.
 *
 * Frameval distinguishes two axes:
 *   - Across experiments: task × executor × model. Each cell becomes one
 *     experiment. Multiple cells form a batch.
 *   - Within an experiment: harnesses become variants of that single
 *     experiment so the Compare view can score them side-by-side.
 *
 * So a 1-task × 1-harness × 1-exec × 2-model selection produces 2
 * experiments (one per model), each holding 1 variant (the harness),
 * grouped under one batch.
 *
 * The launcher uses this expansion to decide how many `/diagnostic/launch`
 * calls to fire and what batch identity to share across them.
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
 * experiment. Order is stable: tasks outermost, executors middle, models
 * innermost — matches the order the variant preview list already uses.
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
