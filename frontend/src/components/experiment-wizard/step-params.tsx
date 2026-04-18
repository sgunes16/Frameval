import { Input } from '../../components/ui/input';

export function StepParams({
  runsPerVariant,
  timeoutSeconds,
  onRunsChange,
  onTimeoutChange,
}: {
  runsPerVariant: number;
  timeoutSeconds: number;
  onRunsChange: (value: number) => void;
  onTimeoutChange: (value: number) => void;
}) {
  return (
    <div className="grid gap-3 sm:grid-cols-2">
      <label className="flex flex-col gap-1 text-xs font-medium text-slate-600">
        Runs per variant
        <Input
          type="number"
          min={1}
          value={runsPerVariant}
          onChange={(event) => onRunsChange(Number(event.target.value))}
        />
      </label>
      <label className="flex flex-col gap-1 text-xs font-medium text-slate-600">
        Timeout (seconds)
        <Input type="number" value={timeoutSeconds} onChange={(event) => onTimeoutChange(Number(event.target.value))} />
      </label>
    </div>
  );
}
