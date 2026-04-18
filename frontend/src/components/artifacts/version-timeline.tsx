import type { ArtifactVersion } from '../../lib/types';

export function VersionTimeline({ versions }: { versions: ArtifactVersion[] }) {
  return <div className="space-y-2">{versions.map((version) => <div key={version.id} className="rounded-md border border-slate-200 p-3 text-sm">{version.file_path} · {new Date(version.created_at).toLocaleString()}</div>)}</div>;
}
