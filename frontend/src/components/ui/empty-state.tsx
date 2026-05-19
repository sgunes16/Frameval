import { ReactNode } from 'react';

export function EmptyState({ title, description, action }: { title: string; description?: string; action?: ReactNode }) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 rounded-xl border border-dashed border-border bg-bg-elev-1/70 px-6 py-10 text-center">
      <div className="text-sm font-semibold text-fg">{title}</div>
      {description && <div className="max-w-md text-xs text-fg-muted">{description}</div>}
      {action}
    </div>
  );
}
