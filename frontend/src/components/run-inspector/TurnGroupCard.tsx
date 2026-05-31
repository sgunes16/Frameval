import { SymptomGlyph } from '../system';
import type { TurnGroup } from './group-turns';
import type { EvidenceForTurn } from '../../lib/symptom-evidence';
import type { ParsedTurn } from '../../lib/types';
import { renderBlock } from './blocks';
import { cn } from '../../lib/utils';
import { roleAccent } from './role-accent';

/**
 * TurnGroupCard renders one "agent decision" — the set of ParsedTurns
 * the backend stamped with the same parent_turn_index (every event
 * opencode emitted under one messageID: assistant text + the tool
 * calls that decision spawned).
 *
 * Visual language is a flight-recorder timeline rather than a stack of
 * dashboard cards:
 *
 *   ●   read app/user_service.py
 *   │   read app/db.py
 *   │
 *   ●   edit app/user_service.py
 *   │   ┌── unified diff ──┐
 *   │   └──────────────────┘
 *   │   The fix is to add an asyncio.Lock…
 *   │
 *   ●   bash $ git log --oneline -5
 *
 * A 1px vertical rail runs through every group; the dot at the group's
 * head encodes the decision type (action / passive read / thinking /
 * error). Adjacent cards' rails visually concatenate because TurnList
 * renders them flush. Action moments (edit, write, bash) get the accent
 * color; passive reads/globs/greps stay muted. There is no card chrome
 * — no border, no header, no "Turn N · Tool · read" redundant label —
 * since the rail itself is the timeline.
 */

interface TurnGroupCardProps {
  group: TurnGroup;
  defaultCollapsed?: boolean;
  onFocus?: (parentTurnIndex: number) => void;
  evidenceByTurn?: Map<number, EvidenceForTurn>;
}

type DotTone = 'action' | 'passive' | 'thinking' | 'error' | 'neutral';

const ACTION_TOOLS = new Set(['edit', 'write', 'bash', 'shell', 'todowrite']);

function dotToneFor(group: TurnGroup): DotTone {
  if (group.blocks.some((b) => b.stage === 'error')) return 'error';
  for (const b of group.blocks) {
    if (b.block_kind !== 'tool_use') continue;
    const name = (b.tool_name ?? '').toLowerCase();
    if (ACTION_TOOLS.has(name)) return 'action';
  }
  if (group.representativeKind === 'tool_use') return 'passive';
  if (group.representativeKind === 'thinking') return 'thinking';
  if (group.representativeKind === 'system') return 'neutral';
  return 'neutral';
}

const dotClass: Record<DotTone, string> = {
  action: 'bg-accent ring-2 ring-accent/20',
  passive: 'bg-fg-subtle',
  thinking: 'bg-chart-5',
  error: 'bg-danger ring-2 ring-danger/20',
  neutral: 'bg-border',
};

function collectGlyphs(group: TurnGroup, evidenceByTurn?: Map<number, EvidenceForTurn>) {
  if (!evidenceByTurn) return [];
  const seen = new Set<number>();
  const out: Array<{
    key: string;
    code: EvidenceForTurn['spans'][number]['code'];
    confidence: number;
    rationale?: string;
  }> = [];
  for (const block of group.blocks) {
    const idx = block.turn_index;
    if (idx === undefined || seen.has(idx)) continue;
    const entry = evidenceByTurn.get(idx);
    if (!entry) continue;
    seen.add(idx);
    for (const span of entry.spans) {
      out.push({
        key: `${idx}-${span.code}`,
        code: span.code,
        confidence: entry.confidence,
        rationale: entry.rationale,
      });
    }
  }
  return out;
}

function isHidden(block: ParsedTurn): boolean {
  // The system block kind is reserved for opencode lifecycle markers
  // (step_start/step_finish) that we already strip in the parser, plus
  // the rare legacy meta-event. Skip any that survive to keep the
  // timeline focused on agent intent + action.
  return block.block_kind === 'system' && !block.content;
}

export function TurnGroupCard({
  group,
  onFocus,
  evidenceByTurn,
}: TurnGroupCardProps) {
  const fire = () => onFocus?.(group.parentTurnIndex);
  const tone = dotToneFor(group);
  const glyphs = collectGlyphs(group, evidenceByTurn);
  const blocks = group.blocks.filter((b) => !isHidden(b));

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
      aria-label={`Focus turn ${group.parentTurnIndex}`}
      className={cn(
          'group/turn relative grid w-full cursor-pointer grid-cols-[28px_1fr] gap-3 border-l-4 px-2 py-2 text-left transition focus:outline-none hover:bg-bg-elev-1/40 focus-visible:bg-bg-elev-1/60',
          roleAccent(group.role),
        )}
    >
      {/* Rail + dot. The rail spans the full card height so adjacent
          cards visually concatenate into a continuous timeline. */}
      <div className="relative">
        <span
          aria-hidden
          className="absolute left-1/2 top-0 h-full w-px -translate-x-1/2 bg-border"
        />
        <span
          aria-hidden
          className={cn(
            'absolute left-1/2 top-2 z-10 h-2.5 w-2.5 -translate-x-1/2 rounded-full',
            dotClass[tone],
          )}
        />
      </div>

      {/* Body: one block per row, no card chrome. Symptom glyphs (if
          the failure classifier flagged this step) float at the top
          right of the body column so they don't compete with content. */}
      <div className="min-w-0 space-y-1.5">
        {group.role && (
          <div className="text-xs uppercase tracking-wider text-fg-muted">
            Role: <span className="font-mono text-fg">{group.role}</span>
          </div>
        )}
        {glyphs.length > 0 && (
          <div
            role="group"
            aria-label={`${glyphs.length} symptom ${glyphs.length === 1 ? 'finding' : 'findings'}`}
            className="flex flex-wrap items-center justify-end gap-1"
          >
            {glyphs.map((g) => (
              <SymptomGlyph
                key={g.key}
                code={g.code}
                confidence={g.confidence}
                rationale={g.rationale}
              />
            ))}
          </div>
        )}
        {blocks.map((block, idx) => (
          <div key={block.turn_index ?? idx}>{renderBlock(block)}</div>
        ))}
      </div>
    </div>
  );
}
