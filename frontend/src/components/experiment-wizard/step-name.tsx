import { Input } from '../../components/ui/input';

export function StepName({ name, description, onNameChange, onDescriptionChange }: { name: string; description: string; onNameChange: (value: string) => void; onDescriptionChange: (value: string) => void }) {
  return <div className="space-y-3"><Input value={name} onChange={(event) => onNameChange(event.target.value)} placeholder="Experiment name" /><textarea className="min-h-32 w-full rounded-md border border-slate-300 p-3 text-sm" value={description} onChange={(event) => onDescriptionChange(event.target.value)} placeholder="Description" /></div>;
}
