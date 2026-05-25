import { useState } from 'react';
import type { Grade } from '../../lib/types';
import { Badge } from '../ui/badge';
import { Card, CardHeader } from '../ui/card';

export function CodeGradingCard({ grade }: { grade: Grade }) {
  const tests = grade.test_results ?? [];
  return (
    <Card>
      <CardHeader
        title="Code grading"
        description="Deterministic test runner + lint + type-check."
      />
      <div className="grid gap-2 text-sm">
        <Row label="Test pass rate" value={`${(grade.test_pass_rate * 100).toFixed(0)}%`} />
        <Row label="Tests" value={`${grade.test_pass_count ?? 0} / ${(grade.test_pass_count ?? 0) + (grade.test_fail_count ?? 0)} passed`} />
        <Row label="Lint score" value={`${grade.lint_score?.toFixed(1) ?? '—'} / 10`} />
        <Row label="Type check" value={grade.type_check_pass ? 'pass' : 'fail'} />
        <Row label="File state" value={grade.file_state_valid ? 'ok' : 'broken'} />
      </div>
      {tests.length > 0 && (
        <div className="mt-3 border-t border-border pt-3">
          <div className="mb-2 text-xs uppercase tracking-wider text-fg-muted">Per-test</div>
          <ul className="space-y-1">
            {tests.map((t, i) => (
              <TestRow key={i} test={t} />
            ))}
          </ul>
        </div>
      )}
    </Card>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between gap-2 border-b border-border/40 py-1 last:border-0">
      <span className="text-fg-muted">{label}</span>
      <span className="font-mono text-fg">{value}</span>
    </div>
  );
}

function TestRow({ test }: { test: { name: string; passed: boolean; output: string } }) {
  const [open, setOpen] = useState(false);
  return (
    <li className="rounded border border-border bg-bg-elev-1 p-2">
      <div className="flex items-center justify-between gap-2">
        <span className="truncate font-mono text-xs">{test.name}</span>
        <div className="flex items-center gap-2">
          {test.passed ? <Badge tone="success">pass</Badge> : <Badge tone="danger">fail</Badge>}
          {test.output && (
            <button
              className="text-xs text-fg-muted underline"
              onClick={() => setOpen((v) => !v)}
            >
              {open ? 'hide' : 'output'}
            </button>
          )}
        </div>
      </div>
      {open && test.output && (
        <pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap rounded bg-bg-elev-2 p-2 text-xs text-fg-muted">
          {test.output}
        </pre>
      )}
    </li>
  );
}
