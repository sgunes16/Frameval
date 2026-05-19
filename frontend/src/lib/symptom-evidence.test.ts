import { describe, expect, it } from 'vitest';

import { buildEvidenceByTurn, type EvidenceForTurn } from './symptom-evidence';
import type { Diagnostic, EvidenceSpan, FailureClassification } from './types';

const mkDiag = (label?: FailureClassification): Diagnostic =>
  ({
    id: 'd1',
    run_id: 'r1',
    fingerprint: {} as unknown as Diagnostic['fingerprint'],
    symptoms: {} as unknown as Diagnostic['symptoms'],
    recovery: {} as unknown as Diagnostic['recovery'],
    failure_label: label ?? null,
  } as Diagnostic);

const span = (turn_index: number, code: EvidenceSpan['code'], quote = ''): EvidenceSpan => ({
  turn_index,
  code,
  quote,
});

describe('buildEvidenceByTurn', () => {
  it('returns an empty map when the diagnostic is null or unlabelled', () => {
    expect(buildEvidenceByTurn(null).size).toBe(0);
    expect(buildEvidenceByTurn(undefined).size).toBe(0);
    expect(buildEvidenceByTurn(mkDiag()).size).toBe(0);
  });

  it('groups EvidenceSpan entries by turn_index', () => {
    const diag = mkDiag({
      primary: 'HAL_API',
      confidence: 0.92,
      rationale: 'agent fabricated an API call',
      evidence: [span(3, 'HAL_API', 'invented endpoint'), span(7, 'SCOPE_DRIFT')],
    });
    const out = buildEvidenceByTurn(diag);
    expect(out.size).toBe(2);
    expect(out.get(3)?.spans).toHaveLength(1);
    expect(out.get(7)?.spans[0]?.code).toBe('SCOPE_DRIFT');
  });

  it('stamps each entry with the primary classification context for tooltip rendering', () => {
    const diag = mkDiag({
      primary: 'STOP_EARLY',
      confidence: 0.71,
      rationale: 'gave up after retry',
      evidence: [span(5, 'STOP_EARLY')],
    });
    const entry = buildEvidenceByTurn(diag).get(5) as EvidenceForTurn;
    expect(entry.primary).toBe('STOP_EARLY');
    expect(entry.confidence).toBe(0.71);
    expect(entry.rationale).toBe('gave up after retry');
  });

  it('keeps multiple spans for the same turn distinct (e.g. two findings on one turn)', () => {
    const diag = mkDiag({
      primary: 'SCOPE_DRIFT',
      confidence: 0.8,
      evidence: [span(4, 'SCOPE_DRIFT'), span(4, 'STOP_EARLY')],
    });
    const entry = buildEvidenceByTurn(diag).get(4) as EvidenceForTurn;
    expect(entry.spans.map((s) => s.code)).toEqual(['SCOPE_DRIFT', 'STOP_EARLY']);
  });
});
