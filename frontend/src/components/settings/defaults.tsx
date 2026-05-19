import { Card, CardHeader } from '../ui/card';

export function DefaultsPanel() {
  return (
    <Card>
      <CardHeader title="Defaults" description="Starting values when you open the experiment wizard." />
      <dl className="grid grid-cols-2 gap-3 text-xs">
        <div>
          <dt className="text-fg-muted">Runs per variant</dt>
          <dd className="font-medium text-fg">5</dd>
        </div>
        <div>
          <dt className="text-fg-muted">Timeout</dt>
          <dd className="font-medium text-fg">600s</dd>
        </div>
        <div>
          <dt className="text-fg-muted">Concurrency</dt>
          <dd className="font-medium text-fg">3</dd>
        </div>
        <div>
          <dt className="text-fg-muted">Temperature</dt>
          <dd className="font-medium text-fg">0.0</dd>
        </div>
      </dl>
    </Card>
  );
}
