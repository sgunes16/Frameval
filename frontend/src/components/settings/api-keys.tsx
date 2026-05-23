import { useState } from 'react';
import type { APIKey } from '../../lib/types';
import { useDeleteAPIKey, useUpsertAPIKey } from '../../lib/hooks';
import { Button } from '../ui/button';
import { Card, CardHeader } from '../ui/card';
import { Input } from '../ui/input';

// Providers the panel always shows a row for, even with no stored key.
// Keep this in sync with grader/llm_client.py _PRESETS plus the existing
// 'cursor' provider already accepted by the api_keys CHECK constraint.
const KNOWN_PROVIDERS = ['openrouter', 'zai', 'ollama', 'openai', 'anthropic', 'cursor'] as const;

export function ApiKeysPanel({ keys }: { keys: APIKey[] }) {
  const stored = new Map(keys.map((k) => [k.provider, k]));
  const rows = KNOWN_PROVIDERS.map((p) => ({
    provider: p,
    stored: stored.get(p),
  }));

  return (
    <Card>
      <CardHeader
        title="API keys"
        description="Stored encrypted in the local database. Editing here takes effect on the next experiment — no engine restart needed."
      />
      <div className="space-y-2">
        {rows.map(({ provider, stored }) => (
          <ApiKeyRow key={provider} provider={provider} stored={stored} />
        ))}
      </div>
    </Card>
  );
}

function ApiKeyRow({ provider, stored }: { provider: string; stored?: APIKey }) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState('');
  const upsert = useUpsertAPIKey();
  const del = useDeleteAPIKey();

  const onSave = () => {
    if (!value.trim()) return;
    upsert.mutate(
      { provider, api_key: value.trim() },
      {
        onSuccess: () => {
          setValue('');
          setEditing(false);
        },
      },
    );
  };

  return (
    <div className="flex items-center justify-between gap-2 rounded-lg border border-border bg-bg-elev-1 px-3 py-2 text-sm">
      <div className="flex flex-1 items-center gap-3">
        <div className="w-24 font-medium capitalize text-fg">{provider}</div>
        {editing ? (
          <Input
            type="password"
            autoFocus
            placeholder={`Enter ${provider} API key`}
            value={value}
            onChange={(e) => setValue(e.target.value)}
            className="flex-1"
          />
        ) : stored ? (
          <code className="rounded bg-bg-elev-2 px-2 py-1 text-xs text-fg-muted">{stored.redacted_key}</code>
        ) : (
          <span className="text-xs text-fg-muted">not set</span>
        )}
      </div>
      <div className="flex gap-1">
        {editing ? (
          <>
            <Button onClick={onSave} disabled={!value.trim() || upsert.isPending}>
              Save
            </Button>
            <Button
              variant="ghost"
              onClick={() => {
                setValue('');
                setEditing(false);
              }}
            >
              Cancel
            </Button>
          </>
        ) : (
          <>
            <Button variant="ghost" onClick={() => setEditing(true)}>
              {stored ? 'Edit' : 'Add'}
            </Button>
            {stored && (
              <Button
                variant="ghost"
                onClick={() => del.mutate(provider)}
                disabled={del.isPending}
              >
                Delete
              </Button>
            )}
          </>
        )}
      </div>
    </div>
  );
}
