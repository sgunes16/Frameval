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
    to: '/baselines',
    label: 'Baselines',
    hint: 'Soon',
    disabled: true,
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" className="h-4 w-4">
        <path strokeLinecap="round" strokeLinejoin="round" d="M4 19V5m0 14h16m-8-4V9m4 6V11M8 15v-2" />
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
    <aside className="flex w-56 flex-col gap-1 border-r border-slate-200/70 bg-white/80 px-3 py-5 backdrop-blur">
      <div className="mb-5 flex items-center gap-2 px-2">
        <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-slate-900 text-sm font-bold text-white">F</div>
        <div>
          <div className="text-sm font-semibold leading-4 text-slate-900">Frameval</div>
          <div className="text-[10px] uppercase tracking-wider text-slate-500">Context eval</div>
        </div>
      </div>
      <nav className="flex flex-col gap-0.5">
        {navItems.map((item) =>
          item.disabled ? (
            <div
              key={item.to}
              className="flex cursor-not-allowed items-center justify-between rounded-lg px-3 py-2 text-sm text-slate-400"
            >
              <div className="flex items-center gap-2.5">
                <span className="text-slate-300">{item.icon}</span>
                <span>{item.label}</span>
              </div>
              {item.hint && (
                <span className="rounded-full border border-slate-200 bg-slate-50 px-1.5 py-0.5 text-[9px] font-medium uppercase tracking-wider text-slate-400">
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
                    ? 'bg-slate-900 text-white shadow-sm'
                    : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900',
                )
              }
            >
              {({ isActive }) => (
                <>
                  <div className="flex items-center gap-2.5">
                    <span className={isActive ? 'text-white' : 'text-slate-400'}>{item.icon}</span>
                    <span>{item.label}</span>
                  </div>
                  {item.hint && (
                    <span
                      className={cn(
                        'rounded-full border px-1.5 py-0.5 text-[9px] font-medium uppercase tracking-wider',
                        isActive
                          ? 'border-white/20 bg-white/10 text-white/80'
                          : 'border-slate-200 bg-slate-50 text-slate-500',
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
      <div className="mt-auto rounded-lg border border-slate-200/70 bg-slate-50/80 p-3 text-[11px] leading-4 text-slate-500">
        <div className="mb-1 font-semibold text-slate-700">Local-first · v0.1</div>
        SQLite, Docker sandboxes, deterministic grading by default.
      </div>
    </aside>
  );
}
