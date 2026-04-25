import { useMemo, useState } from 'react';
import { useQueries } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  PolarAngleAxis,
  PolarGrid,
  PolarRadiusAxis,
  Radar,
  RadarChart,
  ResponsiveContainer,
  Scatter,
  ScatterChart,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { ExportButton } from '../../components/results/export-button';
import { Card, CardHeader } from '../../components/ui/card';
import { api } from '../../lib/api';
import { useExperiment, useExperimentStats, useRuns } from '../../lib/hooks';
import type { ExperimentStat, Grade, Run, Variant } from '../../lib/types';

const VARIANT_COLORS = ['#6366f1', '#f59e0b', '#10b981', '#ef4444', '#8b5cf6', '#06b6d4'];
const METRIC_LABELS: Record<string, string> = {
  composite_score: 'Composite',
  test_pass_rate: 'Test pass',
  lint_score: 'Lint',
  token_efficiency: 'Token eff.',
  context_utilization: 'Context util.',
  judge_correctness: 'LLM judge',
  spec_instruction_compliance: 'Spec comply',
};

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
    const runMap = new Map(runQueries.map((q) => [q.data?.id, q.data] as const));
    return runs.map((run) => runMap.get(run.id) ?? run);
  }, [runQueries, runs]);

  const variants = experiment?.variants ?? [];
  const variantNames = useMemo(
    () => new Map(variants.map((v) => [v.id, v.name])),
    [variants],
  );

  const stats = useMemo(() => {
    if (fetchedStats.length > 0) return fetchedStats;
    return buildFallbackStats(detailedRuns, variants.map((v) => v.id));
  }, [detailedRuns, variants, fetchedStats]);

  const variantSummaries = useMemo(
    () => buildVariantSummaries(detailedRuns, variants),
    [detailedRuns, variants],
  );

  const radarData = useMemo(
    () => buildRadarData(variantSummaries),
    [variantSummaries],
  );

  const barData = useMemo(
    () => buildBarData(stats, variantNames),
    [stats, variantNames],
  );

  const scatterData = useMemo(
    () => buildScatterData(detailedRuns, variants),
    [detailedRuns, variants],
  );

  const completedRuns = detailedRuns.filter((r) => r.status === 'completed');
  const bestScore = Math.max(0, ...completedRuns.map((r) => r.grade?.composite_score ?? 0));
  const avgScore =
    completedRuns.length > 0
      ? completedRuns.reduce((s, r) => s + (r.grade?.composite_score ?? 0), 0) / completedRuns.length
      : 0;

  const winnerVariant = useMemo(() => {
    if (variantSummaries.length < 2) return null;
    return variantSummaries.reduce((best, cur) => (cur.avgComposite > best.avgComposite ? cur : best));
  }, [variantSummaries]);

  const [expandedTest, setExpandedTest] = useState<string | null>(null);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold text-slate-900">{experiment?.name ?? 'Results'}</h1>
          <p className="mt-1 text-sm text-slate-500">
            {experiment?.agent_cli} · {experiment?.model} · {runs.length} run{runs.length !== 1 ? 's' : ''}
          </p>
        </div>
        <ExportButton href={`/api/experiments/${id}/export/json`} />
      </div>

      {/* Hero metrics */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <HeroCard
          label="Best score"
          value={bestScore.toFixed(3)}
          sub="composite"
          accent="indigo"
        />
        <HeroCard
          label="Avg score"
          value={avgScore.toFixed(3)}
          sub="across all runs"
          accent="violet"
        />
        <HeroCard
          label="Completed"
          value={`${completedRuns.length} / ${runs.length}`}
          sub="runs"
          accent="emerald"
        />
        <HeroCard
          label="Winner"
          value={winnerVariant?.name ?? '—'}
          sub={winnerVariant ? `avg ${winnerVariant.avgComposite.toFixed(3)}` : 'no data'}
          accent="amber"
        />
      </div>

      {/* Variant summary row */}
      {variantSummaries.length > 0 && (
        <div className="grid gap-4" style={{ gridTemplateColumns: `repeat(${Math.min(variantSummaries.length, 3)}, 1fr)` }}>
          {variantSummaries.map((summary, i) => (
            <VariantSummaryCard key={summary.id} summary={summary} color={VARIANT_COLORS[i % VARIANT_COLORS.length]} />
          ))}
        </div>
      )}

      {/* Charts row: radar + bar side by side */}
      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader
            title="Metric radar"
            description="Avg scores across dimensions per variant"
          />
          {radarData.metrics.length > 0 ? (
            <ResponsiveContainer width="100%" height={300}>
              <RadarChart data={radarData.points}>
                <PolarGrid stroke="#e2e8f0" />
                <PolarAngleAxis dataKey="metric" tick={{ fill: '#64748b', fontSize: 11 }} />
                <PolarRadiusAxis domain={[0, 1]} tick={{ fill: '#94a3b8', fontSize: 10 }} tickCount={4} />
                {radarData.variantNames.map((name, i) => (
                  <Radar
                    key={name}
                    name={name}
                    dataKey={name}
                    stroke={VARIANT_COLORS[i % VARIANT_COLORS.length]}
                    fill={VARIANT_COLORS[i % VARIANT_COLORS.length]}
                    fillOpacity={0.12}
                    strokeWidth={2}
                  />
                ))}
                <Tooltip
                  contentStyle={{ fontSize: 12, borderRadius: 8, border: '1px solid #e2e8f0' }}
                  formatter={(v: number) => v.toFixed(3)}
                />
                <Legend wrapperStyle={{ fontSize: 12, paddingTop: 12 }} />
              </RadarChart>
            </ResponsiveContainer>
          ) : (
            <EmptyChart />
          )}
        </Card>

        <Card>
          <CardHeader
            title="Metric comparison"
            description="Δ mean per metric (comparison − control)"
          />
          {barData.length > 0 ? (
            <ResponsiveContainer width="100%" height={300}>
              <BarChart data={barData} margin={{ top: 4, right: 8, left: -16, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
                <XAxis dataKey="metric" tick={{ fill: '#64748b', fontSize: 11 }} />
                <YAxis tick={{ fill: '#94a3b8', fontSize: 11 }} />
                <Tooltip
                  contentStyle={{ fontSize: 12, borderRadius: 8, border: '1px solid #e2e8f0' }}
                  formatter={(v: number) => v.toFixed(3)}
                />
                <Legend wrapperStyle={{ fontSize: 12, paddingTop: 8 }} />
                {radarData.variantNames.map((name, i) => (
                  <Bar key={name} dataKey={name} fill={VARIANT_COLORS[i % VARIANT_COLORS.length]} radius={[4, 4, 0, 0]} />
                ))}
              </BarChart>
            </ResponsiveContainer>
          ) : (
            <EmptyChart />
          )}
        </Card>
      </div>

      {/* Run scatter: token efficiency vs composite */}
      <Card>
        <CardHeader
          title="Run scatter"
          description="Each dot = one run. X = token efficiency, Y = composite score. Hover for details."
        />
        {scatterData.series.length > 0 ? (
          <ResponsiveContainer width="100%" height={300}>
            <ScatterChart margin={{ top: 4, right: 16, left: -16, bottom: 4 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
              <XAxis
                type="number"
                dataKey="x"
                name="Token eff."
                domain={[0, 'auto']}
                tick={{ fill: '#64748b', fontSize: 11 }}
                label={{ value: 'Token efficiency', position: 'insideBottom', offset: -4, fill: '#94a3b8', fontSize: 11 }}
              />
              <YAxis
                type="number"
                dataKey="y"
                name="Composite"
                domain={[0, 'auto']}
                tick={{ fill: '#64748b', fontSize: 11 }}
                label={{ value: 'Composite', angle: -90, position: 'insideLeft', fill: '#94a3b8', fontSize: 11 }}
              />
              <Tooltip
                contentStyle={{ fontSize: 12, borderRadius: 8, border: '1px solid #e2e8f0' }}
                content={<RunScatterTooltip />}
              />
              <Legend wrapperStyle={{ fontSize: 12, paddingTop: 8 }} />
              {scatterData.series.map((s, i) => (
                <Scatter
                  key={s.name}
                  name={s.name}
                  data={s.points}
                  fill={VARIANT_COLORS[i % VARIANT_COLORS.length]}
                >
                  {s.points.map((_, pi) => (
                    <Cell key={pi} fill={VARIANT_COLORS[i % VARIANT_COLORS.length]} />
                  ))}
                </Scatter>
              ))}
            </ScatterChart>
          </ResponsiveContainer>
        ) : (
          <EmptyChart />
        )}
      </Card>

      {/* Duration bar chart */}
      {variantSummaries.length > 0 && (
        <Card>
          <CardHeader title="Run duration" description="Avg seconds per run per variant" />
          <ResponsiveContainer width="100%" height={180}>
            <BarChart
              data={variantSummaries.map((s) => ({ name: s.name, 'Avg duration (s)': Math.round(s.avgDuration) }))}
              margin={{ top: 4, right: 16, left: -16, bottom: 0 }}
            >
              <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
              <XAxis dataKey="name" tick={{ fill: '#64748b', fontSize: 11 }} />
              <YAxis tick={{ fill: '#94a3b8', fontSize: 11 }} />
              <Tooltip contentStyle={{ fontSize: 12, borderRadius: 8, border: '1px solid #e2e8f0' }} />
              {variantSummaries.map((s, i) => (
                <Bar key={s.id} dataKey="Avg duration (s)" fill={VARIANT_COLORS[i % VARIANT_COLORS.length]} radius={[4, 4, 0, 0]} />
              ))}
            </BarChart>
          </ResponsiveContainer>
        </Card>
      )}

      {/* Statistical comparison table */}
      {stats.length > 0 && (
        <Card>
          <CardHeader title="Statistical comparison" description="Mann-Whitney U test results" />
          <div className="overflow-auto rounded-lg border border-slate-200">
            <table className="min-w-full text-sm">
              <thead>
                <tr className="border-b border-slate-200 bg-slate-50 text-xs text-slate-500">
                  <th className="px-4 py-2.5 text-left font-medium">Metric</th>
                  <th className="px-4 py-2.5 text-right font-medium">{variantNames.get(stats[0]?.variant_a_id) ?? 'Variant A'}</th>
                  <th className="px-4 py-2.5 text-right font-medium">{variantNames.get(stats[0]?.variant_b_id) ?? 'Variant B'}</th>
                  <th className="px-4 py-2.5 text-right font-medium">Δ</th>
                  <th className="px-4 py-2.5 text-right font-medium">p-value</th>
                  <th className="px-4 py-2.5 text-center font-medium">Significant</th>
                </tr>
              </thead>
              <tbody>
                {stats.map((stat) => {
                  const delta = stat.mean_b - stat.mean_a;
                  return (
                    <tr key={`${stat.metric_name}-${stat.variant_a_id}`} className="border-t border-slate-100 hover:bg-slate-50">
                      <td className="px-4 py-2.5 font-medium text-slate-700">{stat.metric_name}</td>
                      <td className="px-4 py-2.5 text-right tabular-nums text-slate-600">{stat.mean_a.toFixed(3)}</td>
                      <td className="px-4 py-2.5 text-right tabular-nums text-slate-600">{stat.mean_b.toFixed(3)}</td>
                      <td className={`px-4 py-2.5 text-right tabular-nums font-medium ${delta > 0 ? 'text-emerald-600' : delta < 0 ? 'text-red-500' : 'text-slate-400'}`}>
                        {delta >= 0 ? '+' : ''}{delta.toFixed(3)}
                      </td>
                      <td className="px-4 py-2.5 text-right tabular-nums text-slate-500">
                        {Number.isFinite(stat.p_value) ? stat.p_value.toFixed(3) : '—'}
                      </td>
                      <td className="px-4 py-2.5 text-center">
                        {stat.is_significant ? (
                          <span className="inline-flex items-center rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-700">Yes</span>
                        ) : (
                          <span className="inline-flex items-center rounded-full bg-slate-100 px-2 py-0.5 text-[11px] text-slate-500">No</span>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      {/* Per-run results table */}
      {detailedRuns.some((r) => r.grade) && (
        <Card>
          <CardHeader title="Individual runs" description="Score breakdown per run" />
          <div className="overflow-auto rounded-lg border border-slate-200">
            <table className="min-w-full text-xs">
              <thead>
                <tr className="border-b border-slate-200 bg-slate-50 text-slate-500">
                  <th className="px-4 py-2.5 text-left font-medium">Variant</th>
                  <th className="px-4 py-2.5 text-left font-medium">Run</th>
                  <th className="px-4 py-2.5 text-left font-medium">Status</th>
                  <th className="px-4 py-2.5 text-right font-medium">Composite</th>
                  <th className="px-4 py-2.5 text-right font-medium">Test pass</th>
                  <th className="px-4 py-2.5 text-right font-medium">Lint</th>
                  <th className="px-4 py-2.5 text-right font-medium">Token eff.</th>
                  <th className="px-4 py-2.5 text-right font-medium">Duration</th>
                  <th className="px-4 py-2.5 text-left font-medium">Tests</th>
                </tr>
              </thead>
              <tbody>
                {detailedRuns.map((run) => {
                  const variant = variants.find((v) => v.id === run.variant_id);
                  const hasTests = (run.grade?.test_results?.length ?? 0) > 0;
                  const rowKey = run.id;
                  return [
                    <tr key={rowKey} className="border-t border-slate-100 hover:bg-slate-50">
                      <td className="px-4 py-2.5 font-medium text-slate-700">{variant?.name ?? '—'}</td>
                      <td className="px-4 py-2.5 text-slate-500">#{run.run_number}</td>
                      <td className="px-4 py-2.5">
                        <StatusPill status={run.status} />
                      </td>
                      <td className="px-4 py-2.5 text-right tabular-nums font-semibold text-slate-800">
                        {run.grade ? run.grade.composite_score.toFixed(3) : '—'}
                      </td>
                      <td className="px-4 py-2.5 text-right tabular-nums text-slate-600">
                        {run.grade ? fmtPct(run.grade.test_pass_rate) : '—'}
                      </td>
                      <td className="px-4 py-2.5 text-right tabular-nums text-slate-600">
                        {run.grade ? fmtPct(run.grade.lint_score) : '—'}
                      </td>
                      <td className="px-4 py-2.5 text-right tabular-nums text-slate-600">
                        {run.grade ? run.grade.token_efficiency.toFixed(3) : '—'}
                      </td>
                      <td className="px-4 py-2.5 text-right tabular-nums text-slate-500">
                        {run.duration_seconds ? `${run.duration_seconds.toFixed(1)}s` : '—'}
                      </td>
                      <td className="px-4 py-2.5">
                        {hasTests ? (
                          <button
                            className="text-indigo-600 hover:underline"
                            onClick={() => setExpandedTest(expandedTest === rowKey ? null : rowKey)}
                          >
                            {expandedTest === rowKey ? 'hide' : `${run.grade!.test_results!.length} tests`}
                          </button>
                        ) : '—'}
                      </td>
                    </tr>,
                    expandedTest === rowKey && (
                      <tr key={`${rowKey}-tests`}>
                        <td colSpan={9} className="bg-slate-50 px-4 py-3">
                          <div className="space-y-1.5">
                            {run.grade!.test_results!.map((t, ti) => (
                              <div key={ti} className={`flex items-start gap-2 rounded-lg border px-3 py-2 text-xs ${t.passed ? 'border-emerald-200 bg-emerald-50' : 'border-red-200 bg-red-50'}`}>
                                <span className={`mt-0.5 shrink-0 font-mono font-bold ${t.passed ? 'text-emerald-600' : 'text-red-500'}`}>
                                  {t.passed ? '✓' : '✗'}
                                </span>
                                <div>
                                  <div className="font-medium text-slate-700">{t.name}</div>
                                  {t.output && <pre className="mt-1 whitespace-pre-wrap text-slate-500">{t.output.slice(0, 400)}</pre>}
                                </div>
                              </div>
                            ))}
                          </div>
                        </td>
                      </tr>
                    ),
                  ];
                })}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      {/* Context config cards */}
      {variants.length > 0 && (
        <Card>
          <CardHeader title="Context configuration" description="Artifacts and extensions per variant" />
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {variants.map((variant, i) => (
              <ContextCard key={variant.id} variant={variant} color={VARIANT_COLORS[i % VARIANT_COLORS.length]} />
            ))}
          </div>
        </Card>
      )}
    </div>
  );
}

function HeroCard({ label, value, sub, accent }: { label: string; value: string; sub: string; accent: 'indigo' | 'violet' | 'emerald' | 'amber' }) {
  const colors: Record<string, string> = {
    indigo: 'from-indigo-500/10 to-indigo-500/5 border-indigo-200/60',
    violet: 'from-violet-500/10 to-violet-500/5 border-violet-200/60',
    emerald: 'from-emerald-500/10 to-emerald-500/5 border-emerald-200/60',
    amber: 'from-amber-500/10 to-amber-500/5 border-amber-200/60',
  };
  const textColors: Record<string, string> = {
    indigo: 'text-indigo-700',
    violet: 'text-violet-700',
    emerald: 'text-emerald-700',
    amber: 'text-amber-700',
  };
  return (
    <div className={`rounded-xl border bg-gradient-to-br p-4 ${colors[accent]}`}>
      <div className="text-xs font-medium uppercase tracking-wide text-slate-500">{label}</div>
      <div className={`mt-1.5 text-2xl font-bold tabular-nums ${textColors[accent]}`}>{value}</div>
      <div className="mt-0.5 text-xs text-slate-400">{sub}</div>
    </div>
  );
}

type VariantSummary = {
  id: string;
  name: string;
  isControl: boolean;
  completed: number;
  failed: number;
  total: number;
  avgComposite: number;
  avgTestPass: number;
  avgTokenEff: number;
  avgDuration: number;
  bestComposite: number;
};

function VariantSummaryCard({ summary, color }: { summary: VariantSummary; color: string }) {
  const pct = summary.total > 0 ? (summary.completed / summary.total) * 100 : 0;
  return (
    <div className="rounded-xl border border-slate-200 bg-white p-4">
      <div className="flex items-center gap-2">
        <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: color }} />
        <span className="text-sm font-semibold text-slate-800">{summary.name}</span>
        {summary.isControl && (
          <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-slate-500">control</span>
        )}
      </div>
      <div className="mt-3 grid grid-cols-2 gap-x-4 gap-y-2 text-xs">
        <MetricRow label="Composite" value={summary.avgComposite.toFixed(3)} />
        <MetricRow label="Test pass" value={fmtPct(summary.avgTestPass)} />
        <MetricRow label="Token eff." value={summary.avgTokenEff.toFixed(3)} />
        <MetricRow label="Best" value={summary.bestComposite.toFixed(3)} highlight />
        <MetricRow label="Avg duration" value={`${summary.avgDuration.toFixed(1)}s`} />
        <MetricRow label="Runs" value={`${summary.completed}/${summary.total}`} />
      </div>
      <div className="mt-3">
        <div className="mb-1 flex justify-between text-[10px] text-slate-400">
          <span>completion</span>
          <span>{pct.toFixed(0)}%</span>
        </div>
        <div className="h-1.5 rounded-full bg-slate-100">
          <div className="h-1.5 rounded-full transition-all" style={{ width: `${pct}%`, backgroundColor: color }} />
        </div>
      </div>
    </div>
  );
}

function MetricRow({ label, value, highlight }: { label: string; value: string; highlight?: boolean }) {
  return (
    <div className="flex items-baseline justify-between gap-2">
      <span className="text-slate-400">{label}</span>
      <span className={`tabular-nums ${highlight ? 'font-semibold text-slate-800' : 'text-slate-600'}`}>{value}</span>
    </div>
  );
}

function ContextCard({ variant, color }: { variant: Variant; color: string }) {
  const artifacts = variant.artifact_versions ?? [];
  const extensions = artifacts.filter((a) => a.source_kind === 'catalog_extension');
  const customFiles = artifacts.filter((a) => a.source_kind !== 'catalog_extension');
  return (
    <div className="rounded-lg border border-slate-200 p-3">
      <div className="flex items-center gap-2">
        <span className="h-2 w-2 rounded-full" style={{ backgroundColor: color }} />
        <span className="text-xs font-semibold text-slate-700">{variant.name}</span>
        {variant.is_control && (
          <span className="ml-auto rounded-full bg-slate-100 px-1.5 py-0.5 text-[10px] text-slate-400">control</span>
        )}
      </div>
      {extensions.length > 0 && (
        <div className="mt-2 flex flex-wrap gap-1">
          {extensions.map((e) => (
            <span key={e.id} className="rounded-full bg-indigo-50 px-2 py-0.5 text-[10px] font-medium text-indigo-600">
              {e.display_name ?? e.source_ref ?? 'extension'}
            </span>
          ))}
        </div>
      )}
      {customFiles.length > 0 && (
        <div className="mt-1.5 text-[11px] text-slate-400">
          {customFiles.length} custom file{customFiles.length !== 1 ? 's' : ''}
        </div>
      )}
      {artifacts.length === 0 && (
        <div className="mt-1.5 text-[11px] text-slate-400">No context artifacts</div>
      )}
    </div>
  );
}

function StatusPill({ status }: { status: string }) {
  const map: Record<string, string> = {
    completed: 'bg-emerald-50 text-emerald-700 border-emerald-200',
    failed: 'bg-red-50 text-red-600 border-red-200',
    running: 'bg-amber-50 text-amber-600 border-amber-200',
    pending: 'bg-slate-100 text-slate-500 border-slate-200',
  };
  return (
    <span className={`inline-flex rounded-full border px-2 py-0.5 text-[10px] font-medium capitalize ${map[status] ?? map.pending}`}>
      {status}
    </span>
  );
}

function EmptyChart() {
  return (
    <div className="flex h-64 items-center justify-center rounded-lg border border-dashed border-slate-200 text-sm text-slate-400">
      Not enough data yet
    </div>
  );
}

function RunScatterTooltip({ active, payload }: { active?: boolean; payload?: Array<{ payload: { run: number; variant: string; x: number; y: number } }> }) {
  if (!active || !payload?.length) return null;
  const d = payload[0].payload;
  return (
    <div className="rounded-lg border border-slate-200 bg-white p-3 shadow-md">
      <div className="text-xs font-semibold text-slate-700">{d.variant} · Run #{d.run}</div>
      <div className="mt-1 text-xs text-slate-500">Composite: <span className="font-medium text-slate-800">{d.y.toFixed(3)}</span></div>
      <div className="text-xs text-slate-500">Token eff.: <span className="font-medium text-slate-800">{d.x.toFixed(3)}</span></div>
    </div>
  );
}

function fmtPct(v: number) {
  return `${(v * 100).toFixed(1)}%`;
}

function buildVariantSummaries(runs: Run[], variants: Variant[]): VariantSummary[] {
  return variants.map((variant) => {
    const vRuns = runs.filter((r) => r.variant_id === variant.id);
    const grades = vRuns.map((r) => r.grade).filter((g): g is Grade => Boolean(g));
    return {
      id: variant.id,
      name: variant.name,
      isControl: variant.is_control,
      completed: vRuns.filter((r) => r.status === 'completed').length,
      failed: vRuns.filter((r) => r.status === 'failed').length,
      total: vRuns.length,
      avgComposite: avg(grades, 'composite_score'),
      avgTestPass: avg(grades, 'test_pass_rate'),
      avgTokenEff: avg(grades, 'token_efficiency'),
      avgDuration: avgDuration(vRuns),
      bestComposite: Math.max(0, ...grades.map((g) => g.composite_score ?? 0)),
    };
  });
}

function buildRadarData(summaries: VariantSummary[]) {
  const metrics = Object.keys(METRIC_LABELS);
  const metricKeys: Array<keyof VariantSummary> = ['avgComposite', 'avgTestPass', 'avgTokenEff'];
  const shortMetrics = ['Composite', 'Test pass', 'Token eff.'];
  const points = shortMetrics.map((metric, mi) => {
    const point: Record<string, string | number> = { metric };
    for (const s of summaries) {
      point[s.name] = Number((s[metricKeys[mi]] as number).toFixed(3));
    }
    return point;
  });
  void metrics;
  return { points, metrics: shortMetrics, variantNames: summaries.map((s) => s.name) };
}

function buildBarData(stats: ExperimentStat[], variantNames: Map<string, string>) {
  if (stats.length === 0) return [];
  return stats.map((stat) => {
    const point: Record<string, string | number> = { metric: stat.metric_name.replace('Context utilization', 'Context util.').replace('Token efficiency', 'Token eff.') };
    const nameA = variantNames.get(stat.variant_a_id) ?? 'A';
    const nameB = variantNames.get(stat.variant_b_id) ?? 'B';
    point[nameA] = Number(stat.mean_a.toFixed(3));
    point[nameB] = Number(stat.mean_b.toFixed(3));
    return point;
  });
}

function buildScatterData(runs: Run[], variants: Variant[]) {
  const series = variants.map((variant) => ({
    name: variant.name,
    points: runs
      .filter((r) => r.variant_id === variant.id && r.grade)
      .map((r) => ({
        x: r.grade!.token_efficiency,
        y: r.grade!.composite_score,
        run: r.run_number,
        variant: variant.name,
      })),
  })).filter((s) => s.points.length > 0);
  return { series };
}

function buildFallbackStats(runs: Run[], variantIDs: string[]): ExperimentStat[] {
  if (variantIDs.length < 2) return [];
  const [variantAID, variantBID] = variantIDs;
  const variantARuns = runs.filter((r) => r.variant_id === variantAID && r.grade);
  const variantBRuns = runs.filter((r) => r.variant_id === variantBID && r.grade);
  if (!variantARuns.length || !variantBRuns.length) return [];

  const metrics: Array<{ key: keyof Grade; label: string }> = [
    { key: 'composite_score', label: 'Composite score' },
    { key: 'test_pass_rate', label: 'Test pass rate' },
    { key: 'token_efficiency', label: 'Token efficiency' },
    { key: 'context_utilization', label: 'Context utilization' },
  ];

  return metrics.map((m) => ({
    metric_name: m.label,
    variant_a_id: variantAID,
    variant_b_id: variantBID,
    mean_a: avgMetric(variantARuns, m.key),
    mean_b: avgMetric(variantBRuns, m.key),
    p_value: Number.NaN,
    is_significant: false,
  }));
}

function avg(grades: Grade[], key: keyof Grade) {
  const vals = grades.map((g) => g[key]).filter((v): v is number => typeof v === 'number');
  return vals.length ? vals.reduce((s, v) => s + v, 0) / vals.length : 0;
}

function avgMetric(runs: Run[], key: keyof Grade) {
  return avg(runs.map((r) => r.grade).filter((g): g is Grade => Boolean(g)), key);
}

function avgDuration(runs: Run[]) {
  const vals = runs.map((r) => r.duration_seconds).filter((v): v is number => typeof v === 'number');
  return vals.length ? vals.reduce((s, v) => s + v, 0) / vals.length : 0;
}
