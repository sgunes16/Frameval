/**
 * parse-patch — a small unified-diff parser tuned for the engine's
 * transcript.patch field. Inspector V2 uses it to compute a per-turn
 * diff by filtering on the files a turn touched (see use-per-turn-diff).
 *
 * Why a custom parser rather than pulling in `diff` or `parse-diff`:
 * the bundle is already pushing the 500kB chunk-size warning; this
 * file replaces 30kB of upstream dep with ~80 lines tuned for the
 * shapes git produces.
 *
 * Shape supported:
 *   - `diff --git a/foo b/foo` block headers (the common case)
 *   - bare `--- a/foo` / `+++ b/foo` headers (some producers skip
 *     the `diff --git` line)
 *   - `--- /dev/null` for newly-created files
 *   - `+++ /dev/null` for deleted files
 *   - Standard `@@` hunk headers
 *
 * Line counts exclude the `---` / `+++` file-header lines themselves;
 * only real `+`/`-` content lines inside hunks are tallied.
 */

export interface FileDiff {
  path: string;
  added: number;
  removed: number;
  /**
   * The raw text of the file's hunks (everything from the first `@@`
   * line through the end of this file's section). Useful for the
   * diff viewer to render full hunks without re-parsing.
   */
  hunks: string;
}

export function parsePatch(raw: string): Map<string, FileDiff> {
  const out = new Map<string, FileDiff>();
  if (!raw) return out;

  let current: FileDiff | null = null;
  // hunkLines collects the raw hunk text per file so we can attach it
  // when we finalise the entry.
  let hunkLines: string[] = [];
  // Most recent `--- a/<path>` we saw. Used as a fallback when the
  // matching `+++ /dev/null` line tells us this is a deletion and
  // the b-side has no path of its own.
  let pendingSourcePath: string | null = null;

  const finalize = () => {
    if (!current) return;
    current.hunks = hunkLines.join('\n');
    out.set(current.path, current);
    current = null;
    hunkLines = [];
  };

  const lines = raw.split('\n');
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i] ?? '';

    // New-file marker: `diff --git a/<path> b/<path>` — start a fresh entry.
    // Path resolution defers to the `+++` line because `diff --git` is
    // optional and we want one code path for both.
    if (line.startsWith('diff --git')) {
      finalize();
      pendingSourcePath = null;
      continue;
    }

    if (line.startsWith('--- ')) {
      // Source-side path; remembered so a `+++ /dev/null` (deletion)
      // can still produce a FileDiff entry keyed by the deleted path.
      pendingSourcePath = stripPrefix(line.slice(4));
      continue;
    }

    // The `+++` line carries the post-image path. For deletions
    // (`+++ /dev/null`) we fall back to the `--- a/<path>` we just
    // saw; otherwise this is an add/modify and we use the b-side path.
    if (line.startsWith('+++ ')) {
      let path = stripPrefix(line.slice(4));
      if (path === null) {
        // Deletion — recover the path from the matching `---` line.
        if (!pendingSourcePath) {
          pendingSourcePath = null;
          continue;
        }
        path = pendingSourcePath;
      }
      if (current && current.path !== path) finalize();
      current = current ?? { path, added: 0, removed: 0, hunks: '' };
      current.path = path;
      pendingSourcePath = null;
      continue;
    }

    if (line.startsWith('@@')) {
      hunkLines.push(line);
      continue;
    }

    // Content lines inside a hunk: + adds, - removes, anything else
    // (' ', '\\') is context and ignored for counts.
    if (current && (line.startsWith('+') || line.startsWith('-'))) {
      // The +++ and --- lines above are caught earlier so we don't
      // need to re-check the prefix length here.
      if (line.startsWith('+')) current.added += 1;
      if (line.startsWith('-')) current.removed += 1;
      hunkLines.push(line);
      continue;
    }

    if (current) {
      // Context lines join the hunk text but don't change counts.
      hunkLines.push(line);
    }
  }

  finalize();
  return out;
}

/**
 * Strip git's `a/` or `b/` prefix from a diff header path. Returns
 * null for the `/dev/null` sentinel so callers can detect new/deleted
 * files without scraping the magic string themselves.
 */
function stripPrefix(s: string): string | null {
  const trimmed = s.trim();
  if (trimmed === '/dev/null') return null;
  if (trimmed.startsWith('a/') || trimmed.startsWith('b/')) {
    return trimmed.slice(2);
  }
  return trimmed;
}
