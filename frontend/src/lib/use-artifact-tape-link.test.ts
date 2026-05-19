import { renderHook } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { useArtifactTapeLink } from './use-artifact-tape-link';
import type { ParsedTurn } from './types';

const thinking = (id: number, content: string): ParsedTurn => ({
  role: 'assistant',
  content,
  block_kind: 'thinking',
  turn_index: id,
  parent_turn_index: id,
});

describe('useArtifactTapeLink', () => {
  it('returns no highlights when paragraph is null', () => {
    const { result } = renderHook(() =>
      useArtifactTapeLink({ paragraph: null, turns: [thinking(0, 'anything')] }),
    );
    expect(result.current.highlights).toEqual(new Set());
  });

  it('returns no highlights when there are no thinking turns', () => {
    const { result } = renderHook(() =>
      useArtifactTapeLink({
        paragraph: 'rate limit policy',
        turns: [
          { role: 'assistant', content: '', block_kind: 'tool_use', tool_name: 'Edit', turn_index: 0, parent_turn_index: 0 },
        ],
      }),
    );
    expect(result.current.highlights).toEqual(new Set());
  });

  it('returns parent_turn_index of every thinking block whose trigrams jaccard the paragraph ≥ threshold', () => {
    const turns = [
      thinking(0, 'agent considered the rate limit policy at length'),
      thinking(1, 'unrelated chatter about user accounts and sessions'),
      thinking(2, 'rate limit policy needs revision'),
    ];
    const { result } = renderHook(() =>
      useArtifactTapeLink({ paragraph: 'rate limit policy', turns, threshold: 0.3 }),
    );
    expect(result.current.highlights.has(0)).toBe(true);
    expect(result.current.highlights.has(1)).toBe(false);
    expect(result.current.highlights.has(2)).toBe(true);
  });

  it('threshold defaults to 0.3 per the spec', () => {
    const turns = [thinking(0, 'rate limit policy is now active')];
    const { result } = renderHook(() =>
      useArtifactTapeLink({ paragraph: 'rate limit policy', turns }),
    );
    expect(result.current.highlights.has(0)).toBe(true);
  });
});
