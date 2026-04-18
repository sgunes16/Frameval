import { Card, CardHeader } from '../ui/card';

export function DefaultsPanel() {
  return (
    <Card>
      <CardHeader title="Defaults" description="Starting values when you open the experiment wizard." />
      <dl className="grid grid-cols-2 gap-3 text-xs">
        <div>
          <dt className="text-slate-500">Runs per variant</dt>
          <dd className="font-medium text-slate-900">5</dd>
        </div>
        <div>
          <dt className="text-slate-500">Timeout</dt>
          <dd className="font-medium text-slate-900">600s</dd>
        </div>
        <div>
          <dt className="text-slate-500">Concurrency</dt>
          <dd className="font-medium text-slate-900">3</dd>
        </div>
        <div>
          <dt className="text-slate-500">Temperature</dt>
          <dd className="font-medium text-slate-900">0.0</dd>
        </div>
      </dl>
    </Card>
  );
}
