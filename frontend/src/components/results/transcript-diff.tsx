export function TranscriptDiff({ before, after, beforeLabel = 'Before', afterLabel = 'After' }: { before: string; after: string; beforeLabel?: string; afterLabel?: string }) {
  const left = formatTranscript(before);
  const right = formatTranscript(after);

  if (!left && !right) {
    return <div className="rounded-md border border-dashed border-slate-300 px-4 py-8 text-sm text-slate-500">Gosterilecek transcript bulunamadi.</div>;
  }

  return <div className="grid gap-3 md:grid-cols-2"><div><div className="mb-2 text-sm font-medium text-slate-600">{beforeLabel}</div><pre className="rounded-md bg-slate-950 p-4 text-xs text-slate-100 whitespace-pre-wrap break-words">{left}</pre></div><div><div className="mb-2 text-sm font-medium text-slate-600">{afterLabel}</div><pre className="rounded-md bg-slate-900 p-4 text-xs text-slate-100 whitespace-pre-wrap break-words">{right}</pre></div></div>;
}

function formatTranscript(raw: string): string {
  if (!raw.trim()) {
    return '';
  }

  try {
    const parsed = JSON.parse(raw) as { result?: string };
    if (typeof parsed.result === 'string' && parsed.result.trim()) {
      return parsed.result;
    }
    return JSON.stringify(parsed, null, 2);
  } catch {
    return raw;
  }
}
