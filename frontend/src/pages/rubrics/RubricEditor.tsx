import { useState } from 'react';
import type { Rubric } from '../../lib/types';
import { useUpdateRubric } from '../../lib/hooks';
import { Button } from '../../components/ui/button';
import { Card, CardHeader } from '../../components/ui/card';
import { Input } from '../../components/ui/input';

export function RubricEditor({ rubric, onClose }: { rubric: Rubric; onClose: () => void }) {
  const [displayName, setDisplayName] = useState(rubric.display_name);
  const [prompt, setPrompt] = useState(rubric.prompt);
  const update = useUpdateRubric();

  const onSave = () => {
    update.mutate(
      { key: rubric.key, display_name: displayName, prompt },
      { onSuccess: onClose },
    );
  };

  return (
    <Card>
      <CardHeader
        title={`Edit ${rubric.key}`}
        description={
          rubric.is_builtin
            ? 'Builtin rubric — edits persist; cannot delete.'
            : 'Custom rubric.'
        }
      />
      <div className="space-y-3">
        <div>
          <label className="mb-1 block text-xs text-fg-muted">Display name</label>
          <Input value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
        </div>
        <div>
          <label className="mb-1 block text-xs text-fg-muted">
            Prompt (system prompt for this dimension's LLM call)
          </label>
          <textarea
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            rows={24}
            className="w-full rounded-md border border-border bg-bg-elev-1 p-2 font-mono text-xs text-fg"
          />
        </div>
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={onSave} disabled={update.isPending}>
            {update.isPending ? 'Saving…' : 'Save'}
          </Button>
        </div>
      </div>
    </Card>
  );
}
