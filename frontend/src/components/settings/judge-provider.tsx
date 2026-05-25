import { useEffect, useState } from 'react';
import type { LLMProvider } from '../../lib/types';
import { useLLMSettings, useSaveLLMSettings } from '../../lib/hooks';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardHeader } from '../ui/card';
import { Input } from '../ui/input';

const PROVIDERS: { value: LLMProvider; label: string; defaultModel: string }[] = [
  { value: 'openrouter', label: 'OpenRouter (free tier)', defaultModel: 'deepseek/deepseek-chat-v3-0324:free' },
  { value: 'zai',        label: 'Z.ai',                   defaultModel: 'glm-4.6' },
  { value: 'ollama',     label: 'Ollama (local)',         defaultModel: 'qwen2.5-coder:32b' },
  { value: 'openai',     label: 'OpenAI',                 defaultModel: 'gpt-4o-mini' },
  { value: 'anthropic',  label: 'Anthropic',              defaultModel: 'claude-haiku-4-5-20251001' },
];

export function JudgeProviderPanel() {
  const { data: settings, isLoading } = useLLMSettings();
  const save = useSaveLLMSettings();
  const [provider, setProvider] = useState<LLMProvider>('openrouter');
  const [model, setModel] = useState('');
  const [enabled, setEnabled] = useState(true);

  // Initialize local state once settings load.
  useEffect(() => {
    if (settings) {
      setProvider(settings.provider);
      setModel(settings.model);
      setEnabled(settings.enabled);
    }
  }, [settings]);

  if (isLoading || !settings) {
    return (
      <Card>
        <CardHeader title="LLM judge" description="Loading…" />
      </Card>
    );
  }

  const placeholder = PROVIDERS.find((p) => p.value === provider)?.defaultModel ?? '';
  const dirty =
    provider !== settings.provider || model !== settings.model || enabled !== settings.enabled;

  const keyStatus = () => {
    if (provider !== settings.provider) {
      return null; // wait for save to refresh
    }
    if (settings.api_key_present) {
      return <Badge tone="success">API key present</Badge>;
    }
    if (provider === 'ollama') {
      return <Badge tone="muted">no key needed</Badge>;
    }
    return <Badge tone="danger">API key missing</Badge>;
  };

  return (
    <Card>
      <CardHeader
        title="LLM judge"
        description="Picks the model that scores agent runs on 5 dimensions. Changes take effect on the next experiment."
      />
      <div className="space-y-3">
        <div className="grid grid-cols-[120px_1fr] items-center gap-2">
          <label className="text-sm text-fg-muted">Provider</label>
          <select
            value={provider}
            onChange={(e) => setProvider(e.target.value as LLMProvider)}
            className="rounded-md border border-border bg-bg-elev-1 px-2 py-1.5 text-sm text-fg"
          >
            {PROVIDERS.map((p) => (
              <option key={p.value} value={p.value}>{p.label}</option>
            ))}
          </select>

          <label className="text-sm text-fg-muted">Model</label>
          <Input
            value={model}
            onChange={(e) => setModel(e.target.value)}
            placeholder={placeholder}
          />

          <label className="text-sm text-fg-muted">Enabled</label>
          <div>
            <Button
              variant={enabled ? 'primary' : 'ghost'}
              onClick={() => setEnabled((v) => !v)}
            >
              {enabled ? 'On' : 'Off'}
            </Button>
          </div>
        </div>

        <div className="flex items-center justify-between gap-2 border-t border-border pt-3 text-xs">
          <div className="flex items-center gap-2 text-fg-muted">
            <span>Status:</span>
            <span>{settings.provider} / {settings.model}</span>
            {keyStatus()}
          </div>
          <Button onClick={() => save.mutate({ provider, model, enabled })} disabled={!dirty || save.isPending}>
            {save.isPending ? 'Saving…' : 'Save'}
          </Button>
        </div>
      </div>
    </Card>
  );
}
