export function ArtifactDiff({ diff }: { diff: string }) {
  return <pre className="overflow-auto rounded-md bg-slate-950 p-4 text-xs text-slate-100">{diff || 'No diff yet.'}</pre>;
}
