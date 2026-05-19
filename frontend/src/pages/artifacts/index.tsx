import { Link } from 'react-router-dom';
import { Badge } from '../../components/ui/badge';
import { Card, CardHeader } from '../../components/ui/card';
import { EmptyState } from '../../components/ui/empty-state';
import { useExperiments } from '../../lib/hooks';

export function ArtifactsPage() {
  const { data: experiments = [] } = useExperiments();
  const variants = experiments.flatMap((experiment) =>
    (experiment.variants ?? []).map((variant) => ({ variant, experiment })),
  );

  return (
    <div className="space-y-4">
      <Card className="border-warning/30 bg-warning/10">
        <div className="flex items-start justify-between gap-3">
          <div>
            <div className="text-sm font-semibold text-warning-fg">Preview surface</div>
            <div className="mt-1 text-xs text-warning-fg">
              Artifact management will graduate into a dedicated editor. For now you can inspect existing variants
              below; editing is read-only and routed through the experiment wizard.
            </div>
          </div>
          <Badge tone="warning">Preview</Badge>
        </div>
      </Card>
      <Card>
        <CardHeader title="Variant artifacts" description="Context files attached per variant across all experiments." />
        {variants.length === 0 ? (
          <EmptyState
            title="No artifacts yet"
            description="Once you create an experiment with custom context files they will appear here."
          />
        ) : (
          <div className="space-y-2">
            {variants.map(({ variant, experiment }) => (
              <Link
                key={variant.id}
                className="flex items-center justify-between rounded-lg border border-border bg-bg-elev-1 px-4 py-3 text-sm transition hover:border-border-strong"
                to={`/artifacts/${variant.id}`}
              >
                <div>
                  <div className="font-medium text-fg">{variant.name}</div>
                  <div className="text-xs text-fg-muted">
                    {experiment.name} · {variant.is_control ? 'control' : 'comparison'}
                  </div>
                </div>
                <span className="text-xs text-fg-subtle">{variant.artifact_versions?.length ?? 0} artifacts</span>
              </Link>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
