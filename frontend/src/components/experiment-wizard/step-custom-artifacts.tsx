import { Button } from '../ui/button';
import { Input } from '../ui/input';

export type DraftArtifactFile = {
  id: string;
  artifact_type: string;
  file_path: string;
  content: string;
};

const ARTIFACT_PRESETS: Array<{ label: string; file_path: string; artifact_type: string }> = [
  { label: 'AGENTS.md', file_path: 'AGENTS.md', artifact_type: 'agents_md' },
  { label: 'CLAUDE.md', file_path: 'CLAUDE.md', artifact_type: 'claude_md' },
  { label: '.cursorrules', file_path: '.cursorrules', artifact_type: 'cursorrules' },
  { label: 'Custom file', file_path: 'docs/context.md', artifact_type: 'supporting_file' },
];

export function StepCustomArtifacts({
  files,
  onChange,
}: {
  files: DraftArtifactFile[];
  onChange: (files: DraftArtifactFile[]) => void;
}) {
  return (
    <div className="space-y-3">
      {files.map((file) => (
        <div key={file.id} className="rounded-lg border border-slate-200 bg-white p-3">
          <div className="grid gap-2 sm:grid-cols-2">
            <Input
              value={file.file_path}
              onChange={(event) => onChange(files.map((item) => (item.id === file.id ? { ...item, file_path: event.target.value } : item)))}
              placeholder="Relative path (AGENTS.md, docs/context.md ...)"
            />
            <Input
              value={file.artifact_type}
              onChange={(event) => onChange(files.map((item) => (item.id === file.id ? { ...item, artifact_type: event.target.value } : item)))}
              placeholder="agents_md, claude_md, supporting_file"
            />
          </div>
          <textarea
            className="mt-3 min-h-36 w-full rounded-lg border border-slate-300 bg-white p-3 font-mono text-xs leading-5 shadow-[0_1px_2px_rgba(15,23,42,0.04)]"
            value={file.content}
            onChange={(event) => onChange(files.map((item) => (item.id === file.id ? { ...item, content: event.target.value } : item)))}
            placeholder="File contents"
          />
          <div className="mt-2 flex justify-end">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => onChange(files.filter((item) => item.id !== file.id))}
            >
              Remove
            </Button>
          </div>
        </div>
      ))}
      <div className="flex flex-wrap gap-2">
        {ARTIFACT_PRESETS.map((preset) => (
          <Button
            key={preset.label}
            type="button"
            variant="outline"
            size="sm"
            onClick={() =>
              onChange([
                ...files,
                {
                  id: crypto.randomUUID(),
                  artifact_type: preset.artifact_type,
                  file_path: preset.file_path,
                  content: '',
                },
              ])
            }
          >
            + {preset.label}
          </Button>
        ))}
      </div>
    </div>
  );
}
