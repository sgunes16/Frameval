/**
 * System component primitives — the token-driven building blocks the
 * rest of the app composes with. New code should import from this
 * barrel rather than reaching directly into the underlying files:
 *
 *   import { StatusDot, TokenChip, ErrorState } from '@/components/system';
 *
 * Each component reads design tokens (see src/styles/tokens.css) so a
 * future theme switch is a single class flip on <html>.
 */
export { StatusDot, type StatusVariant } from './StatusDot';
export { TokenChip } from './TokenChip';
export { FilePath } from './FilePath';
export { Kbd } from './Kbd';
export { LoadingSkeleton } from './LoadingSkeleton';
export { ErrorState } from './ErrorState';
