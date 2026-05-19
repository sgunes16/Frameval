import { cn } from '../../lib/utils';

/**
 * LoadingSkeleton renders a shimmer placeholder while data is loading.
 * Three variants cover the dominant container shapes in the codebase:
 *
 *   - "card"  — a card-height block, used inside Card containers
 *   - "row"   — a single horizontal bar; pass `count` to stack N rows
 *               for table-like loading states
 *   - "block" — a square block, used for media previews and the
 *               diagnostic-summary grid
 */

interface LoadingSkeletonProps {
  variant: 'card' | 'row' | 'block';
  count?: number;
  className?: string;
}

export function LoadingSkeleton({ variant, count = 1, className }: LoadingSkeletonProps) {
  if (variant === 'row') {
    return (
      <div className={cn('flex flex-col gap-2', className)}>
        {Array.from({ length: count }, (_, i) => (
          <div
            key={i}
            data-testid="skeleton-row"
            className="h-3 w-full animate-pulse rounded-sm bg-bg-elev-2"
          />
        ))}
      </div>
    );
  }
  if (variant === 'block') {
    return (
      <div
        data-testid="skeleton-block"
        className={cn('h-24 w-full animate-pulse rounded-md bg-bg-elev-2', className)}
      />
    );
  }
  // card variant
  return (
    <div
      data-testid="skeleton-card"
      className={cn('h-40 w-full animate-pulse rounded-lg bg-bg-elev-2', className)}
    />
  );
}
