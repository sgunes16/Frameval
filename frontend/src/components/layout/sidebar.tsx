import type { ReactElement } from 'react';
import { NavLink } from 'react-router-dom';
import { cn } from '../../lib/utils';

type NavItem = {
  to: string;
  label: string;
  icon: ReactElement;
  hint?: string;
  disabled?: boolean;
};

const navItems: NavItem[] = [
  {
    to: '/',
    label: 'Dashboard',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" className="h-4 w-4">
        <path strokeLinecap="round" strokeLinejoin="round" d="M3 12l9-9 9 9" />
        <path strokeLinecap="round" strokeLinejoin="round" d="M5 10v10h14V10" />
      </svg>
    ),
  },
  {
    to: '/diagnostic/launch',
    label: 'Run diagnostic',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" className="h-4 w-4">
        <path strokeLinecap="round" strokeLinejoin="round" d="M5 3l3 9-3 9 16-9z" />
      </svg>
    ),
  },
  {
    to: '/diagnostic/compare',
    label: 'Compare runs',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" className="h-4 w-4">
        <path strokeLinecap="round" strokeLinejoin="round" d="M3 6h7m-7 6h7m-7 6h7M14 6h7m-7 6h7m-7 6h7" />
      </svg>
    ),
  },
  {
    to: '/experiments',
    label: 'Experiments',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" className="h-4 w-4">
        <path strokeLinecap="round" strokeLinejoin="round" d="M9 3v6l-5 9a2 2 0 001.7 3h12.6a2 2 0 001.7-3l-5-9V3" />
        <path strokeLinecap="round" strokeLinejoin="round" d="M7 3h10" />
      </svg>
    ),
  },
  {
    to: '/tasks',
    label: 'Task library',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" className="h-4 w-4">
        <rect x="4" y="4" width="16" height="16" rx="3" />
        <path strokeLinecap="round" strokeLinejoin="round" d="M8 10h8M8 14h5" />
      </svg>
    ),
  },
  {
    to: '/artifacts',
    label: 'Artifacts',
    hint: 'Preview',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" className="h-4 w-4">
        <path strokeLinecap="round" strokeLinejoin="round" d="M4 7l8-4 8 4M4 7v10l8 4 8-4V7M4 7l8 4 8-4M12 11v10" />
      </svg>
    ),
  },
  {
    to: '/settings',
    label: 'Settings',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" className="h-4 w-4">
        <circle cx="12" cy="12" r="3" />
        <path strokeLinecap="round" strokeLinejoin="round" d="M19.4 15a1.7 1.7 0 00.3 1.8l.1.1a2 2 0 11-2.8 2.8l-.1-.1a1.7 1.7 0 00-1.8-.3 1.7 1.7 0 00-1 1.5V21a2 2 0 11-4 0v-.1a1.7 1.7 0 00-1.1-1.5 1.7 1.7 0 00-1.8.3l-.1.1a2 2 0 11-2.8-2.8l.1-.1a1.7 1.7 0 00.3-1.8 1.7 1.7 0 00-1.5-1H3a2 2 0 110-4h.1A1.7 1.7 0 004.6 9a1.7 1.7 0 00-.3-1.8l-.1-.1a2 2 0 112.8-2.8l.1.1a1.7 1.7 0 001.8.3H9a1.7 1.7 0 001-1.5V3a2 2 0 114 0v.1a1.7 1.7 0 001 1.5 1.7 1.7 0 001.8-.3l.1-.1a2 2 0 112.8 2.8l-.1.1a1.7 1.7 0 00-.3 1.8V9a1.7 1.7 0 001.5 1H21a2 2 0 110 4h-.1a1.7 1.7 0 00-1.5 1z" />
      </svg>
    ),
  },
];

export function Sidebar() {
  return (
    <aside className="flex w-56 flex-col gap-1 border-r border-border bg-bg-elev-1/80 px-3 py-5 backdrop-blur">
      <div className="mb-5 flex items-center gap-2 px-2">
        <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-fg text-sm font-bold text-bg">F</div>
        <div>
          <div className="text-sm font-semibold leading-4 text-fg">Frameval</div>
          <div className="text-xs uppercase tracking-wider text-fg-muted">Context eval</div>
        </div>
      </div>
      <nav className="flex flex-col gap-0.5">
        {navItems.map((item) =>
          item.disabled ? (
            <div
              key={item.to}
              className="flex cursor-not-allowed items-center justify-between rounded-lg px-3 py-2 text-sm text-fg-subtle"
            >
              <div className="flex items-center gap-2.5">
                <span className="text-fg-subtle">{item.icon}</span>
                <span>{item.label}</span>
              </div>
              {item.hint && (
                <span className="rounded-full border border-border bg-bg-elev-2 px-1.5 py-0.5 text-xs font-medium uppercase tracking-wider text-fg-subtle">
                  {item.hint}
                </span>
              )}
            </div>
          ) : (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === '/'}
              className={({ isActive }) =>
                cn(
                  'flex items-center justify-between rounded-lg px-3 py-2 text-sm transition',
                  isActive
                    ? 'bg-fg text-bg shadow-sm'
                    : 'text-fg-muted hover:bg-bg-elev-2 hover:text-fg',
                )
              }
            >
              {({ isActive }) => (
                <>
                  <div className="flex items-center gap-2.5">
                    <span className={isActive ? 'text-bg' : 'text-fg-subtle'}>{item.icon}</span>
                    <span>{item.label}</span>
                  </div>
                  {item.hint && (
                    <span
                      className={cn(
                        'rounded-full border px-1.5 py-0.5 text-xs font-medium uppercase tracking-wider',
                        isActive
                          ? 'border-bg/20 bg-bg/10 text-bg/80'
                          : 'border-border bg-bg-elev-2 text-fg-muted',
                      )}
                    >
                      {item.hint}
                    </span>
                  )}
                </>
              )}
            </NavLink>
          ),
        )}
      </nav>
      <div className="mt-auto rounded-lg border border-border bg-bg-elev-2/80 p-3 text-xs leading-4 text-fg-muted">
        <div className="mb-1 font-semibold text-fg">Local-first · v0.1</div>
        SQLite, Docker sandboxes, deterministic grading by default.
      </div>
    </aside>
  );
}
