import { describe, expect, it } from 'vitest';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { resolve } from 'node:path';

/**
 * Token-definition smoke tests.
 *
 * These tests don't render anything — they assert the *contract* the
 * design system spec promises: every documented token resolves both in
 * light and dark modes, lives in the canonical `tokens.css` file, and is
 * exposed to Tailwind via `tailwind.config.ts`.
 *
 * If a token is removed or renamed, every component consuming it would
 * silently fall back to Tailwind's defaults; catching that here is much
 * cheaper than discovering it in a visual regression diff after a
 * migration PR.
 */

const tokensCss = readFileSync(resolve(__dirname, '../../src/styles/tokens.css'), 'utf8');
const tailwindConfig = readFileSync(resolve(__dirname, '../../tailwind.config.ts'), 'utf8');

const requiredColorTokens = [
  // surfaces
  '--bg',
  '--bg-elev-1',
  '--bg-elev-2',
  // text
  '--fg',
  '--fg-muted',
  '--fg-subtle',
  // hairlines + focus
  '--border',
  '--border-strong',
  // semantic
  '--accent',
  '--success',
  '--warning',
  '--danger',
  '--info',
  // monospace + diff
  '--code-bg',
  '--diff-add',
  '--diff-del',
  '--diff-add-text',
  '--diff-del-text',
];

const requiredChartTokens = Array.from({ length: 8 }, (_, i) => `--chart-${i + 1}`);

describe('design tokens', () => {
  it('defines every color token in :root (light mode default)', () => {
    const rootBlockMatch = tokensCss.match(/:root\s*{([\s\S]*?)}/);
    expect(rootBlockMatch, 'tokens.css must contain a :root block').not.toBeNull();
    const rootBlock = rootBlockMatch![1];
    for (const token of requiredColorTokens) {
      expect(rootBlock).toContain(`${token}:`);
    }
  });

  it('defines every color token in .dark (dark mode override)', () => {
    const darkBlockMatch = tokensCss.match(/\.dark\s*{([\s\S]*?)}/);
    expect(darkBlockMatch, 'tokens.css must contain a .dark block').not.toBeNull();
    const darkBlock = darkBlockMatch![1];
    for (const token of requiredColorTokens) {
      expect(darkBlock).toContain(`${token}:`);
    }
  });

  it('defines all 8 chart tokens in both themes', () => {
    const rootBlock = tokensCss.match(/:root\s*{([\s\S]*?)}/)![1];
    const darkBlock = tokensCss.match(/\.dark\s*{([\s\S]*?)}/)![1];
    for (const token of requiredChartTokens) {
      expect(rootBlock, `light mode ${token}`).toContain(`${token}:`);
      expect(darkBlock, `dark mode ${token}`).toContain(`${token}:`);
    }
  });

  it('exposes tokens to Tailwind via hsl(var(--*)) bindings', () => {
    // The Tailwind config must read the CSS variables so utility classes
    // like bg-elev-1, text-fg-muted, border-strong all resolve to the
    // right color in whichever theme is active.
    for (const token of ['--bg', '--bg-elev-1', '--fg', '--fg-muted', '--border', '--accent']) {
      expect(tailwindConfig).toContain(`var(${token})`);
    }
  });

  it('declares dark mode in class-based form', () => {
    // class-based dark mode is required by Story #71 (theme toggle).
    // Accepts either `darkMode: 'class'` (string) or `darkMode: ['class']`
    // (array) — Tailwind 3.x docs document both as equivalent.
    expect(tailwindConfig).toMatch(/darkMode\s*:\s*(?:\[\s*)?['"]class['"]/);
  });
});

/**
 * Source-tree sweep: the spec for Story #74 forbids ad-hoc Tailwind
 * color literals (slate/white/emerald/amber/red/etc.) and arbitrary
 * pixel sizes (text-[10px], text-[11px]). These tests scan every
 * tsx file under src/ to catch regressions before they land. The
 * lint rule planned for the second slice of #74 will eventually
 * enforce the same — these tests are the floor.
 */

const FORBIDDEN_COLOR_PATTERN =
  /(bg|text|border)-(emerald|amber|red|green|yellow|orange|blue|purple|pink|indigo|cyan|teal|rose|sky|fuchsia|violet|lime|stone|zinc|neutral|gray|slate)-\d+/;
const FORBIDDEN_PX_PATTERN = /text-\[1[01]px\]/;

function tsxFiles(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir)) {
    const full = resolve(dir, entry);
    if (statSync(full).isDirectory()) {
      out.push(...tsxFiles(full));
    } else if (entry.endsWith('.tsx')) {
      out.push(full);
    }
  }
  return out;
}

describe('design-token source sweep', () => {
  const files = tsxFiles(resolve(__dirname, '../../src'));

  it('has no hardcoded Tailwind color-scale literals in component source', () => {
    const offenders: string[] = [];
    for (const file of files) {
      const text = readFileSync(file, 'utf8');
      if (FORBIDDEN_COLOR_PATTERN.test(text)) offenders.push(file);
    }
    expect(offenders).toEqual([]);
  });

  it('has no text-[10px] / text-[11px] arbitrary pixel sizes', () => {
    const offenders: string[] = [];
    for (const file of files) {
      const text = readFileSync(file, 'utf8');
      if (FORBIDDEN_PX_PATTERN.test(text)) offenders.push(file);
    }
    expect(offenders).toEqual([]);
  });
});
