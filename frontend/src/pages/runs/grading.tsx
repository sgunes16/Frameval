import { useParams } from 'react-router-dom';

import {
  CodeGradingCard,
  FailureClassifierCard,
  GradingHeader,
  LLMJudgeCard,
  ProcessMetricsCard,
} from '../../components/grading-inspector';
import { ErrorState, LoadingSkeleton } from '../../components/system';
import { useDiagnostic, useGrade, useRegradeRun, useRun } from '../../lib/hooks';
import type { Grade } from '../../lib/types';

/**
 * Run Grading Inspector — `/runs/:id/grading`.
 *
 * Symmetric to the Turn Inspector (`/runs/:id/inspect`) but for the
 * grading pipeline rather than execution flow. Renders five Cards:
 * header (composite + regrade), code grader, process metrics, LLM
 * judge rationale, and failure classifier evidence.
 */
export function RunGradingPage() {
  const { id } = useParams<{ id: string }>();
  const runQuery = useRun(id);
  const gradeQuery = useGrade(id, runQuery.data?.status);
  const diagnosticQuery = useDiagnostic(id);
  const regrade = useRegradeRun();

  if (runQuery.isError) {
    return (
      <ErrorState
        title="Could not load run"
        description="The engine returned an error or the run doesn't exist."
        onRetry={() => {
          runQuery.refetch();
          gradeQuery.refetch();
        }}
      />
    );
  }

  if (runQuery.isLoading) {
    return (
      <div className="space-y-2">
        <LoadingSkeleton variant="row" count={6} />
      </div>
    );
  }

  const run = runQuery.data;
  if (!run || !id) {
    return <ErrorState title="Run not found" description="No data to display." />;
  }

  // grade may be undefined while the LLM judge is in flight. Render whatever's available.
  const grade = gradeQuery.data;
  const isGrading = !grade || !grade.judge_scores || Object.keys(grade.judge_scores).length === 0;

  return (
    <div className="space-y-4">
      <GradingHeader
        run={run}
        grade={grade ?? ({} as Grade)}
        onRegrade={() => regrade.mutate(id)}
        regradeBusy={regrade.isPending}
      />
      <CodeGradingCard grade={grade ?? ({} as Grade)} />
      <ProcessMetricsCard grade={grade ?? ({} as Grade)} />
      <LLMJudgeCard grade={grade ?? ({} as Grade)} isGrading={isGrading} />
      <FailureClassifierCard diagnostic={diagnosticQuery.data} runId={id} />
    </div>
  );
}
