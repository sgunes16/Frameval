import { useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import { ArtifactDiff } from '../../components/artifacts/artifact-diff';
import { ArtifactEditor } from '../../components/artifacts/artifact-editor';
import { ArtifactUpload } from '../../components/artifacts/artifact-upload';
import { Card, CardHeader } from '../../components/ui/card';
import { useArtifacts, useCreateArtifact } from '../../lib/hooks';

export function ArtifactDetailPage() {
  const { id } = useParams();
  const { data: versions = [] } = useArtifacts(id);
  const latest = versions[0];
  const [content, setContent] = useState(latest?.content ?? '');
  const createArtifact = useCreateArtifact();
  const diff = useMemo(() => {
    if (versions.length < 2) return '';
    return versions[1].content === versions[0].content
      ? ''
      : `- ${versions[1].content}\n+ ${versions[0].content}`;
  }, [versions]);

  return (
    <div className="grid gap-4 lg:grid-cols-[2fr_1fr]">
      <Card>
        <CardHeader title="Artifact editor" description="Read-only preview of the latest context file for this variant." />
        <ArtifactEditor value={content} onChange={setContent} />
        {latest && (
          <div className="mt-4 space-y-3">
            <ArtifactDiff diff={diff} />
          </div>
        )}
      </Card>
      <Card>
        <CardHeader title="New version" description="Upload a follow-up revision or different artifact." />
        <ArtifactUpload onCreate={(payload) => id && createArtifact.mutate({ variantId: id, payload })} />
      </Card>
    </div>
  );
}
