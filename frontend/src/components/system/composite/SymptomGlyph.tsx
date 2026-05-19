import { cn } from '../../../lib/utils';

/**
 * SymptomGlyph renders the failure-code badge attached to a turn in
 * Inspector V2 — a 2-letter abbreviation in a colored pill, with the
 * full code + confidence + rationale on hover. Codes come from the
 * AgentDx FailureCode taxonomy (see pkg/diagnostic/taxonomy.go).
 *
 * 2-letter abbreviations are chosen so the glyph stays a fixed width
 * across every code, keeping turn-list alignment stable. NONE renders
 * a muted em-dash so the layout doesn't shift between turns that have
 * a failure tag and turns that don't.
 *
 * Tone mapping clusters codes by severity:
 *   - Hallucinations / scope drift → danger
 *   - Premature stops / silent skips → warning
 *   - Environment / dependency issues → info
 *   - NONE → neutral
 */

export type FailureCode =
  | 'NONE'
  | 'HAL_API'
  | 'HAL_FILE'
  | 'DEP_MISS'
  | 'STOP_EARLY'
  | 'STOP_GIVEUP'
  | 'LOOP_INF'
  | 'WRONG_ABS'
  | 'MISREAD'
  | 'ENV_ERR'
  | 'SCOPE_DRIFT'
  | 'TIMEOUT'
  | 'SILENT_SKIP';

interface SymptomGlyphProps {
  code: FailureCode;
  confidence: number;
  rationale?: string;
  className?: string;
}

// 2-letter abbreviations, stable per code.
const abbrev: Record<FailureCode, string> = {
  NONE: '—',
  HAL_API: 'HA',
  HAL_FILE: 'HF',
  DEP_MISS: 'DM',
  STOP_EARLY: 'SE',
  STOP_GIVEUP: 'SG',
  LOOP_INF: 'LP',
  WRONG_ABS: 'WA',
  MISREAD: 'MR',
  ENV_ERR: 'EE',
  SCOPE_DRIFT: 'SD',
  TIMEOUT: 'TO',
  SILENT_SKIP: 'SS',
};

const tone: Record<FailureCode, string> = {
  NONE: 'bg-bg-elev-2 text-fg-subtle border-border',
  HAL_API: 'bg-danger/15 text-danger border-danger/40',
  HAL_FILE: 'bg-danger/15 text-danger border-danger/40',
  SCOPE_DRIFT: 'bg-danger/15 text-danger border-danger/40',
  WRONG_ABS: 'bg-danger/15 text-danger border-danger/40',
  STOP_EARLY: 'bg-warning/15 text-warning border-warning/40',
  STOP_GIVEUP: 'bg-warning/15 text-warning border-warning/40',
  SILENT_SKIP: 'bg-warning/15 text-warning border-warning/40',
  LOOP_INF: 'bg-warning/15 text-warning border-warning/40',
  MISREAD: 'bg-warning/15 text-warning border-warning/40',
  DEP_MISS: 'bg-info/15 text-info border-info/40',
  ENV_ERR: 'bg-info/15 text-info border-info/40',
  TIMEOUT: 'bg-info/15 text-info border-info/40',
};

export function SymptomGlyph({ code, confidence, rationale, className }: SymptomGlyphProps) {
  const titleParts = [`${code} (confidence ${confidence.toFixed(2)})`];
  if (rationale) titleParts.push(rationale);

  return (
    <span
      role="img"
      aria-label={`${code} (confidence ${confidence.toFixed(2)})`}
      title={titleParts.join(' — ')}
      className={cn(
        'inline-flex h-5 min-w-5 items-center justify-center rounded-sm border px-1 font-mono text-[10px] font-semibold leading-none',
        tone[code],
        className,
      )}
    >
      {abbrev[code]}
    </span>
  );
}
