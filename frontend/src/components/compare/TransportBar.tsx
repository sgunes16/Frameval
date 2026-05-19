/**
 * TransportBar — playback controls for the Compare V2 Replay tab.
 *
 * Layout: play/pause, step back/forward, scrub slider, speed select.
 * The speed select is constrained to the documented values (0.5×, 1×,
 * 2×, 5×, 25×) so the UI doesn't leak free-form numeric input.
 *
 * Keyboard shortcuts (space / ←/→) are bound at the ReplayTab level,
 * not here — this component is a presentation surface only.
 */

interface TransportBarProps {
  playing: boolean;
  step: number;
  totalSteps: number;
  speed: number;
  onPlayPause: () => void;
  onStepBack: () => void;
  onStepForward: () => void;
  onScrub: (step: number) => void;
  onSpeedChange: (speed: number) => void;
}

const SPEEDS = [0.5, 1, 2, 5, 25];

export function TransportBar({
  playing,
  step,
  totalSteps,
  speed,
  onPlayPause,
  onStepBack,
  onStepForward,
  onScrub,
  onSpeedChange,
}: TransportBarProps) {
  const max = Math.max(0, totalSteps - 1);
  return (
    <div
      role="toolbar"
      aria-label="Replay transport"
      className="flex items-center gap-3 rounded-md border border-border bg-bg-elev-2 px-3 py-2"
    >
      <button
        type="button"
        aria-label={playing ? 'Pause replay' : 'Play replay'}
        onClick={onPlayPause}
        className="inline-flex h-7 w-7 items-center justify-center rounded-sm border border-border bg-bg-elev-1 text-fg hover:bg-bg-elev-2"
      >
        {playing ? '⏸' : '▶'}
      </button>
      <button
        type="button"
        aria-label="Step backward"
        onClick={onStepBack}
        className="inline-flex h-7 w-7 items-center justify-center rounded-sm border border-border bg-bg-elev-1 text-fg hover:bg-bg-elev-2"
      >
        ◀
      </button>
      <button
        type="button"
        aria-label="Step forward"
        onClick={onStepForward}
        className="inline-flex h-7 w-7 items-center justify-center rounded-sm border border-border bg-bg-elev-1 text-fg hover:bg-bg-elev-2"
      >
        ▶
      </button>
      <input
        type="range"
        min={0}
        max={max}
        value={step}
        onChange={(e) => onScrub(Number(e.target.value))}
        aria-label="Replay position"
        className="flex-1 accent-accent"
      />
      <span className="font-mono text-xs text-fg-muted">
        {step + 1}/{totalSteps}
      </span>
      {/*
        The wrapping <label> alone provides the accessible name via
        its visible sr-only text — adding aria-label="Replay speed"
        would double-announce the name on NVDA+Chrome. Pick one.
      */}
      <label className="flex items-center gap-1 text-xs text-fg-muted">
        <span className="sr-only">Replay speed</span>
        <select
          value={speed}
          onChange={(e) => onSpeedChange(Number(e.target.value))}
          className="rounded-sm border border-border bg-bg-elev-1 px-2 py-0.5 text-xs text-fg"
        >
          {SPEEDS.map((s) => (
            <option key={s} value={s}>
              {s}×
            </option>
          ))}
        </select>
      </label>
    </div>
  );
}
