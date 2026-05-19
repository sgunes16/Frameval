import { describe, expect, it } from 'vitest';

import { matchesRunTurnEvent, parseTurnEventPayload } from './use-turn-stream';

describe('matchesRunTurnEvent', () => {
  it('returns true for run.turn events whose payload run_id matches', () => {
    const ev = { type: 'run.turn', payload: { run_id: 'r1', experiment_id: 'e1', turn_count: 3 } };
    expect(matchesRunTurnEvent(ev, 'r1')).toBe(true);
  });

  it('returns false for run.turn events with a different run_id', () => {
    const ev = { type: 'run.turn', payload: { run_id: 'r2', experiment_id: 'e1', turn_count: 5 } };
    expect(matchesRunTurnEvent(ev, 'r1')).toBe(false);
  });

  it('returns false for unrelated event types even when run_id matches', () => {
    const ev = { type: 'run.log', payload: { run_id: 'r1' } };
    expect(matchesRunTurnEvent(ev, 'r1')).toBe(false);
  });

  it('returns false when payload is malformed (missing run_id)', () => {
    expect(matchesRunTurnEvent({ type: 'run.turn', payload: {} }, 'r1')).toBe(false);
    expect(matchesRunTurnEvent({ type: 'run.turn', payload: null }, 'r1')).toBe(false);
  });
});

describe('parseTurnEventPayload', () => {
  it('extracts experiment_id, run_id, and turn_count when present', () => {
    const out = parseTurnEventPayload({ experiment_id: 'e1', run_id: 'r1', turn_count: 7 });
    expect(out).toEqual({ experimentId: 'e1', runId: 'r1', turnCount: 7 });
  });

  it('returns null for malformed payloads', () => {
    expect(parseTurnEventPayload(null)).toBeNull();
    expect(parseTurnEventPayload('string')).toBeNull();
    expect(parseTurnEventPayload({ run_id: 'r1' })).toBeNull(); // missing turn_count
  });

  it('returns null when turn_count is not a finite number', () => {
    expect(parseTurnEventPayload({ run_id: 'r1', experiment_id: 'e1', turn_count: 'oops' })).toBeNull();
    expect(parseTurnEventPayload({ run_id: 'r1', experiment_id: 'e1', turn_count: NaN })).toBeNull();
  });
});
