import { ButtonHTMLAttributes, forwardRef } from 'react';
import { cn } from '../../lib/utils';

type Variant = 'primary' | 'secondary' | 'outline' | 'ghost' | 'danger';
type Size = 'sm' | 'md' | 'lg';

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant;
  size?: Size;
};

const variantStyles: Record<Variant, string> = {
  primary: 'bg-fg text-bg hover:bg-fg border border-transparent',
  secondary: 'bg-bg-elev-2 text-fg hover:bg-bg-elev-2 border border-transparent',
  outline: 'bg-bg-elev-1 text-fg border border-border-strong hover:border-border-strong',
  ghost: 'bg-transparent text-fg hover:bg-bg-elev-2 border border-transparent',
  danger: 'bg-danger text-bg hover:bg-danger/90 border border-transparent',
};

const sizeStyles: Record<Size, string> = {
  sm: 'h-8 px-3 text-xs',
  md: 'h-9 px-4 text-sm',
  lg: 'h-10 px-5 text-sm',
};

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { className, variant = 'primary', size = 'md', ...props },
  ref,
) {
  return (
    <button
      ref={ref}
      className={cn(
        'inline-flex items-center justify-center gap-2 rounded-lg font-medium transition focus:outline-none focus:ring-2 focus:ring-fg/10 disabled:cursor-not-allowed disabled:opacity-50',
        variantStyles[variant],
        sizeStyles[size],
        className,
      )}
      {...props}
    />
  );
});
