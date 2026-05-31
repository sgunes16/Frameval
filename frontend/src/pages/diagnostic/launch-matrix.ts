/**
 * Pure expansion of the launcher's variant matrix into experiment cells.
 *
 * Frameval distinguishes two axes:
 *   - Across experiments: task × executor × model × speckit-extension.
 *     Each cell becomes one experiment. Multiple cells form a batch.
 *   - Within an experiment: harnesses become variants of that single
 *     experiment so the Compare view can score them side-by-side.
 *
 * The speckit-extension axis collapses to a single empty-string entry
 * when the user hasn't selected the speckit harness or hasn't picked
 * any extensions — so non-speckit selections keep producing the same
 * cell counts they did before this axis existed.
 *
 * The launcher uses this expansion to decide how many `/diagnostic/launch`
 * calls to fire and what batch identity to share across them.
 */

export interface LaunchCell {
  taskId: string;
  executorId: string;
  modelId: string;
  speckitExtension: string;
}

export interface ExpansionInput {
  taskIds: string[];
  executorIds: string[];
  modelIds: string[];
  speckitExtensions: string[];
}

/** Number of experiments the current selection will produce. */
export function countExperiments(input: ExpansionInput): number {
  return Math.max(input.taskIds.length, 1)
    * Math.max(input.executorIds.length, 1)
    * Math.max(input.modelIds.length, 1)
    * Math.max(input.speckitExtensions.length, 1);
}

/**
 * Expand the (task × executor × model × speckit-extension) cross-product
 * into one cell per experiment. Order is stable: tasks outermost,
 * speckit-extensions innermost — matches the order the variant preview
 * list already uses.
 */
export function expandLaunchMatrix(input: ExpansionInput): LaunchCell[] {
  const out: LaunchCell[] = [];
  for (const taskId of input.taskIds) {
    for (const executorId of input.executorIds) {
      for (const modelId of input.modelIds) {
        for (const speckitExtension of input.speckitExtensions) {
          out.push({ taskId, executorId, modelId, speckitExtension });
        }
      }
    }
  }
  return out;
}
