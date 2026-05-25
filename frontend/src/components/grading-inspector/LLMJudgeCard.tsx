import { useState } from 'react';
import type { Grade } from '../../lib/types';
import { Card, CardHeader } from '../ui/card';

export function LLMJudgeCard({ grade }: { grade: Grade }) {
  const [showRaw, setShowRaw] = useState(false);
  const rationale = extractRationale(grade.raw_judge_responses);
  const dims = [
    { label: 'Correctness', value: grade.judge_correctness },
    { label: 'Maintainability', value: grade.judge_maintainability ?? 0 },
    { label: 'Completeness', value: grade.judge_completeness ?? 0 },
    { label: 'Best practices', value: grade.judge_best_practices ?? 0 },
    { label: 'Error handling', value: grade.judge_error_handling ?? 0 },
  ];
  return (
    <Card>
      <CardHeader
        title="LLM-as-judge rubric"
        description="Five-dimension judgment from the configured judge model."
      />
      <div className="space-y-1">
        {dims.map((d) => (
          <DimRow key={d.label} label={d.label} value={d.value} />
        ))}
      </div>
      {grade.judge_irr_alpha != null && grade.judge_irr_alpha > 0 && (
        <div className="mt-2 text-xs text-fg-muted">
          Inter-rater α: <span className="font-mono">{grade.judge_irr_alpha.toFixed(2)}</span>
        </div>
      )}
      {rationale && (
        <div className="mt-3 border-t border-border pt-3">
          <div className="mb-1 text-xs uppercase tracking-wider text-fg-muted">Rationale</div>
          <p className="text-sm text-fg">{rationale}</p>
        </div>
      )}
      {(grade.raw_judge_responses?.length ?? 0) > 0 && (
        <div className="mt-3 border-t border-border pt-3">
          <button
            className="text-xs text-fg-muted underline"
            onClick={() => setShowRaw((v) => !v)}
          >
            {showRaw ? 'hide raw response' : 'show raw response (debug)'}
          </button>
          {showRaw && (
            <pre className="mt-2 max-h-64 overflow-auto whitespace-pre-wrap rounded bg-bg-elev-2 p-2 text-xs text-fg-muted">
              {JSON.stringify(grade.raw_judge_responses, null, 2)}
            </pre>
          )}
        </div>
      )}
    </Card>
  );
}

function DimRow({ label, value }: { label: string; value: number }) {
  const pct = Math.max(0, Math.min(100, (value / 10) * 100));
  return (
    <div className="flex items-center gap-3 text-sm">
      <div className="w-32 text-fg-muted">{label}</div>
      <div className="flex h-2 flex-1 overflow-hidden rounded bg-bg-elev-2">
        <div className="h-full bg-primary" style={{ width: `${pct}%` }} />
      </div>
      <div className="w-12 text-right font-mono text-fg">{value.toFixed(2)}</div>
    </div>
  );
}

function extractRationale(raw: string[] | undefined): string | null {
  if (!raw || raw.length === 0) return null;
  for (const entry of raw) {
    try {
      const parsed = JSON.parse(entry);
      if (typeof parsed?.rationale === 'string' && parsed.rationale.length > 0) {
        return parsed.rationale;
      }
    } catch {
      // not JSON (probably a sentinel like "judge_unavailable: ..."); skip
    }
  }
  return null;
}
