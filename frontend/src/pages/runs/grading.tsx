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
  const gradeQuery = useGrade(id);
  const diagnosticQuery = useDiagnostic(id);
  const regrade = useRegradeRun();

  if (runQuery.isError || gradeQuery.isError) {
    return (
      <ErrorState
        title="Could not load grading data"
        description="The engine returned an error or the grade doesn't exist."
        onRetry={() => {
          runQuery.refetch();
          gradeQuery.refetch();
        }}
      />
    );
  }

  if (runQuery.isLoading || gradeQuery.isLoading) {
    return (
      <div className="space-y-2">
        <LoadingSkeleton variant="row" count={6} />
      </div>
    );
  }

  const run = runQuery.data;
  const grade = gradeQuery.data;
  if (!run || !grade || !id) {
    return <ErrorState title="Run not found" description="No data to display." />;
  }

  return (
    <div className="space-y-4">
      <GradingHeader
        run={run}
        grade={grade}
        onRegrade={() => regrade.mutate(id)}
        regradeBusy={regrade.isPending}
      />
      <CodeGradingCard grade={grade} />
      <ProcessMetricsCard grade={grade} />
      <LLMJudgeCard grade={grade} />
      <FailureClassifierCard diagnostic={diagnosticQuery.data} runId={id} />
    </div>
  );
}
