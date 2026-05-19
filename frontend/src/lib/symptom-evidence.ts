import type { Diagnostic, EvidenceSpan, FailureCode } from './types';

/**
 * One turn's worth of failure-label evidence, pre-grouped for the
 * Inspector V2 turn cards. Carries the overall classification context
 * (primary code + confidence + rationale) alongside the per-span list
 * so the SymptomGlyph tooltip can render full context without each
 * card re-fetching the diagnostic.
 */
export interface EvidenceForTurn {
  spans: EvidenceSpan[];
  primary: FailureCode;
  confidence: number;
  rationale?: string;
}

/**
 * Build an O(1) lookup from `turn_index` to that turn's evidence
 * entry. Used by TurnGroupCard to decide whether to attach a
 * SymptomGlyph and what to render in its tooltip.
 *
 * Returns an empty map when the diagnostic is missing or has no
 * failure_label — the caller never has to null-check the result.
 */
export function buildEvidenceByTurn(
  diagnostic: Diagnostic | null | undefined,
): Map<number, EvidenceForTurn> {
  const out = new Map<number, EvidenceForTurn>();
  const label = diagnostic?.failure_label;
  if (!label?.evidence?.length) return out;

  for (const span of label.evidence) {
    const existing = out.get(span.turn_index);
    if (existing) {
      existing.spans.push(span);
    } else {
      out.set(span.turn_index, {
        spans: [span],
        primary: label.primary,
        confidence: label.confidence,
        rationale: label.rationale,
      });
    }
  }
  return out;
}
