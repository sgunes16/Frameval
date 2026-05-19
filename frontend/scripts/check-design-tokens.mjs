#!/usr/bin/env node
/**
 * check-design-tokens — design-system source sweep, slice 2 of #74.
 *
 * Scans every .tsx file under frontend/src/ for two forbidden
 * patterns and exits non-zero with file:line output if anything
 * matches:
 *
 *   1. Raw Tailwind color-scale classes
 *      (bg|text|border)-(emerald|amber|red|green|yellow|orange|blue|
 *       purple|pink|indigo|cyan|teal|rose|sky|fuchsia|violet|lime|
 *       stone|zinc|neutral|gray|slate)-N
 *      The semantic tokens (success/warning/danger/info, plus their
 *      `-fg` body-copy variants) replace these.
 *
 *   2. Arbitrary pixel sizes: text-[10px] / text-[11px].
 *      Replaced by Tailwind's text-xs across the migration.
 *
 * Single-line `//` and block `/* * /` comments are stripped before
 * matching so prose mentions (e.g. "// was bg-red-600 before") don't
 * trip the check.
 *
 * Output format matches GitHub's annotation syntax — running this
 * in Actions will surface offending lines as PR annotations.
 *
 *   ::error file=src/foo.tsx,line=42::bg-emerald-500 is forbidden …
 *
 * Locally, the same lines render as plain text.
 */

import { readdirSync, readFileSync, statSync } from 'node:fs';
import { resolve, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = fileURLToPath(new URL('.', import.meta.url));
const ROOT = resolve(__dirname, '..');
const SRC = resolve(ROOT, 'src');

// Color literals: numeric-scale palette families AND `white`/`black`.
// Both bypass the semantic token system equally.
const FORBIDDEN_COLOR = new RegExp(
  '(bg|text|border)-(' +
    '(?:emerald|amber|red|green|yellow|orange|blue|' +
    'purple|pink|indigo|cyan|teal|rose|sky|fuchsia|violet|lime|' +
    'stone|zinc|neutral|gray|slate)-\\d+' +
    '|white|black' +
    ')',
  'g',
);

// All arbitrary pixel font sizes — not just 10px/11px. The semantic
// scale (text-xs / text-sm / text-base / text-lg / etc.) is the
// supported surface.
const FORBIDDEN_PX = /text-\[\d+px\]/g;

const IS_CI = process.env.CI === 'true' || process.env.GITHUB_ACTIONS === 'true';

function tsxFiles(dir) {
  const out = [];
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

function stripCommentsInPlace(lines) {
  // Strip // and /* */ comments while honouring string boundaries —
  // `//` inside a URL string ("https://...") or `/*` inside a
  // template literal must NOT count as a comment, or we'd silently
  // skip real offenders. Tracks single-quote, double-quote, and
  // backtick string state and only acts on comment markers when not
  // currently inside a string.
  let inBlock = false;
  return lines.map((line) => {
    let result = '';
    let i = 0;
    /** @type {'' | "'" | '"' | '`'} */
    let inString = '';
    while (i < line.length) {
      const ch = line[i];
      if (inBlock) {
        const end = line.indexOf('*/', i);
        if (end === -1) return result;
        i = end + 2;
        inBlock = false;
        continue;
      }
      if (inString) {
        if (ch === '\\') {
          // Preserve escape + next char untouched.
          result += line.slice(i, i + 2);
          i += 2;
          continue;
        }
        if (ch === inString) {
          inString = '';
        }
        result += ch;
        i += 1;
        continue;
      }
      // Not in a string and not in a block — comment markers count.
      if (line.startsWith('/*', i)) {
        const end = line.indexOf('*/', i + 2);
        if (end === -1) {
          inBlock = true;
          return result;
        }
        i = end + 2;
        continue;
      }
      if (line.startsWith('//', i)) {
        return result;
      }
      if (ch === "'" || ch === '"' || ch === '`') {
        inString = /** @type {"'" | '"' | '`'} */ (ch);
      }
      result += ch;
      i += 1;
    }
    return result;
  });
}

function checkFile(path) {
  const lines = readFileSync(path, 'utf8').split('\n');
  const stripped = stripCommentsInPlace(lines);
  const offenders = [];
  for (let i = 0; i < stripped.length; i++) {
    const line = stripped[i];
    // Use matchAll so multiple offenders on one line all get
    // reported, not just the first.
    for (const m of line.matchAll(FORBIDDEN_COLOR)) {
      offenders.push({ lineNumber: i + 1, match: m[0], kind: 'color' });
    }
    for (const m of line.matchAll(FORBIDDEN_PX)) {
      offenders.push({ lineNumber: i + 1, match: m[0], kind: 'px' });
    }
  }
  return offenders;
}

function formatOffender(file, offender) {
  const rel = relative(ROOT, file);
  const reason =
    offender.kind === 'color'
      ? `\`${offender.match}\` is a raw Tailwind color literal — use a semantic token (success / warning / danger / info, plus -fg variants for body copy on tinted backgrounds). See src/styles/tokens.css.`
      : `\`${offender.match}\` is an arbitrary pixel size — use text-xs or text-sm.`;
  if (IS_CI) {
    return `::error file=frontend/${rel},line=${offender.lineNumber}::${reason}`;
  }
  return `  ${rel}:${offender.lineNumber}  ${reason}`;
}

function main() {
  const files = tsxFiles(SRC);
  const offenderEntries = [];
  for (const file of files) {
    const offenders = checkFile(file);
    for (const o of offenders) {
      offenderEntries.push({ file, offender: o });
    }
  }
  if (offenderEntries.length === 0) {
    console.log(`✓ design-token sweep clean across ${files.length} .tsx files`);
    return;
  }
  console.error(
    `✗ design-token sweep found ${offenderEntries.length} offender(s):\n`,
  );
  for (const { file, offender } of offenderEntries) {
    console.error(formatOffender(file, offender));
  }
  console.error(
    '\nFix by replacing raw color scales with semantic tokens, or by switching arbitrary pixel sizes to text-xs / text-sm. See docs/superpowers/specs/2026-05-14-design-system-v2.md.',
  );
  process.exit(1);
}

main();
