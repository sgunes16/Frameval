import { useEffect, useRef } from 'react';

type CheckboxProps = React.InputHTMLAttributes<HTMLInputElement> & {
  /** Renders the native indeterminate (dash) state; ignored when `checked`. */
  indeterminate?: boolean;
};

/**
 * Checkbox is a thin styled wrapper over the native input so we get
 * indeterminate support (used by the experiments list "select all" header)
 * without pulling in another Radix dependency.
 */
export function Checkbox({ indeterminate = false, className = '', ...props }: CheckboxProps) {
  const ref = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (ref.current) ref.current.indeterminate = indeterminate;
  }, [indeterminate]);
  return (
    <input
      type="checkbox"
      ref={ref}
      className={`h-4 w-4 cursor-pointer rounded border border-border bg-bg-elev-1 accent-fg ${className}`}
      {...props}
    />
  );
}
