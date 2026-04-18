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
            className="flex items-center justify-between rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm"
          >
            <div>
              <div className="font-medium text-slate-900">{model.display_name}</div>
              <div className="text-[11px] text-slate-500">{model.provider} · {model.model_id}</div>
            </div>
            <div className="text-[11px] text-slate-500">
              ${model.input_price_per_1k.toFixed(3)} / ${model.output_price_per_1k.toFixed(3)} per 1K
            </div>
          </div>
        ))}
        {models.length === 0 && <div className="text-xs text-slate-500">No models registered.</div>}
      </div>
    </Card>
  );
}
