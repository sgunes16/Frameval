import type { MultiAgentConfig } from '../../lib/types';

const NAME_PATTERN = /^[a-z][a-z0-9_]*$/;
const MIN_ROLES = 1;
const MAX_ROLES = 5;

/**
 * True when the supplied config matches every backend constraint:
 * - between 1 and 5 roles
 * - every name is snake_case ASCII (`^[a-z][a-z0-9_]*$`)
 * - every prompt is non-empty after trimming
 * - all names are unique
 *
 * Pure function — no DOM, no React, no side effects. The launcher
 * uses it as the submit gate; mirror the same validation rules the
 * backend's extractRoles helper enforces so the UI never lets users
 * submit configs the harness would reject.
 */
export function validateMultiAgentConfig(value: MultiAgentConfig | undefined): boolean {
  if (!value || !Array.isArray(value.roles)) return false;
  const { roles } = value;
  if (roles.length < MIN_ROLES || roles.length > MAX_ROLES) return false;
  const seen = new Set<string>();
  for (const r of roles) {
    if (!r || typeof r.name !== 'string' || typeof r.prompt !== 'string') return false;
    if (!NAME_PATTERN.test(r.name)) return false;
    if (r.prompt.trim().length === 0) return false;
    if (seen.has(r.name)) return false;
    seen.add(r.name);
  }
  return true;
}
