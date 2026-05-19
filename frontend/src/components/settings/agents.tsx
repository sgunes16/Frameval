import type { AgentInfo } from '../../lib/types';
import { Badge } from '../ui/badge';
import { Card, CardHeader } from '../ui/card';

export function AgentsPanel({ agents }: { agents: AgentInfo[] }) {
  return (
    <Card>
      <CardHeader title="Agents" description="CLI agents Frameval can invoke inside sandboxes." />
      <div className="space-y-2">
        {agents.map((agent) => (
          <div
            key={agent.name}
            className="flex items-center justify-between rounded-lg border border-border bg-bg-elev-1 px-3 py-2 text-sm"
          >
            <div>
              <div className="font-medium text-fg">{agent.name}</div>
              <div className="text-xs text-fg-muted">{agent.modes.join(' · ')}</div>
            </div>
            <Badge tone={agent.available ? 'success' : 'muted'}>{agent.available ? 'Available' : 'Not configured'}</Badge>
          </div>
        ))}
        {agents.length === 0 && <div className="text-xs text-fg-muted">No agents discovered yet.</div>}
      </div>
    </Card>
  );
}
