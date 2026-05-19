import { SymptomGlyph, TurnCard, type BlockKind as CardBlockKind } from '../system';
import type { TurnGroup } from './group-turns';
import type { EvidenceForTurn } from '../../lib/symptom-evidence';
import { renderBlock } from './blocks';

/**
 * TurnGroupCard renders one "decision" — the parent_turn_index group
 * that AssignTurnGrouping (engine/pkg/executor/grouping.go) produced.
 * It composes the design-system primitive TurnCard with one block
 * renderer per child member of the group.
 *
 * The Inspector V2 list renders one of these per group; clicking the
 * card focuses it in the right-side detail pane.
 *
 * Click target: a <div role="button"> wraps the card surface, with
 * keyboard handlers for Enter / Space so non-mouse users can focus
 * turns. The TurnCard's internal expand/collapse button must NOT be
 * nested inside an outer <button> — that is invalid HTML and breaks
 * the inner button's click handling.
 */

interface TurnGroupCardProps {
  group: TurnGroup;
  defaultCollapsed?: boolean;
  onFocus?: (parentTurnIndex: number) => void;
  /**
   * Pre-indexed failure-label evidence (see lib/symptom-evidence.ts).
   * Keyed by `turn_index`; the card surfaces a SymptomGlyph for each
   * matching child block. Optional — passing nothing simply renders
   * no glyphs.
   */
  evidenceByTurn?: Map<number, EvidenceForTurn>;
}

/**
 * Map the wider transcript-level BlockKind (which includes '') to the
 * narrower card visual kind via groupTurns' representativeKind. '' →
 * 'text' is the legacy fallback for unstamped data.
 */
function cardKindForGroup(group: TurnGroup): CardBlockKind {
  const repr = group.representativeKind;
  if (repr === 'thinking' || repr === 'text' || repr === 'tool_use' || repr === 'tool_result' || repr === 'system') {
    return repr;
  }
  return 'text';
}

export function TurnGroupCard({
  group,
  defaultCollapsed,
  onFocus,
  evidenceByTurn,
}: TurnGroupCardProps) {
  const fire = () => onFocus?.(group.parentTurnIndex);

  // Collect evidence entries for any block in this group. A single
  // turn group can have multiple evidence findings — render the
  // glyphs in turn_index order so the visual ordering matches the
  // transcript timeline.
  const glyphs = evidenceByTurn
    ? group.blocks
        .map((b) => ({ idx: b.turn_index ?? -1, entry: evidenceByTurn.get(b.turn_index ?? -1) }))
        .filter((x): x is { idx: number; entry: EvidenceForTurn } => x.entry !== undefined)
        .flatMap(({ idx, entry }) =>
          entry.spans.map((span) => ({
            key: `${idx}-${span.code}`,
            code: span.code,
            confidence: entry.confidence,
            rationale: entry.rationale,
          })),
        )
    : [];

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={fire}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          fire();
        }
      }}
      className="block w-full cursor-pointer text-left focus:outline-none focus-visible:ring-2 focus-visible:ring-accent"
      aria-label={`Focus turn ${group.parentTurnIndex}`}
    >
      <TurnCard
        turnIndex={group.parentTurnIndex}
        blockKind={cardKindForGroup(group)}
        toolName={group.toolName}
        defaultCollapsed={defaultCollapsed}
        symptomGlyph={
          glyphs.length === 0
            ? undefined
            : (
                <span className="flex flex-wrap gap-1">
                  {glyphs.map((g) => (
                    <SymptomGlyph
                      key={g.key}
                      code={g.code}
                      confidence={g.confidence}
                      rationale={g.rationale}
                    />
                  ))}
                </span>
              )
        }
      >
        <div className="space-y-2">
          {group.blocks.map((block, idx) => (
            <div key={block.turn_index ?? idx}>{renderBlock(block)}</div>
          ))}
        </div>
      </TurnCard>
    </div>
  );
}
