import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';

import { RunGradingPage } from './grading';

// Stub the hooks module — only what RunGradingPage consumes.
vi.mock('../../lib/hooks', () => ({
  useRun: () => ({
    data: { id: 'r-1', variant_id: 'v-1', status: 'completed' },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
  useGrade: () => ({
    data: {
      composite_score: 6.5,
      source: 'grader',
      test_pass_rate: 1.0,
      test_pass_count: 3,
      test_fail_count: 0,
      lint_score: 7,
      type_check_pass: true,
      file_state_valid: true,
      token_efficiency: 0.8,
      context_utilization: 0.6,
      spec_instruction_compliance: 9,
      test_results: [{ name: 'test_one', passed: true, output: 'ok' }],
      judge_correctness: 8,
      judge_maintainability: 7,
      judge_completeness: 8,
      judge_best_practices: 6,
      judge_error_handling: 5,
      judge_irr_alpha: 0,
      raw_judge_responses: [
        'dim=correctness;{"score":8.0,"rationale":"solid solution on correctness"}',
        'dim=maintainability;{"score":7.0,"rationale":"clean names"}',
        'dim=completeness;{"score":9.0,"rationale":"all requirements covered"}',
        'dim=best_practices;{"score":6.0,"rationale":"sync lock in async code"}',
        'dim=error_handling;{"score":5.0,"rationale":"happy path only"}',
      ],
      turn_count: 5,
      total_tokens: 1200,
    },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
  useDiagnostic: () => ({ data: undefined }),
  useRegradeRun: () => ({ mutate: vi.fn(), isPending: false }),
}));

function renderPage() {
  const qc = new QueryClient();
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/runs/r-1/grading']}>
        <Routes>
          <Route path="/runs/:id/grading" element={<RunGradingPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('RunGradingPage', () => {
  it('renders composite score, judge dimension labels, rationale, and Regrade button', () => {
    renderPage();
    expect(screen.getByText(/Composite score: 6\.50/)).toBeInTheDocument();
    expect(screen.getByText(/Correctness/)).toBeInTheDocument();
    expect(screen.getByText(/solid solution on correctness/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Regrade/ })).toBeInTheDocument();
  });
});
