import { useMemo } from 'react';
import { useQueries } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { ComparisonTable } from '../../components/results/comparison-table';
import { DimensionDrilldown } from '../../components/results/dimension-drilldown';
import { ExportButton } from '../../components/results/export-button';
import { HeatmapChart } from '../../components/results/heatmap';
import { ScatterPlotChart } from '../../components/results/scatter-plot';
import { SummaryCard } from '../../components/results/summary-card';
import { TranscriptDiff } from '../../components/results/transcript-diff';
import { Card } from '../../components/ui/card';
import { api } from '../../lib/api';
import { useExperiment, useExperimentStats, useRuns } from '../../lib/hooks';
import type { ArtifactVersion, ExperimentStat, Grade, Run, Variant } from '../../lib/types';

export function ExperimentResultsPage() {
  const { id } = useParams();
  const { data: experiment } = useExperiment(id);
  const { data: runs = [] } = useRuns(id);
  const { data: fetchedStats = [] } = useExperimentStats(id);
  const runQueries = useQueries({
    queries: runs.map((run) => ({
      queryKey: ['run', run.id],
      queryFn: () => api.get<Run>(`/runs/${run.id}`),
      enabled: Boolean(run.id),
    })),
  });

  const detailedRuns = useMemo(() => {
    const runMap = new Map(runQueries.map((query) => [query.data?.id, query.data] as const));
    return runs.map((run) => runMap.get(run.id) ?? run);
  }, [runQueries, runs]);

  const variantNames = useMemo(() => new Map((experiment?.variants ?? []).map((variant) => [variant.id, variant.name])), [experiment?.variants]);
  const stats = useMemo(() => {
    if (fetchedStats.length > 0) {
      return fetchedStats;
    }
    return buildFallbackStats(detailedRuns, experiment?.variants?.map((variant) => variant.id) ?? []);
  }, [detailedRuns, experiment?.variants, fetchedStats]);
  const bestScore = useMemo(() => Math.max(0, ...detailedRuns.map((run) => run.grade?.composite_score ?? 0)), [detailedRuns]);
  const variantSummary = useMemo(() => buildVariantSummary(detailedRuns, variantNames), [detailedRuns, variantNames]);
  const transcriptPair = useMemo(() => buildTranscriptPair(detailedRuns, variantNames), [detailedRuns, variantNames]);
  const contextSummary = useMemo(() => buildContextSummary(experiment?.variants ?? []), [experiment?.variants]);

  return (
    <div className="space-y-4">
      <Card className="border-slate-200 bg-slate-50/60">
        <div className="text-sm text-slate-700">
          Deterministic results prioritised. Composite score is derived from test pass rate and process metrics;
          LLM judge and spec adherence modules remain in the architecture but are toggleable at runtime.
        </div>
      </Card>
      <div className="grid gap-4 md:grid-cols-4"><SummaryCard title="Status" value={experiment?.status || '-'} /><SummaryCard title="Runs" value={runs.length} /><SummaryCard title="Completed" value={detailedRuns.filter((run) => run.status === 'completed').length} /><SummaryCard title="Best score" value={bestScore.toFixed(2)} /></div>
      <Card>
        <div className="mb-3 text-lg font-semibold">Variant summary</div>
        <div className="overflow-auto rounded-md border border-slate-200">
          <table className="min-w-full text-sm">
            <thead className="bg-slate-100">
              <tr>
                <th className="px-3 py-2 text-left">Variant</th>
                <th className="px-3 py-2 text-left">Completed</th>
                <th className="px-3 py-2 text-left">Failed</th>
                <th className="px-3 py-2 text-left">Avg composite</th>
                <th className="px-3 py-2 text-left">Avg test pass</th>
                <th className="px-3 py-2 text-left">Avg token efficiency</th>
                <th className="px-3 py-2 text-left">Avg duration</th>
                <th className="px-3 py-2 text-left">Best run</th>
              </tr>
            </thead>
            <tbody>
              {variantSummary.map((summary) => (
                <tr key={summary.name} className="border-t border-slate-200">
                  <td className="px-3 py-2 font-medium">{summary.name}</td>
                  <td className="px-3 py-2">{summary.completed}</td>
                  <td className="px-3 py-2">{summary.failed}</td>
                  <td className="px-3 py-2">{summary.avgComposite.toFixed(2)}</td>
                  <td className="px-3 py-2">{summary.avgTestPass.toFixed(2)}</td>
                  <td className="px-3 py-2">{summary.avgTokenEfficiency.toFixed(2)}</td>
                  <td className="px-3 py-2">{summary.avgDuration.toFixed(1)}s</td>
                  <td className="px-3 py-2">{summary.bestComposite.toFixed(2)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>
      <Card>
        <div className="mb-3 text-lg font-semibold">Context sources</div>
        <div className="space-y-3">
          {contextSummary.map((summary) => (
            <div key={summary.name} className="rounded-md border border-slate-200 p-3">
              <div className="font-medium">{summary.name}</div>
              <div className="mt-1 text-sm text-slate-600">Catalog extension: {summary.catalogExtensions || '-'} </div>
              <div className="mt-1 text-sm text-slate-600">Custom file count: {summary.customFileCount}</div>
              <div className="mt-1 text-sm text-slate-600">Workspace files: {summary.filePaths || 'No additional context'}</div>
            </div>
          ))}
          {!contextSummary.length && <div className="text-sm text-slate-500">Variant context summary is not ready yet.</div>}
        </div>
      </Card>
      <Card><div className="mb-3 flex items-center justify-between"><div className="text-lg font-semibold">Statistical analysis</div><ExportButton href={`/api/experiments/${id}/export/json`} /></div><ComparisonTable stats={stats} /></Card>
      <div className="grid gap-4 lg:grid-cols-2"><Card><div className="mb-3 text-lg font-semibold">Heatmap</div><HeatmapChart stats={stats} /></Card><Card><div className="mb-3 text-lg font-semibold">Scatter</div><ScatterPlotChart stats={stats} /></Card></div>
      <Card><div className="mb-3 text-lg font-semibold">Transcript diff</div><TranscriptDiff before={transcriptPair.before} after={transcriptPair.after} beforeLabel={transcriptPair.beforeLabel} afterLabel={transcriptPair.afterLabel} /></Card>
      <Card><div className="mb-3 text-lg font-semibold">Dimension drilldown</div><DimensionDrilldown artifact={experiment?.variants?.[0]?.artifact_versions?.[0]} /></Card>
    </div>
  );
}

function buildFallbackStats(runs: Run[], variantIDs: string[]): ExperimentStat[] {
  if (variantIDs.length < 2) {
    return [];
  }

  const [variantAID, variantBID] = variantIDs;
  const variantARuns = runs.filter((run) => run.variant_id === variantAID && run.grade);
  const variantBRuns = runs.filter((run) => run.variant_id === variantBID && run.grade);
  if (variantARuns.length === 0 || variantBRuns.length === 0) {
    return [];
  }

  const metrics: Array<{ key: keyof Grade; label: string }> = [
    { key: 'composite_score', label: 'Composite score' },
    { key: 'test_pass_rate', label: 'Test pass rate' },
    { key: 'token_efficiency', label: 'Token efficiency' },
    { key: 'context_utilization', label: 'Context utilization' },
  ];

  return metrics.map((metric) => ({
    metric_name: metric.label,
    variant_a_id: variantAID,
    variant_b_id: variantBID,
    mean_a: averageMetric(variantARuns, metric.key),
    mean_b: averageMetric(variantBRuns, metric.key),
    p_value: Number.NaN,
    is_significant: false,
  }));
}

function averageMetric(runs: Run[], key: keyof Grade): number {
  const values = runs
    .map((run) => run.grade?.[key])
    .filter((value): value is number => typeof value === 'number');
  if (values.length === 0) {
    return 0;
  }
  return values.reduce((sum, value) => sum + value, 0) / values.length;
}

function buildVariantSummary(runs: Run[], variantNames: Map<string, string>) {
  const grouped = new Map<string, Run[]>();
  for (const run of runs) {
    const key = variantNames.get(run.variant_id) ?? run.variant_id;
    grouped.set(key, [...(grouped.get(key) ?? []), run]);
  }

  return Array.from(grouped.entries()).map(([name, groupedRuns]) => {
    const grades = groupedRuns.map((run) => run.grade).filter((grade): grade is Grade => Boolean(grade));
    return {
      name,
      completed: groupedRuns.filter((run) => run.status === 'completed').length,
      failed: groupedRuns.filter((run) => run.status === 'failed').length,
      avgComposite: averageGrade(grades, 'composite_score'),
      avgTestPass: averageGrade(grades, 'test_pass_rate'),
      avgTokenEfficiency: averageGrade(grades, 'token_efficiency'),
      avgDuration: averageDuration(groupedRuns),
      bestComposite: Math.max(0, ...grades.map((grade) => grade.composite_score ?? 0)),
    };
  });
}

function averageGrade(grades: Grade[], key: keyof Grade): number {
  const values = grades
    .map((grade) => grade[key])
    .filter((value): value is number => typeof value === 'number');
  if (values.length === 0) {
    return 0;
  }
  return values.reduce((sum, value) => sum + value, 0) / values.length;
}

function averageDuration(runs: Run[]): number {
  const values = runs.map((run) => run.duration_seconds).filter((value): value is number => typeof value === 'number');
  if (values.length === 0) {
    return 0;
  }
  return values.reduce((sum, value) => sum + value, 0) / values.length;
}

function buildTranscriptPair(runs: Run[], variantNames: Map<string, string>) {
  const completedRuns = runs.filter((run) => run.status === 'completed' && run.transcript);
  const byVariant = new Map<string, Run[]>();
  for (const run of completedRuns) {
    const key = variantNames.get(run.variant_id) ?? run.variant_id;
    byVariant.set(key, [...(byVariant.get(key) ?? []), run]);
  }

  const variants = Array.from(byVariant.entries())
    .map(([name, variantRuns]) => ({
      name,
      run: [...variantRuns].sort((left, right) => (right.grade?.composite_score ?? 0) - (left.grade?.composite_score ?? 0))[0],
    }))
    .filter((item) => item.run);

  return {
    beforeLabel: variants[0]?.name ?? 'Variant A',
    before: variants[0]?.run?.transcript?.raw_output ?? '',
    afterLabel: variants[1]?.name ?? variants[0]?.name ?? 'Variant B',
    after: variants[1]?.run?.transcript?.raw_output ?? variants[0]?.run?.transcript?.raw_output ?? '',
  };
}

function buildContextSummary(variants: Variant[]) {
  return variants.map((variant) => {
    const artifacts = variant.artifact_versions ?? [];
    const catalogExtensions = Array.from(new Set(artifacts.filter((artifact) => artifact.source_kind === 'catalog_extension').map((artifact) => artifact.source_ref).filter(Boolean)));
    const customFiles = artifacts.filter((artifact) => artifact.source_kind !== 'catalog_extension');
    return {
      name: variant.name,
      catalogExtensions: catalogExtensions.join(', '),
      customFileCount: customFiles.length,
      filePaths: summarizePaths(artifacts),
    };
  });
}

function summarizePaths(artifacts: ArtifactVersion[]): string {
  const paths = artifacts.map((artifact) => artifact.file_path).filter(Boolean);
  if (paths.length === 0) {
    return '';
  }
  return paths.slice(0, 4).join(', ') + (paths.length > 4 ? ` +${paths.length - 4} more` : '');
}
