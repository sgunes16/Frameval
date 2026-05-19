import { useTheme, type ThemeMode } from '../../lib/use-theme';
import { cn } from '../../lib/utils';

/**
 * ThemeToggle renders a 3-segment switch (system / light / dark)
 * lives in the page header. Compact enough to share row-space with
 * the breadcrumb and Cmd-K hint.
 *
 * Keyboard: each segment is a real <button>, so Tab + Enter cycles
 * through choices the same way the mouse path does. The active
 * choice carries aria-pressed='true' for screen-reader feedback.
 */

const options: Array<{ value: ThemeMode; label: string; glyph: string }> = [
  { value: 'system', label: 'System theme', glyph: '⌂' },
  { value: 'light', label: 'Light theme', glyph: '☀' },
  { value: 'dark', label: 'Dark theme', glyph: '☾' },
];

export function ThemeToggle() {
  const { mode, setMode } = useTheme();
  return (
    <div
      role="group"
      aria-label="Theme"
      className="inline-flex items-center rounded-md border border-border bg-bg-elev-2 p-0.5"
    >
      {options.map(({ value, label, glyph }) => {
        const active = mode === value;
        return (
          <button
            key={value}
            type="button"
            aria-pressed={active}
            aria-label={label}
            title={label}
            onClick={() => setMode(value)}
            className={cn(
              'inline-flex h-6 w-6 items-center justify-center rounded-sm text-xs transition',
              active
                ? 'bg-bg-elev-1 text-fg shadow-sm'
                : 'text-fg-muted hover:bg-bg-elev-1/60 hover:text-fg',
            )}
          >
            {glyph}
          </button>
        );
      })}
    </div>
  );
}
