import { useMemo } from 'react';

import { alignAnchors, type AlignmentRow, type RunAnchors } from './anchor-alignment';

/**
 * Thin React hook over `alignAnchors`. Memoised on the input ref so
 * callers passing a stable array (e.g. from `useQuery({select: ...})`)
 * don't re-walk the alignment on every render.
 *
 * The hook intentionally does NOT memo-deep-equal — that's the
 * caller's job. Identity-based memoisation matches how the engine's
 * AnchorBundle is fetched (a single React Query cache entry whose
 * object identity changes only on refetch).
 */
export function useAnchorAlignment(runs: RunAnchors[]): AlignmentRow[] {
  return useMemo(() => alignAnchors(runs), [runs]);
}
