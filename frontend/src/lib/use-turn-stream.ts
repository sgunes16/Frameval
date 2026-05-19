import { useEffect, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';

/**
 * Inspector V2 live-streaming hook (story #128, closes #64).
 *
 * Opens a WebSocket to the engine's broadcast endpoint, filters the
 * incoming `run.turn` events for the focused run, and invalidates
 * the TanStack Query cache so the existing `useRunTurns` hook
 * refetches via REST. The orchestrator only emits a small payload
 * (`{experiment_id, run_id, turn_count}`) per turn, so the WS is the
 * notification channel and the REST endpoint stays the source of
 * truth — the design choice from PR #118.
 *
 * Reconcile-on-reconnect: when the socket closes and re-opens, the
 * hook invalidates the cache once on reopen so the REST refetch
 * reconciles any turns that closed during the outage. No persistent
 * in-memory turn list to merge.
 *
 * `lastEventAt` exposes the millis-timestamp of the last matching
 * event for the inspect page header's "Live · 2s ago" indicator.
 */

interface WebSocketEvent {
  type: string;
  payload: unknown;
}

export interface ParsedTurnEvent {
  experimentId: string;
  runId: string;
  turnCount: number;
}

/**
 * True iff the event is a run.turn payload for the given run.
 * Defensive parsing: malformed payloads (missing keys, null) do
 * NOT match. Exposed for unit testing.
 */
export function matchesRunTurnEvent(ev: WebSocketEvent, runId: string): boolean {
  if (ev.type !== 'run.turn') return false;
  if (typeof ev.payload !== 'object' || ev.payload === null) return false;
  return (ev.payload as { run_id?: unknown }).run_id === runId;
}

/**
 * Convert a raw run.turn payload to our canonical shape, or return
 * null if any required field is missing / invalid. Exposed for unit
 * testing.
 */
export function parseTurnEventPayload(payload: unknown): ParsedTurnEvent | null {
  if (typeof payload !== 'object' || payload === null) return null;
  const p = payload as { experiment_id?: unknown; run_id?: unknown; turn_count?: unknown };
  if (typeof p.experiment_id !== 'string') return null;
  if (typeof p.run_id !== 'string') return null;
  if (typeof p.turn_count !== 'number' || !Number.isFinite(p.turn_count)) return null;
  return { experimentId: p.experiment_id, runId: p.run_id, turnCount: p.turn_count };
}

interface UseTurnStreamResult {
  isConnected: boolean;
  lastEventAt: number | null;
  lastTurnCount: number | null;
}

/**
 * Subscribes to run.turn events for `runId` and invalidates the
 * `['run-turns', runId]` and `['transcript', runId]` queries on
 * each matching event so the existing query consumers refresh.
 *
 * Pass undefined `runId` to no-op (e.g. while the route param is
 * still loading).
 */
export function useTurnStream(runId: string | undefined): UseTurnStreamResult {
  const client = useQueryClient();
  const [state, setState] = useState<UseTurnStreamResult>({
    isConnected: false,
    lastEventAt: null,
    lastTurnCount: null,
  });
  const socketRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    if (!runId) return;
    // happy-dom v18 doesn't ship a WebSocket constructor; SSR /
    // older browsers may also lack it. Bail gracefully so the
    // Inspector route still renders (just without the live badge).
    if (typeof WebSocket === 'undefined') return;
    const wsBase =
      import.meta.env.VITE_WS_BASE_URL ||
      `${window.location.origin.replace(/^http/, 'ws')}/ws`;
    let cancelled = false;

    const connect = () => {
      const socket = new WebSocket(wsBase);
      socketRef.current = socket;

      socket.onopen = () => {
        if (cancelled) return;
        setState((prev) => ({ ...prev, isConnected: true }));
        // Reconcile any turns we missed while disconnected.
        client.invalidateQueries({ queryKey: ['run-turns', runId] });
        client.invalidateQueries({ queryKey: ['transcript', runId] });
      };

      socket.onmessage = (event) => {
        if (cancelled) return;
        let data: WebSocketEvent;
        try {
          data = JSON.parse(event.data) as WebSocketEvent;
        } catch {
          return;
        }
        if (!matchesRunTurnEvent(data, runId)) return;
        const parsed = parseTurnEventPayload(data.payload);
        if (!parsed) return;
        setState({
          isConnected: true,
          lastEventAt: Date.now(),
          lastTurnCount: parsed.turnCount,
        });
        client.invalidateQueries({ queryKey: ['run-turns', runId] });
        client.invalidateQueries({ queryKey: ['transcript', runId] });
      };

      socket.onclose = () => {
        if (cancelled) return;
        setState((prev) => ({ ...prev, isConnected: false }));
        // Backoff before reconnect. 2s is a deliberate compromise:
        // long enough that a brief blip doesn't thrash the engine,
        // short enough that the user doesn't perceive a gap.
        setTimeout(() => {
          if (!cancelled) connect();
        }, 2000);
      };

      socket.onerror = () => {
        // onclose fires next; the reconnect path lives there.
      };
    };

    connect();

    return () => {
      cancelled = true;
      socketRef.current?.close();
    };
  }, [runId, client]);

  return state;
}
