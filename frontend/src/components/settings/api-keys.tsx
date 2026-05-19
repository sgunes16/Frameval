import type { APIKey } from '../../lib/types';
import { Card, CardHeader } from '../ui/card';

export function ApiKeysPanel({ keys }: { keys: APIKey[] }) {
  return (
    <Card>
      <CardHeader title="API keys" description="Set these via environment variables before starting the engine." />
      <div className="space-y-2">
        {keys.map((key) => (
          <div
            key={key.id}
            className="flex items-center justify-between rounded-lg border border-border bg-bg-elev-1 px-3 py-2 text-sm"
          >
            <div className="font-medium capitalize text-fg">{key.provider}</div>
            <code className="rounded bg-bg-elev-2 px-2 py-1 text-xs text-fg-muted">{key.redacted_key}</code>
          </div>
        ))}
        {keys.length === 0 && <div className="text-xs text-fg-muted">No API keys detected in the environment.</div>}
      </div>
    </Card>
  );
}
