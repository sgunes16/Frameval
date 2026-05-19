import { useCallback, useEffect, useState } from 'react';

/**
 * useTheme — the three-state theme controller for Design System V2.
 *
 * `mode` is what the user explicitly chose (system / dark / light).
 * `resolved` is the concrete value that drives the .dark class on
 * <html>: when mode === 'system', resolved reflects the OS preference
 * via prefers-color-scheme; otherwise resolved === mode.
 *
 * State is persisted to localStorage under STORAGE_KEY so the choice
 * survives reloads. The system-mode listener attaches to matchMedia
 * and reacts live to OS theme changes (e.g. Night Shift kicking in)
 * without the user touching the toggle.
 */

export type ThemeMode = 'system' | 'dark' | 'light';
export type ResolvedTheme = 'dark' | 'light';

const STORAGE_KEY = 'frameval-theme';

function readStoredMode(): ThemeMode {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw === 'dark' || raw === 'light' || raw === 'system') return raw;
  } catch {
    // localStorage can be disabled (Safari private mode); fall back.
  }
  return 'system';
}

function prefersDark(): boolean {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches;
}

function applyResolved(resolved: ResolvedTheme) {
  if (typeof document === 'undefined') return;
  document.documentElement.classList.toggle('dark', resolved === 'dark');
}

export function useTheme() {
  const [mode, setModeState] = useState<ThemeMode>(() => readStoredMode());
  const [systemDark, setSystemDark] = useState<boolean>(() => prefersDark());

  // Listen to OS theme changes; cleanup on unmount.
  useEffect(() => {
    if (typeof window === 'undefined') return;
    const mq = window.matchMedia('(prefers-color-scheme: dark)');
    const onChange = (e: { matches: boolean }) => setSystemDark(e.matches);
    mq.addEventListener('change', onChange);
    return () => mq.removeEventListener('change', onChange);
  }, []);

  // Compute the resolved value from mode + system preference.
  const resolved: ResolvedTheme = mode === 'system' ? (systemDark ? 'dark' : 'light') : mode;

  // Whenever resolved changes, toggle the html class.
  useEffect(() => {
    applyResolved(resolved);
  }, [resolved]);

  const setMode = useCallback((next: ThemeMode) => {
    setModeState(next);
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      // localStorage can be disabled; choice still persists for this
      // session via React state.
    }
  }, []);

  return { mode, setMode, resolved };
}
