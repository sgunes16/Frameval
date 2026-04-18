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
        'rounded-xl border border-slate-200/80 bg-white/90 shadow-[0_1px_2px_rgba(15,23,42,0.04)]',
        padded && 'p-5',
        hoverable && 'transition hover:border-slate-300 hover:shadow-[0_6px_16px_-8px_rgba(15,23,42,0.18)]',
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
        <div className="text-sm font-semibold tracking-tight text-slate-900">{title}</div>
        {description && <div className="mt-1 text-xs text-slate-500">{description}</div>}
      </div>
      {action}
    </div>
  );
}
