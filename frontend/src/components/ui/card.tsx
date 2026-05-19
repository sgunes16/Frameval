import { HTMLAttributes, PropsWithChildren } from 'react';
import { cn } from '../../lib/utils';

type CardProps = PropsWithChildren<HTMLAttributes<HTMLDivElement>> & {
  padded?: boolean;
  hoverable?: boolean;
};

export function Card({ children, className, padded = true, hoverable = false, ...rest }: CardProps) {
  return (
    <div
      className={cn(
        'rounded-xl border border-border bg-bg-elev-1/90 shadow-sm',
        padded && 'p-5',
        hoverable && 'transition hover:border-border-strong hover:shadow-md',
        className,
      )}
      {...rest}
    >
      {children}
    </div>
  );
}

export function CardHeader({ title, description, action }: { title: string; description?: string; action?: React.ReactNode }) {
  return (
    <div className="mb-4 flex items-start justify-between gap-4">
      <div>
        <div className="text-sm font-semibold tracking-tight text-fg">{title}</div>
        {description && <div className="mt-1 text-xs text-fg-muted">{description}</div>}
      </div>
      {action}
    </div>
  );
}
