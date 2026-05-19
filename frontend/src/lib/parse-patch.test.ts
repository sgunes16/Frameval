import { describe, expect, it } from 'vitest';

import { parsePatch, type FileDiff } from './parse-patch';

const samplePatch = `diff --git a/src/main.go b/src/main.go
index abc..def 100644
--- a/src/main.go
+++ b/src/main.go
@@ -1,4 +1,5 @@
 package main

-import "fmt"
+import "log"
+import "os"

diff --git a/README.md b/README.md
new file mode 100644
--- /dev/null
+++ b/README.md
@@ -0,0 +1,2 @@
+# Project
+New file
`;

describe('parsePatch', () => {
  it('returns an empty map for an empty input', () => {
    expect(parsePatch('').size).toBe(0);
  });

  it('parses one entry per file', () => {
    const result = parsePatch(samplePatch);
    expect(result.size).toBe(2);
    expect(result.has('src/main.go')).toBe(true);
    expect(result.has('README.md')).toBe(true);
  });

  it('counts added vs removed lines', () => {
    const result = parsePatch(samplePatch);
    const main = result.get('src/main.go') as FileDiff;
    expect(main.added).toBe(2);
    expect(main.removed).toBe(1);

    const readme = result.get('README.md') as FileDiff;
    expect(readme.added).toBe(2);
    expect(readme.removed).toBe(0);
  });

  it('skips diff metadata lines from line counts (---, +++)', () => {
    // The header has --- and +++ which start with - and + but must NOT
    // be counted as removed/added content lines.
    const result = parsePatch(samplePatch);
    const main = result.get('src/main.go') as FileDiff;
    // 2 real additions (log, os), 1 real removal (fmt) — verifying
    // that the header didn't sneak into the totals.
    expect(main.added + main.removed).toBe(3);
  });

  it('preserves the file’s raw hunks for downstream renderers', () => {
    const result = parsePatch(samplePatch);
    const main = result.get('src/main.go') as FileDiff;
    expect(main.hunks).toContain('@@ -1,4 +1,5 @@');
    expect(main.hunks).toContain('+import "log"');
  });

  it('handles a single-file diff with no diff --git header gracefully', () => {
    // Some unified-diff producers emit just --- / +++ without the
    // top-level 'diff --git' line. parser should still find the path.
    const raw = `--- a/foo.txt
+++ b/foo.txt
@@ -1 +1,2 @@
 line
+added
`;
    const result = parsePatch(raw);
    expect(result.has('foo.txt')).toBe(true);
    expect(result.get('foo.txt')?.added).toBe(1);
  });
});
