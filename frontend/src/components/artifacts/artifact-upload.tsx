import { useState } from 'react';
import { Button } from '../ui/button';
import { Input } from '../ui/input';

export function ArtifactUpload({ onCreate }: { onCreate: (payload: { artifact_type: string; file_path: string; content: string }) => void }) {
  const [artifactType, setArtifactType] = useState('claude_md');
  const [filePath, setFilePath] = useState('CLAUDE.md');
  const [content, setContent] = useState('');
  return (
    <div className="space-y-3">
      <Input value={artifactType} onChange={(event) => setArtifactType(event.target.value)} placeholder="Artifact type" />
      <Input value={filePath} onChange={(event) => setFilePath(event.target.value)} placeholder="File path" />
      <textarea className="min-h-40 w-full rounded-md border border-border-strong p-3 text-sm" value={content} onChange={(event) => setContent(event.target.value)} />
      <Button onClick={() => onCreate({ artifact_type: artifactType, file_path: filePath, content })}>Save artifact</Button>
    </div>
  );
}
