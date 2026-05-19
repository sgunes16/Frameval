import { describe, expect, it } from 'vitest';
import { execSync } from 'node:child_process';
import { writeFileSync, rmSync } from 'node:fs';
import { resolve } from 'node:path';

/**
 * Integration test for the standalone design-token sweep script
 * (frontend/scripts/check-design-tokens.mjs). Verifies the script
 * exits 0 on a clean source tree and exits 1 on injected offenders.
 *
 * Why exec rather than import: the script is intentionally an .mjs
 * Node script (not a module the test imports). Running it as a child
 * process is the same surface CI sees.
 */

const ROOT = resolve(__dirname, '../..');
const SCRIPT = resolve(ROOT, 'scripts/check-design-tokens.mjs');

describe('check-design-tokens script', () => {
  it('exits 0 on the real source tree', () => {
    const out = execSync(`node ${SCRIPT}`, { cwd: ROOT }).toString();
    expect(out).toMatch(/design-token sweep clean/);
  });

  it('exits non-zero and reports each offender on a regression', () => {
    // Stage a fixture under src/ so the script's tsxFiles walker
    // picks it up. The walker resolves paths from frontend/src so we
    // can't run the script against a fully different directory
    // without also rewriting it — easier to inject + clean up.
    const fixturePath = resolve(ROOT, 'src/__sweep_fixture.tsx');
    writeFileSync(
      fixturePath,
      'export const X = (<div className="bg-red-500 text-amber-700 text-[11px]">x</div>);\n',
    );
    let stderr = '';
    let exitCode = 0;
    try {
      execSync(`node ${SCRIPT}`, { cwd: ROOT, stdio: ['ignore', 'pipe', 'pipe'] });
    } catch (e: unknown) {
      const err = e as { status?: number; stderr?: Buffer };
      exitCode = err.status ?? 0;
      stderr = err.stderr?.toString() ?? '';
    } finally {
      rmSync(fixturePath);
    }
    expect(exitCode).toBe(1);
    expect(stderr).toMatch(/bg-red-500/);
    expect(stderr).toMatch(/text-amber-700/);
    expect(stderr).toMatch(/text-\[11px\]/);
  });

  it('ignores forbidden patterns that appear only in comments', () => {
    const fixturePath = resolve(ROOT, 'src/__sweep_comment_fixture.tsx');
    writeFileSync(
      fixturePath,
      // The two forbidden patterns appear only in comments — the
      // sweep must NOT flag them.
      [
        '// was bg-red-600 before the migration',
        '/* legacy class: text-[10px] */',
        'export const X = (<div className="bg-bg-elev-1 text-fg">x</div>);',
        '',
      ].join('\n'),
    );
    let exitCode = 0;
    try {
      execSync(`node ${SCRIPT}`, { cwd: ROOT, stdio: ['ignore', 'pipe', 'pipe'] });
    } catch (e: unknown) {
      const err = e as { status?: number };
      exitCode = err.status ?? 0;
    } finally {
      rmSync(fixturePath);
    }
    expect(exitCode).toBe(0);
  });
});
