import type { PropsWithChildren } from 'react';
import { cn } from '../../lib/utils';

/**
 * Kbd is a keyboard-shortcut pill ("⌘K", "Esc", "Enter"). Used in the
 * Cmd-K palette (#73), tooltips on action buttons, and the Inspector
 * V2 turn-navigation hints.
 *
 * Renders as a semantic <kbd> element so screen readers announce it as
 * a keyboard input. The styling is a hairline-bordered pill in the
 * monospace family so the visual matches the OS-level rendering of a
 * key on a real keyboard.
 */

export function Kbd({ children, className }: PropsWithChildren<{ className?: string }>) {
  return (
    <kbd
      className={cn(
        'inline-flex items-center justify-center rounded-sm border border-border bg-bg-elev-2 px-1.5 py-0.5 font-mono text-[11px] font-medium text-fg-muted',
        className,
      )}
    >
      {children}
    </kbd>
  );
}
