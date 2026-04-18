import { useEffect, useRef } from 'react';

export function LogViewer({ lines }: { lines: string[] }) {
  const ref = useRef<HTMLPreElement | null>(null);

  useEffect(() => {
    if (ref.current) {
      ref.current.scrollTop = ref.current.scrollHeight;
    }
  }, [lines]);

  return (
    <pre ref={ref} className="min-h-48 max-h-[360px] overflow-auto rounded-lg bg-slate-950 p-4 font-mono text-[11px] leading-5 text-slate-100">
      {lines.length ? lines.join('\n') : 'Waiting for logs...'}
    </pre>
  );
}
