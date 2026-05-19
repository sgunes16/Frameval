import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, renderHook } from '@testing-library/react';

import { useTheme, type ThemeMode } from './use-theme';

const STORAGE_KEY = 'frameval-theme';

/**
 * matchMedia stub — happy-dom exposes the API but always returns false
 * unless we install a fake. The hook reads `prefers-color-scheme: dark`
 * to decide what 'system' mode resolves to; the helper below lets each
 * test pick what the OS reports.
 */
function installMatchMedia(prefersDark: boolean) {
  const listeners: Array<(e: { matches: boolean }) => void> = [];
  const stub = vi.fn().mockImplementation((query: string) => ({
    matches: query.includes('dark') && prefersDark,
    media: query,
    addEventListener: (_evt: string, cb: (e: { matches: boolean }) => void) => listeners.push(cb),
    removeEventListener: (_evt: string, cb: (e: { matches: boolean }) => void) => {
      const i = listeners.indexOf(cb);
      if (i >= 0) listeners.splice(i, 1);
    },
    dispatchEvent: () => true,
  }));
  Object.defineProperty(window, 'matchMedia', { value: stub, writable: true, configurable: true });
  return {
    emitChange(matches: boolean) {
      listeners.forEach((cb) => cb({ matches }));
    },
  };
}

beforeEach(() => {
  localStorage.clear();
  document.documentElement.classList.remove('dark');
});

afterEach(() => {
  localStorage.clear();
  document.documentElement.classList.remove('dark');
});

describe('useTheme', () => {
  it('defaults to system mode when localStorage is empty', () => {
    installMatchMedia(false);
    const { result } = renderHook(() => useTheme());
    expect(result.current.mode).toBe<ThemeMode>('system');
  });

  it('loads the persisted mode from localStorage on mount', () => {
    localStorage.setItem(STORAGE_KEY, 'dark');
    installMatchMedia(false);
    const { result } = renderHook(() => useTheme());
    expect(result.current.mode).toBe<ThemeMode>('dark');
  });

  it('persists the mode and applies the .dark class when set to dark', () => {
    installMatchMedia(false);
    const { result } = renderHook(() => useTheme());
    act(() => result.current.setMode('dark'));
    expect(localStorage.getItem(STORAGE_KEY)).toBe('dark');
    expect(document.documentElement.classList.contains('dark')).toBe(true);
  });

  it('removes the .dark class when set to light', () => {
    document.documentElement.classList.add('dark');
    installMatchMedia(true);
    const { result } = renderHook(() => useTheme());
    act(() => result.current.setMode('light'));
    expect(document.documentElement.classList.contains('dark')).toBe(false);
  });

  it('resolves system mode by reading prefers-color-scheme', () => {
    installMatchMedia(true);
    const { result } = renderHook(() => useTheme());
    act(() => result.current.setMode('system'));
    // OS says dark — html should carry the .dark class.
    expect(document.documentElement.classList.contains('dark')).toBe(true);
    expect(result.current.resolved).toBe('dark');
  });

  it('reacts to OS theme changes while in system mode', () => {
    const { emitChange } = installMatchMedia(false);
    const { result } = renderHook(() => useTheme());
    act(() => result.current.setMode('system'));

    expect(document.documentElement.classList.contains('dark')).toBe(false);

    act(() => emitChange(true));
    expect(document.documentElement.classList.contains('dark')).toBe(true);
  });
});
