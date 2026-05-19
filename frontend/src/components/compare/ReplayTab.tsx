import { useEffect, useMemo, useState } from 'react';

import { useAnchorAlignment } from '../../lib/use-anchor-alignment';
import { useReplayClock } from '../../lib/use-replay-clock';
import type { RunAnchors, AlignmentRow } from '../../lib/anchor-alignment';
import { TapeRow } from './TapeRow';
import { TransportBar } from './TransportBar';

/**
 * ReplayTab — synced playback of the Tape rows.
 *
 * The clock advances over the same row list TapeTab renders. Each
 * step "reveals" one more row; unrevealed rows are masked with a
 * dimmed placeholder so the user sees the agent's decisions
 * accumulate, like watching the run play out frame-by-frame.
 *
 * Anchor-step mode is the only mode shipped here; wall-clock mode
 * (which would compute per-step intervalMs from ParsedTurn
 * timestamps) is a follow-up alongside the Inspector live-streaming
 * work in #128.
 *
 * Keyboard:
 *   - space: play/pause
 *   - ←/→: step back / step forward
 * J/K (next/prev fork) is wired alongside the ForkDrawer in a later
 * story.
 */

interface ReplayTabProps {
  runs: RunAnchors[];
}

export function ReplayTab({ runs }: ReplayTabProps) {
  const rows = useAnchorAlignment(runs);
  const runIds = useMemo(() => runs.map((r) => r.run_id), [runs]);
  const [speed, setSpeed] = useState(1);
  const clock = useReplayClock({ totalSteps: Math.max(1, rows.length), speed });

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      // Don't hijack keys while the user is typing in a control.
      const active = document.activeElement as HTMLElement | null;
      const tag = (active?.tagName ?? '').toLowerCase();
      if (tag === 'input' || tag === 'textarea' || active?.getAttribute('contenteditable') === 'true') return;
      if (e.key === ' ') {
        e.preventDefault();
        clock.playing ? clock.pause() : clock.play();
      } else if (e.key === 'ArrowRight') {
        e.preventDefault();
        clock.advance();
      } else if (e.key === 'ArrowLeft') {
        e.preventDefault();
        clock.stepBack();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [clock]);

  if (rows.length === 0) {
    return (
      <div className="flex h-40 items-center justify-center rounded-md border border-dashed border-border bg-bg-elev-1 text-sm text-fg-muted">
        Select runs with completed transcripts to replay their anchored timeline.
      </div>
    );
  }

  const revealed = clock.step + 1;

  return (
    <div className="flex flex-col gap-3">
      <div
        role="grid"
        aria-label="Replay tape"
        className="overflow-x-auto rounded-md border border-border bg-bg-elev-1"
      >
        <div
          role="row"
          className="grid items-stretch border-b border-border-strong bg-bg-elev-2 font-mono text-xs uppercase tracking-wider text-fg-muted"
          style={{ gridTemplateColumns: `220px repeat(${runIds.length}, minmax(140px, 1fr))` }}
        >
          <div role="columnheader" className="border-r border-border px-3 py-2">
            Anchor
          </div>
          {runIds.map((id) => (
            <div role="columnheader" key={id} className="border-r border-border px-3 py-2 last:border-r-0">
              {id}
            </div>
          ))}
        </div>
        {rows.slice(0, revealed).map((row, i) => (
          <TapeRow key={i} row={row} runIds={runIds} />
        ))}
        {rows.slice(revealed).map((_row, i) => (
          <MaskedRow key={`mask-${i}`} runIds={runIds} />
        ))}
      </div>
      <TransportBar
        playing={clock.playing}
        step={clock.step}
        totalSteps={rows.length}
        speed={speed}
        onPlayPause={() => (clock.playing ? clock.pause() : clock.play())}
        onStepBack={clock.stepBack}
        onStepForward={clock.advance}
        onScrub={clock.jumpTo}
        onSpeedChange={setSpeed}
      />
    </div>
  );
}

function MaskedRow({ runIds }: { runIds: string[] }) {
  return (
    <div
      role="row"
      aria-hidden="true"
      className="grid items-stretch border-b border-border bg-bg-elev-2/30"
      style={{ gridTemplateColumns: `220px repeat(${runIds.length}, minmax(140px, 1fr))` }}
    >
      <div className="border-r border-border px-3 py-2 font-mono text-xs text-fg-subtle">·····</div>
      {runIds.map((id) => (
        <div key={id} className="border-r border-border px-3 py-2 text-xs text-fg-subtle last:border-r-0">
          ·
        </div>
      ))}
    </div>
  );
}
