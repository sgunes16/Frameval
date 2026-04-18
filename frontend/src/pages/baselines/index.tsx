import { Badge } from '../../components/ui/badge';
import { Card, CardHeader } from '../../components/ui/card';

export function BaselinesPage() {
  return (
    <div className="space-y-4">
      <Card className="border-slate-200 bg-slate-50">
        <div className="flex items-start justify-between gap-3">
          <div>
            <div className="text-sm font-semibold text-slate-900">Baselines are coming soon</div>
            <div className="mt-1 text-xs text-slate-500">
              Dedicated baseline management — reference runs, regression tracking, and drift alerts — is under
              construction. The rest of the app will operate without it for now.
            </div>
          </div>
          <Badge tone="muted">Soon</Badge>
        </div>
      </Card>
      <Card>
        <CardHeader title="Planned features" description="What the baseline surface will unlock once it lands." />
        <ul className="list-disc space-y-1 pl-5 text-sm text-slate-600">
          <li>Pin reference runs per task and compare new experiments against them.</li>
          <li>Detect regressions with deterministic metric thresholds.</li>
          <li>Surface drift over time with a changelog of baseline updates.</li>
        </ul>
      </Card>
    </div>
  );
}
