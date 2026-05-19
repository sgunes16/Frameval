import { cn } from '../../lib/utils';

/**
 * FilePath renders a workspace-relative file path with middle-truncation
 * when it overflows. The full path is always available via the title
 * attribute so hover discovers it. Used heavily in Inspector V2's tool
 * histogram, per-turn diff header, and Compare V2's anchor labels.
 *
 * Why middle-truncate rather than end-truncate: filename + extension is
 * the load-bearing identifier ("`...path/main.go`" tells a reader more
 * than "`src/very/long/...`"). Both ends are preserved on overflow.
 */

interface FilePathProps {
  path: string;
  maxChars?: number;
  className?: string;
}

const ELLIPSIS = '…';

export function FilePath({ path, maxChars = 48, className }: FilePathProps) {
  const display = maxChars > 0 && path.length > maxChars ? truncateMiddle(path, maxChars) : path;
  return (
    <span
      title={path}
      className={cn('inline-block font-mono text-xs text-fg-muted', className)}
    >
      {display}
    </span>
  );
}

/**
 * truncateMiddle keeps as many head + tail characters as possible while
 * staying under maxChars total (including the ellipsis). When maxChars
 * is too small to fit head + ellipsis + tail we fall back to plain
 * end-truncation so the function never returns a longer string than the
 * input (or longer than maxChars).
 */
function truncateMiddle(s: string, maxChars: number): string {
  if (s.length <= maxChars) return s;
  // Need at least 3 characters to fit one head char, ellipsis, one tail
  // char. Below that, fall back to end-truncation with an ellipsis,
  // which is the only way to honor the maxChars contract.
  if (maxChars < 3) {
    return s.slice(0, Math.max(0, maxChars - 1)) + ELLIPSIS;
  }
  const headLen = Math.max(1, Math.floor((maxChars - 1) / 2));
  const tailLen = Math.max(1, maxChars - 1 - headLen);
  return s.slice(0, headLen) + ELLIPSIS + s.slice(s.length - tailLen);
}
