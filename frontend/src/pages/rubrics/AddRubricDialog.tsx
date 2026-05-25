import { useState } from 'react';
import { useCreateRubric } from '../../lib/hooks';
import { Button } from '../../components/ui/button';
import { Card, CardHeader } from '../../components/ui/card';
import { Input } from '../../components/ui/input';

const STARTER_PROMPT = `You are a strict senior code reviewer scoring ONE
dimension of an AI coding agent's output: **REPLACE_ME**.

Describe what this dimension measures and what to look for. Reference
specific evidence from the output files, test results, or transcript.

## Output format
Return a JSON object with:
- score: float in [0.0, 10.0]
- rationale: string up to 600 chars

## Calibration
Most outputs score 4-7. Reserve 8-10 for production-ready work.
Reserve 0-2 for output that does not address this dimension.`;

export function AddRubricDialog({ onClose }: { onClose: () => void }) {
  const [key, setKey] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [prompt, setPrompt] = useState(STARTER_PROMPT);
  const [error, setError] = useState<string | null>(null);
  const create = useCreateRubric();

  const onCreate = () => {
    setError(null);
    if (!/^[a-z][a-z0-9_]{1,40}$/.test(key)) {
      setError('Key must be lowercase snake_case, 2–41 chars (e.g., security).');
      return;
    }
    if (!displayName.trim()) {
      setError('Display name is required.');
      return;
    }
    if (prompt.length < 50) {
      setError('Prompt must be at least 50 characters.');
      return;
    }
    create.mutate(
      { key, display_name: displayName.trim(), prompt },
      {
        onSuccess: onClose,
        onError: (err) => setError(String(err)),
      },
    );
  };

  return (
    <Card>
      <CardHeader
        title="Add dimension"
        description="Define a new judge dimension. The LLM will score every grade against it."
      />
      <div className="space-y-3">
        <div>
          <label className="mb-1 block text-xs text-fg-muted">
            Key (lowercase snake_case, 2–41 chars)
          </label>
          <Input
            value={key}
            onChange={(e) => setKey(e.target.value)}
            placeholder="e.g., security"
          />
        </div>
        <div>
          <label className="mb-1 block text-xs text-fg-muted">Display name</label>
          <Input
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            placeholder="Security"
          />
        </div>
        <div>
          <label className="mb-1 block text-xs text-fg-muted">Prompt</label>
          <textarea
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            rows={20}
            className="w-full rounded-md border border-border bg-bg-elev-1 p-2 font-mono text-xs text-fg"
          />
        </div>
        {error && <div className="text-xs text-danger">{error}</div>}
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={onCreate} disabled={create.isPending}>
            {create.isPending ? 'Creating…' : 'Create'}
          </Button>
        </div>
      </div>
    </Card>
  );
}
