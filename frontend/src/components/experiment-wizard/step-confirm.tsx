export function StepConfirm({ estimate }: { estimate?: number }) {
  return <div className="rounded-md border border-slate-200 bg-slate-50 p-4 text-sm">Estimated cost: {estimate?.toFixed(4) ?? '-'} USD</div>;
}
