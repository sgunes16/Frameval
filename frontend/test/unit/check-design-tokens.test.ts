import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { execSync } from 'node:child_process';
import { existsSync, writeFileSync, rmSync } from 'node:fs';
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

/**
 * Fixture paths the tests write into `frontend/src/` so the script's
 * tsxFiles walker picks them up. If a previous test run was killed
 * mid-flight (SIGKILL, OOM), these can leak and poison the "clean
 * tree" test case until manually deleted. The beforeEach/afterEach
 * hooks below scrub them both at the start and end of every test,
 * so a leak is self-healing on the next invocation.
 */
const FIXTURE_PATHS = [
  resolve(ROOT, 'src/__sweep_fixture.tsx'),
  resolve(ROOT, 'src/__sweep_comment_fixture.tsx'),
];

function scrubFixtures() {
  for (const p of FIXTURE_PATHS) {
    if (existsSync(p)) rmSync(p);
  }
}

describe('check-design-tokens script', () => {
  beforeEach(scrubFixtures);
  afterEach(scrubFixtures);

  it('exits 0 on the real source tree', () => {
    const out = execSync(`node ${SCRIPT}`, { cwd: ROOT }).toString();
    expect(out).toMatch(/design-token sweep clean/);
  });

  it('exits non-zero and reports each offender on a regression', () => {
    // Stage a fixture under src/ so the script's tsxFiles walker
    // picks it up. afterEach scrubs it even on a hard kill.
    const fixturePath = FIXTURE_PATHS[0];
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
    }
    expect(exitCode).toBe(1);
    expect(stderr).toMatch(/bg-red-500/);
    expect(stderr).toMatch(/text-amber-700/);
    expect(stderr).toMatch(/text-\[11px\]/);
  });

  it('flags bg-white / text-black raw literals', () => {
    const fixturePath = FIXTURE_PATHS[0];
    writeFileSync(
      fixturePath,
      'export const X = (<div className="bg-white text-black">x</div>);\n',
    );
    let exitCode = 0;
    let stderr = '';
    try {
      execSync(`node ${SCRIPT}`, { cwd: ROOT, stdio: ['ignore', 'pipe', 'pipe'] });
    } catch (e: unknown) {
      const err = e as { status?: number; stderr?: Buffer };
      exitCode = err.status ?? 0;
      stderr = err.stderr?.toString() ?? '';
    }
    expect(exitCode).toBe(1);
    expect(stderr).toMatch(/bg-white/);
    expect(stderr).toMatch(/text-black/);
  });

  it('does NOT treat // inside a URL string as a comment marker', () => {
    // The naive comment stripper would discard everything after the
    // `//` in "https://...", silently skipping the real offender that
    // follows it. The string-aware stripper keeps the line intact.
    const fixturePath = FIXTURE_PATHS[0];
    writeFileSync(
      fixturePath,
      'const url = "https://example.com/x"; const cls = "bg-red-500";\n',
    );
    let exitCode = 0;
    let stderr = '';
    try {
      execSync(`node ${SCRIPT}`, { cwd: ROOT, stdio: ['ignore', 'pipe', 'pipe'] });
    } catch (e: unknown) {
      const err = e as { status?: number; stderr?: Buffer };
      exitCode = err.status ?? 0;
      stderr = err.stderr?.toString() ?? '';
    }
    expect(exitCode).toBe(1);
    expect(stderr).toMatch(/bg-red-500/);
  });

  it('ignores forbidden patterns that appear only in comments', () => {
    const fixturePath = FIXTURE_PATHS[1];
    writeFileSync(
      fixturePath,
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
    }
    expect(exitCode).toBe(0);
  });
});
