import { useCallback, useEffect, useRef, useState } from 'react';

/**
 * useReplayClock — drives the Replay tab transport in Compare V2
 * (story #68). Counts a discrete `step` from 0 to `totalSteps - 1`,
 * advancing automatically on a setInterval when `playing` is true.
 *
 * Steps are deliberately units-agnostic. The Replay tab uses them
 * as "rows of the Tape table" in anchor-step mode; a future wall-
 * clock mode can compute a custom intervalMs per step from the
 * ParsedTurn timestamps without changing this hook's shape.
 *
 * speed:
 *   - 1 = baseline; one step per intervalMs.
 *   - 2 = effective intervalMs halved.
 *   - 0.5 = effective intervalMs doubled.
 *
 * The clock pauses itself on reaching the final step so the
 * transport bar's play button can re-arm with rewind + play.
 */

interface UseReplayClockArgs {
  totalSteps: number;
  intervalMs?: number;
  speed?: number;
}

export interface ReplayClockState {
  step: number;
  playing: boolean;
  play: () => void;
  pause: () => void;
  advance: () => void;
  stepBack: () => void;
  jumpTo: (target: number) => void;
}

export function useReplayClock({
  totalSteps,
  intervalMs = 1000,
  speed = 1,
}: UseReplayClockArgs): ReplayClockState {
  const [step, setStep] = useState(0);
  const [playing, setPlaying] = useState(false);
  // Latest step in a ref so the interval callback (created once)
  // reads the current value instead of capturing 0 forever.
  const stepRef = useRef(step);
  stepRef.current = step;

  const clamp = useCallback(
    (v: number) => Math.max(0, Math.min(totalSteps - 1, v)),
    [totalSteps],
  );

  const advance = useCallback(() => {
    setStep((prev) => {
      const next = prev + 1;
      if (next >= totalSteps - 1) {
        setPlaying(false);
        return totalSteps - 1;
      }
      return next;
    });
  }, [totalSteps]);

  useEffect(() => {
    if (!playing) return;
    const effective = Math.max(16, intervalMs / Math.max(0.01, speed));
    const id = setInterval(advance, effective);
    return () => clearInterval(id);
  }, [playing, intervalMs, speed, advance]);

  const advanceManual = useCallback(() => setStep((prev) => clamp(prev + 1)), [clamp]);
  const stepBack = useCallback(() => setStep((prev) => clamp(prev - 1)), [clamp]);
  const jumpTo = useCallback((target: number) => setStep(clamp(target)), [clamp]);
  const play = useCallback(() => setPlaying(true), []);
  const pause = useCallback(() => setPlaying(false), []);

  return {
    step,
    playing,
    play,
    pause,
    advance: advanceManual,
    stepBack,
    jumpTo,
  };
}
