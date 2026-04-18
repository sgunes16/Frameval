import { AgentsPanel } from '../../components/settings/agents';
import { ApiKeysPanel } from '../../components/settings/api-keys';
import { DefaultsPanel } from '../../components/settings/defaults';
import { ModelsPanel } from '../../components/settings/models';
import { Card, CardHeader } from '../../components/ui/card';
import { useAgents, useAPIKeys, useModels } from '../../lib/hooks';

export function SettingsPage() {
  const { data: agents = [] } = useAgents();
  const { data: apiKeys = [] } = useAPIKeys();
  const { data: models = [] } = useModels();
  return (
    <div className="space-y-4">
      <Card className="border-slate-200 bg-slate-50/60">
        <CardHeader
          title="Environment"
          description="Frameval reads configuration from environment variables. Restart the engine to apply changes."
        />
        <div className="grid gap-2 text-xs text-slate-600 sm:grid-cols-2">
          <span><code className="rounded bg-white px-2 py-1">ANTHROPIC_API_KEY</code> — Claude / Cursor</span>
          <span><code className="rounded bg-white px-2 py-1">OPENAI_API_KEY</code> — GPT family</span>
          <span><code className="rounded bg-white px-2 py-1">GOOGLE_API_KEY</code> — Gemini</span>
          <span><code className="rounded bg-white px-2 py-1">FRAMEVAL_ENABLE_LLM_JUDGE</code> — enable LLM grading</span>
        </div>
      </Card>
      <div className="grid gap-4 lg:grid-cols-2">
        <ApiKeysPanel keys={apiKeys} />
        <ModelsPanel models={models} />
        <AgentsPanel agents={agents} />
        <DefaultsPanel />
      </div>
    </div>
  );
}
