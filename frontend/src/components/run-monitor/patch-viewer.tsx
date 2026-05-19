import { useEffect, useMemo, useState } from 'react';
import type { Transcript } from '../../lib/types';

type PatchSection = {
  file: string;
  diff: string;
};

export function PatchViewer({ transcript, runLabel }: { transcript?: Transcript; runLabel: string }) {
  const sections = useMemo(() => parsePatchSections(transcript?.patch ?? ''), [transcript?.patch]);
  const [selectedFile, setSelectedFile] = useState<string>('');

  useEffect(() => {
    if (!sections.length) {
      setSelectedFile('');
      return;
    }
    if (!sections.some((section) => section.file === selectedFile)) {
      setSelectedFile(sections[0].file);
    }
  }, [sections, selectedFile]);

  const selectedSection = sections.find((section) => section.file === selectedFile) ?? sections[0];

  if (!transcript) {
    return (
      <div className="rounded-lg border border-dashed border-border p-4 text-sm text-fg-muted">
        Select a completed run to view its patch.
      </div>
    );
  }

  if (!sections.length) {
    return (
      <div className="space-y-3">
        <div className="rounded-lg border border-dashed border-border p-4 text-sm text-fg-muted">
          No patch recorded for {runLabel}.
        </div>
        {transcript.filesystem_diff && (
          <pre className="max-h-60 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-code-bg p-4 font-mono text-xs leading-5 text-fg-subtle">
            {transcript.filesystem_diff}
          </pre>
        )}
      </div>
    );
  }

  return (
    <div className="grid gap-3 lg:grid-cols-[260px_minmax(0,1fr)]">
      <div className="space-y-2">
        <div className="rounded-lg border border-border bg-bg-elev-2 px-3 py-2 text-xs text-fg-muted">
          {sections.length} file changed
        </div>
        <div className="max-h-[420px] overflow-auto rounded-lg border border-border bg-bg-elev-1 p-2">
          {sections.map((section) => (
            <button
              key={section.file}
              onClick={() => setSelectedFile(section.file)}
              className={`mb-1 block w-full rounded-md px-3 py-2 text-left font-mono text-xs transition ${
                selectedSection?.file === section.file ? 'bg-fg text-bg' : 'text-fg-muted hover:bg-bg-elev-2'
              }`}
            >
              {section.file}
            </button>
          ))}
        </div>
      </div>
      <pre className="max-h-[520px] overflow-auto whitespace-pre-wrap break-words rounded-lg bg-code-bg p-4 font-mono text-xs leading-5 text-fg-subtle">
        {selectedSection?.diff ?? ''}
      </pre>
    </div>
  );
}

function parsePatchSections(patch: string): PatchSection[] {
  const trimmed = patch.trim();
  if (!trimmed) return [];

  const normalized = trimmed.replace(/\r\n/g, '\n');
  const parts = normalized.split(/^diff --git /m).filter(Boolean);
  return parts.map((part) => {
    const body = `diff --git ${part}`.trim();
    const firstLine = body.split('\n', 1)[0] ?? '';
    const match = firstLine.match(/^diff --git a\/(.+?) b\/(.+)$/);
    return {
      file: match?.[2] ?? firstLine.replace(/^diff --git /, ''),
      diff: body,
    };
  });
}
