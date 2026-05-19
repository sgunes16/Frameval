export function ArtifactDiff({ diff }: { diff: string }) {
  return <pre className="overflow-auto rounded-md bg-code-bg p-4 text-xs text-fg-subtle">{diff || 'No diff yet.'}</pre>;
}
