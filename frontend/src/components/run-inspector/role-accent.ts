/**
 * roleAccent maps a role name to a stable Tailwind left-border class.
 * Same role → same color across all runs and pages. The palette reuses
 * existing semantic tokens with /50 alpha so it sits naturally next to
 * the rest of the design language; no new tokens are introduced.
 *
 * Empty / undefined → the neutral default border class (the absence of
 * a role should look like no accent at all).
 */

const PALETTE: readonly string[] = [
  'border-l-info-fg/50',
  'border-l-success-fg/50',
  'border-l-warning-fg/50',
  'border-l-fg-subtle/50',
  'border-l-fg-muted/50',
];

export function roleAccent(role: string | undefined): string {
  if (!role) return 'border-l-border';
  let h = 0;
  for (let i = 0; i < role.length; i++) {
    h = (h * 31 + role.charCodeAt(i)) >>> 0;
  }
  return PALETTE[h % PALETTE.length];
}
