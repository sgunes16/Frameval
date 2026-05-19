import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { useReplayClock } from './use-replay-clock';

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('useReplayClock', () => {
  it('starts at step 0 and paused by default', () => {
    const { result } = renderHook(() => useReplayClock({ totalSteps: 10 }));
    expect(result.current.step).toBe(0);
    expect(result.current.playing).toBe(false);
  });

  it('play() toggles playing on; pause() turns it off', () => {
    const { result } = renderHook(() => useReplayClock({ totalSteps: 10 }));
    act(() => result.current.play());
    expect(result.current.playing).toBe(true);
    act(() => result.current.pause());
    expect(result.current.playing).toBe(false);
  });

  it('advance() moves forward by one; stepBack() retreats but clamps at 0', () => {
    const { result } = renderHook(() => useReplayClock({ totalSteps: 10 }));
    act(() => result.current.advance());
    expect(result.current.step).toBe(1);
    act(() => result.current.stepBack());
    expect(result.current.step).toBe(0);
    act(() => result.current.stepBack());
    expect(result.current.step).toBe(0); // clamped
  });

  it('autoplay advances at intervalMs intervals when playing', () => {
    const { result } = renderHook(() => useReplayClock({ totalSteps: 5, intervalMs: 1000 }));
    act(() => result.current.play());
    act(() => vi.advanceTimersByTime(2500));
    expect(result.current.step).toBe(2);
  });

  it('reaching totalSteps - 1 pauses autoplay', () => {
    const { result } = renderHook(() => useReplayClock({ totalSteps: 3, intervalMs: 100 }));
    act(() => result.current.play());
    act(() => vi.advanceTimersByTime(500));
    expect(result.current.step).toBe(2);
    expect(result.current.playing).toBe(false);
  });

  it('speed multiplier scales the interval; 2× halves it', () => {
    const { result } = renderHook(() =>
      useReplayClock({ totalSteps: 10, intervalMs: 1000, speed: 2 }),
    );
    act(() => result.current.play());
    act(() => vi.advanceTimersByTime(1000));
    // 1000ms at 2× = effective 500ms tick → 2 advances.
    expect(result.current.step).toBe(2);
  });

  it('jumpTo() clamps to [0, totalSteps-1]', () => {
    const { result } = renderHook(() => useReplayClock({ totalSteps: 5 }));
    act(() => result.current.jumpTo(99));
    expect(result.current.step).toBe(4);
    act(() => result.current.jumpTo(-5));
    expect(result.current.step).toBe(0);
  });
});
