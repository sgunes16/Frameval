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
});
