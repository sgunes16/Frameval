import { describe, expect, it } from 'vitest';
import { parseAgentEvents, type AgentLogEvent } from './log-viewer';
import type { Run } from '../../lib/types';

// Regression: Aider emits plain text, not JSON. Every line ran through the
// raw-fallback branch. appendItem only merged 'assistant' and 'thinking',
// so N raw lines from the same run produced N stub cards in the timeline.
// The fix groups consecutive raw lines from the same run into one card.
function rawEvent(id: number, line: string, runId = 'run-1', runNumber = 1): AgentLogEvent {
  return { id, line, runId, runNumber, stage: 'executor' };
}

function assistantEvent(id: number, text: string, runId = 'run-1', runNumber = 1): AgentLogEvent {
  return {
    id,
    line: JSON.stringify({ type: 'assistant', message: { content: [{ text }] } }),
    runId,
    runNumber,
    stage: 'executor',
  };
}

const runs: Run[] = [];

describe('parseAgentEvents', () => {
  it('merges consecutive raw lines from the same run into a single card', () => {
    const events = [
      rawEvent(1, 'Warning: Input is not a terminal.'),
      rawEvent(2, 'Update git name with: git config user.name "Your Name"'),
      rawEvent(3, 'Aider v0.86.2'),
      rawEvent(4, 'Model: openai/llama3.2:1b'),
    ];
    const { items } = parseAgentEvents(events, runs);
    expect(items).toHaveLength(1);
    expect(items[0].kind).toBe('raw');
    expect(items[0].body).toContain('Warning: Input');
    expect(items[0].body).toContain('Aider v0.86.2');
    // Lines should be separated by newlines so the <pre whitespace-pre-wrap>
    // renders them as a multi-line block, not a single soft-wrapped run-on.
    expect(items[0].body.split('\n')).toHaveLength(4);
  });

  it('does NOT merge raw lines from different runs', () => {
    const events = [
      rawEvent(1, 'aider output from run 1', 'run-1', 1),
      rawEvent(2, 'aider output from run 2', 'run-2', 2),
    ];
    const { items } = parseAgentEvents(events, runs);
    expect(items).toHaveLength(2);
  });

  it('still merges streaming assistant events (no regression)', () => {
    const events = [assistantEvent(1, 'Hello '), assistantEvent(2, 'world')];
    const { items } = parseAgentEvents(events, runs);
    expect(items).toHaveLength(1);
    expect(items[0].kind).toBe('assistant');
  });

  it('breaks raw run when a structured event lands between two raw lines', () => {
    // Confirms the merge is "consecutive" — a non-raw event in the middle
    // splits the raw stream into two cards.
    const events = [
      rawEvent(1, 'first raw line'),
      assistantEvent(2, 'hello from assistant'),
      rawEvent(3, 'second raw line'),
    ];
    const { items } = parseAgentEvents(events, runs);
    expect(items).toHaveLength(3);
    expect(items[0].kind).toBe('raw');
    expect(items[1].kind).toBe('assistant');
    expect(items[2].kind).toBe('raw');
  });
});
