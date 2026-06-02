import type { LaunchVariant } from '../../lib/types';

/**
 * Build the intra-experiment variant list from the launcher's harness +
 * spec-kit-extension selection.
 *
 * Each non-speckit harness becomes exactly one variant carrying only its
 * own config block. The spec-kit harness, when selected, fans out into
 * one variant per chosen extension (named `speckit/<ext>`), each carrying
 * `{ speckit: { extension_id } }`. We do NOT cross non-speckit harnesses
 * with extensions — extensions are variants of the spec-kit harness only.
 *
 * Variants appear in `selectedHarnesses` order; spec-kit's slot expands
 * in place into its extension variants.
 */
export function buildLaunchVariants(
  selectedHarnesses: string[],
  harnessConfigs: Record<string, unknown>,
  speckitExtensions: string[],
): LaunchVariant[] {
  const out: LaunchVariant[] = [];
  for (const h of selectedHarnesses) {
    if (h === 'speckit') {
      for (const ext of speckitExtensions) {
        out.push({
          harness_id: 'speckit',
          name: `speckit/${ext}`,
          harness_config: { speckit: { extension_id: ext } },
        });
      }
      continue;
    }
    const cfg = configForHarness(h, harnessConfigs);
    out.push({ harness_id: h, name: h, harness_config: cfg });
  }
  return out;
}

// Return only the harness's own config block, or undefined when the
// harness needs none (bare, ralph) or the user hasn't supplied it yet.
function configForHarness(
  harnessId: string,
  harnessConfigs: Record<string, unknown>,
): Record<string, unknown> | undefined {
  const block = harnessConfigs[harnessId];
  if (block === undefined || block === null) return undefined;
  return { [harnessId]: block };
}
