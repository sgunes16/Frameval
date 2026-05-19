import { useEffect, useState } from 'react';

/**
 * LiveCursor — small live-indicator badge in the Inspector header
 * while a run is actively streaming turns over WS.
 *
 * Three states:
 *   - 'connected' + recent event: green dot + "Live" + relative "Xs ago"
 *   - 'connected' + no recent event: muted dot + "Connected"
 *   - 'disconnected': amber pulsing dot + "Reconnecting…"
 *
 * Rendered as a small inline pill, no animation beyond the dot's
 * pulse — the goal is "I see this is moving" not "look here".
 *
 * A 1-second `now` tick keeps the "Xs ago" label fresh between
 * events; without it the label freezes at the value computed on the
 * last event, which would imply the stream is more recent than it
 * actually is.
 */

interface LiveCursorProps {
  isConnected: boolean;
  lastEventAt: number | null;
  /** Total turns last reported by the WS payload. Surfaces "12 turns" in the badge. */
  turnCount: number | null;
}

/** Forces a re-render every `tickMs` so relative-time labels stay accurate. */
function useTick(tickMs: number): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), tickMs);
    return () => clearInterval(id);
  }, [tickMs]);
  return now;
}

function relativeTimeFromNow(ts: number, now: number): string {
  const deltaSec = Math.max(0, Math.floor((now - ts) / 1000));
  if (deltaSec < 1) return 'just now';
  if (deltaSec < 60) return `${deltaSec}s ago`;
  const min = Math.floor(deltaSec / 60);
  return `${min}m ago`;
}

export function LiveCursor({ isConnected, lastEventAt, turnCount }: LiveCursorProps) {
  // Tick once per second so the "Xs ago" label advances between
  // WS events. Cheap (setInterval, no DOM mutation beyond the
  // small badge re-render) and only active while the component is
  // mounted on the Inspector route.
  const now = useTick(1000);
  if (!isConnected) {
    return (
      <span
        role="status"
        aria-live="polite"
        className="inline-flex items-center gap-1.5 rounded-full border border-warning/30 bg-warning/10 px-2 py-0.5 text-xs text-warning-fg"
      >
        <span className="inline-block h-1.5 w-1.5 animate-pulse rounded-full bg-warning" aria-hidden="true" />
        Reconnecting…
      </span>
    );
  }
  if (lastEventAt === null) {
    return (
      <span
        role="status"
        className="inline-flex items-center gap-1.5 rounded-full border border-border bg-bg-elev-2 px-2 py-0.5 text-xs text-fg-muted"
      >
        <span className="inline-block h-1.5 w-1.5 rounded-full bg-fg-subtle" aria-hidden="true" />
        Connected
      </span>
    );
  }
  const turnsLabel = turnCount !== null ? ` · ${turnCount} turns` : '';
  return (
    <span
      role="status"
      aria-live="polite"
      className="inline-flex items-center gap-1.5 rounded-full border border-success/30 bg-success/10 px-2 py-0.5 text-xs text-success-fg"
    >
      <span className="inline-block h-1.5 w-1.5 animate-pulse rounded-full bg-success" aria-hidden="true" />
      Live · {relativeTimeFromNow(lastEventAt, now)}
      {turnsLabel}
    </span>
  );
}
