import type { ModelConfig } from '../../lib/types';
import { Card, CardHeader } from '../ui/card';

export function ModelsPanel({ models }: { models: ModelConfig[] }) {
  return (
    <Card>
      <CardHeader title="Models" description="Registered model catalog used by the agent runner." />
      <div className="space-y-2">
        {models.map((model) => (
          <div
            key={model.id}
            className="flex items-center justify-between rounded-lg border border-border bg-bg-elev-1 px-3 py-2 text-sm"
          >
            <div>
              <div className="font-medium text-fg">{model.display_name}</div>
              <div className="text-[11px] text-fg-muted">{model.provider} · {model.model_id}</div>
            </div>
            <div className="text-[11px] text-fg-muted">
              ${model.input_price_per_1k.toFixed(3)} / ${model.output_price_per_1k.toFixed(3)} per 1K
            </div>
          </div>
        ))}
        {models.length === 0 && <div className="text-xs text-fg-muted">No models registered.</div>}
      </div>
    </Card>
  );
}
