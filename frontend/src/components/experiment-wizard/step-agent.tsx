import type { AgentInfo, ModelConfig } from '../../lib/types';

export function StepAgent({
  agents,
  models,
  agent,
  model,
  onAgentChange,
  onModelChange,
}: {
  agents: AgentInfo[];
  models: ModelConfig[];
  agent: string;
  model: string;
  onAgentChange: (value: string) => void;
  onModelChange: (value: string) => void;
}) {
  return (
    <div className="grid gap-3 sm:grid-cols-2">
      <label className="flex flex-col gap-1 text-xs font-medium text-slate-600">
        Agent CLI
        <select
          className="h-9 rounded-lg border border-slate-300 bg-white px-3 text-sm shadow-[0_1px_2px_rgba(15,23,42,0.04)]"
          value={agent}
          onChange={(event) => onAgentChange(event.target.value)}
        >
          {agents.map((item) => (
            <option key={item.name} value={item.name}>
              {item.name} {item.available ? '' : '(unavailable)'}
            </option>
          ))}
        </select>
      </label>
      <label className="flex flex-col gap-1 text-xs font-medium text-slate-600">
        Model
        <select
          className="h-9 rounded-lg border border-slate-300 bg-white px-3 text-sm shadow-[0_1px_2px_rgba(15,23,42,0.04)]"
          value={model}
          onChange={(event) => onModelChange(event.target.value)}
        >
          {models.map((item) => (
            <option key={item.model_id} value={item.model_id}>
              {item.display_name}
            </option>
          ))}
        </select>
      </label>
    </div>
  );
}
