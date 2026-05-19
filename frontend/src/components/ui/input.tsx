import { InputHTMLAttributes, forwardRef } from 'react';
import { cn } from '../../lib/utils';

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(function Input(props, ref) {
  return (
    <input
      ref={ref}
      {...props}
      className={cn(
        'h-9 w-full rounded-lg border border-border-strong bg-bg-elev-1 px-3 text-sm text-fg shadow-sm placeholder:text-fg-subtle focus:border-border-strong focus:outline-none focus:ring-2 focus:ring-fg/10 disabled:opacity-50',
        props.className,
      )}
    />
  );
});
