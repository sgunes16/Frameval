import { cn } from '../../lib/utils';

/**
 * ErrorState is the canonical "something broke" panel rendered when a
 * TanStack Query throws, when a route's errorElement fires, or when an
 * explicit handler decides to surface failure to the user.
 *
 * The retry button is opt-in via onRetry — many error contexts have no
 * retryable action (a 4xx validation failure, for example) and would
 * mislead the user by offering one.
 */

interface ErrorStateProps {
  title: string;
  description?: string;
  onRetry?: () => void;
  className?: string;
}

export function ErrorState({ title, description, onRetry, className }: ErrorStateProps) {
  return (
    <div
      role="alert"
      className={cn(
        'flex flex-col items-center justify-center gap-3 rounded-lg border border-danger/30 bg-bg-elev-1 px-6 py-8 text-center',
        className,
      )}
    >
      <div className="text-md font-semibold text-danger">{title}</div>
      {description && <div className="max-w-md text-sm text-fg-muted">{description}</div>}
      {onRetry && (
        <button
          type="button"
          onClick={onRetry}
          className="inline-flex items-center gap-2 rounded-md border border-border-strong bg-bg-elev-2 px-3 py-1.5 text-xs font-medium text-fg transition hover:bg-bg-elev-1"
        >
          Try again
        </button>
      )}
    </div>
  );
}
