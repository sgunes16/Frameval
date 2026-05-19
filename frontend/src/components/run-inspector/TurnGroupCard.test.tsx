import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';

import { TurnGroupCard } from './TurnGroupCard';
import type { TurnGroup } from './group-turns';
import type { EvidenceForTurn } from '../../lib/symptom-evidence';

const group: TurnGroup = {
  parentTurnIndex: 4,
  blocks: [
    {
      role: 'assistant',
      content: 'reasoning',
      block_kind: 'thinking',
      turn_index: 4,
      parent_turn_index: 4,
    },
    {
      role: 'assistant',
      content: 'edit',
      block_kind: 'tool_use',
      turn_index: 5,
      parent_turn_index: 4,
      tool_name: 'Edit',
    },
  ],
  representativeKind: 'tool_use',
  toolName: 'Edit',
};

describe('TurnGroupCard', () => {
  it('renders without a SymptomGlyph when no evidence applies', () => {
    render(<TurnGroupCard group={group} />);
    // The glyph carries role=img with the FailureCode label; absence
    // is what we're asserting here.
    expect(screen.queryByRole('img')).not.toBeInTheDocument();
  });

  it('attaches a SymptomGlyph for a turn_index in evidenceByTurn', () => {
    const evidenceByTurn = new Map<number, EvidenceForTurn>([
      [
        5,
        {
          spans: [{ code: 'HAL_API', turn_index: 5, quote: 'invented endpoint' }],
          primary: 'HAL_API',
          confidence: 0.92,
          rationale: 'agent invented an API call',
        },
      ],
    ]);
    render(<TurnGroupCard group={group} evidenceByTurn={evidenceByTurn} />);
    const glyph = screen.getByRole('img', { name: /HAL_API/i });
    expect(glyph).toBeInTheDocument();
    expect(glyph).toHaveAttribute('title', expect.stringContaining('agent invented'));
  });

  it('renders multiple glyphs when one turn has multiple findings', () => {
    const evidenceByTurn = new Map<number, EvidenceForTurn>([
      [
        5,
        {
          spans: [
            { code: 'HAL_API', turn_index: 5, quote: '' },
            { code: 'SCOPE_DRIFT', turn_index: 5, quote: '' },
          ],
          primary: 'HAL_API',
          confidence: 0.8,
        },
      ],
    ]);
    render(<TurnGroupCard group={group} evidenceByTurn={evidenceByTurn} />);
    expect(screen.getByRole('img', { name: /HAL_API/i })).toBeInTheDocument();
    expect(screen.getByRole('img', { name: /SCOPE_DRIFT/i })).toBeInTheDocument();
  });

  it('deduplicates evidence when two blocks share a turn_index', () => {
    // tool_use and its matching tool_result commonly share the same
    // turn_index — collecting evidence per-block would render the
    // glyph twice with duplicate React keys.
    const sharedIdxGroup: TurnGroup = {
      parentTurnIndex: 5,
      blocks: [
        { role: 'assistant', content: '', block_kind: 'tool_use', turn_index: 5, parent_turn_index: 5, tool_name: 'Edit' },
        { role: 'tool', content: 'ok', block_kind: 'tool_result', turn_index: 5, parent_turn_index: 5 },
      ],
      representativeKind: 'tool_use',
      toolName: 'Edit',
    };
    const evidenceByTurn = new Map<number, EvidenceForTurn>([
      [
        5,
        {
          spans: [{ code: 'HAL_API', turn_index: 5, quote: '' }],
          primary: 'HAL_API',
          confidence: 0.9,
        },
      ],
    ]);
    render(<TurnGroupCard group={sharedIdxGroup} evidenceByTurn={evidenceByTurn} />);
    expect(screen.getAllByRole('img', { name: /HAL_API/i })).toHaveLength(1);
  });

  it('groups the glyphs with an aria-label so AT can hear the finding count', () => {
    const evidenceByTurn = new Map<number, EvidenceForTurn>([
      [
        5,
        {
          spans: [
            { code: 'HAL_API', turn_index: 5, quote: '' },
            { code: 'SCOPE_DRIFT', turn_index: 5, quote: '' },
          ],
          primary: 'HAL_API',
          confidence: 0.8,
        },
      ],
    ]);
    render(<TurnGroupCard group={group} evidenceByTurn={evidenceByTurn} />);
    expect(
      screen.getByRole('group', { name: /2 symptom findings/i }),
    ).toBeInTheDocument();
  });

  it('silently ignores evidence pointing at turn indices outside the group', () => {
    const evidenceByTurn = new Map<number, EvidenceForTurn>([
      [
        99, // not present in `group`
        {
          spans: [{ code: 'HAL_API', turn_index: 99, quote: '' }],
          primary: 'HAL_API',
          confidence: 0.9,
        },
      ],
    ]);
    render(<TurnGroupCard group={group} evidenceByTurn={evidenceByTurn} />);
    expect(screen.queryByRole('img')).not.toBeInTheDocument();
  });
});
