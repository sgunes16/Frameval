import type { Config } from 'tailwindcss';

/**
 * Tailwind config exposes the design tokens (see src/styles/tokens.css)
 * as utility classes. Every color reads through `hsl(var(--*) / <alpha-value>)`
 * so the same class adapts to whichever theme is active.
 *
 * Legacy shadcn-style colors (border / input / ring / background /
 * foreground / primary / secondary / muted / accent / destructive)
 * are kept as aliases over the new tokens so existing components keep
 * working during the migration story (#74). New code should consume the
 * token names directly (bg-elev-1, text-fg-muted, border-strong, etc.).
 */
export default {
  darkMode: ['class'],
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        // Design System V2 token surface — canonical.
        bg: 'hsl(var(--bg) / <alpha-value>)',
        'bg-elev-1': 'hsl(var(--bg-elev-1) / <alpha-value>)',
        'bg-elev-2': 'hsl(var(--bg-elev-2) / <alpha-value>)',
        fg: 'hsl(var(--fg) / <alpha-value>)',
        'fg-muted': 'hsl(var(--fg-muted) / <alpha-value>)',
        'fg-subtle': 'hsl(var(--fg-subtle) / <alpha-value>)',
        'border-strong': 'hsl(var(--border-strong) / <alpha-value>)',
        'code-bg': 'hsl(var(--code-bg) / <alpha-value>)',
        'diff-add': 'hsl(var(--diff-add) / <alpha-value>)',
        'diff-del': 'hsl(var(--diff-del) / <alpha-value>)',
        'diff-add-text': 'hsl(var(--diff-add-text) / <alpha-value>)',
        'diff-del-text': 'hsl(var(--diff-del-text) / <alpha-value>)',
        success: 'hsl(var(--success) / <alpha-value>)',
        warning: 'hsl(var(--warning) / <alpha-value>)',
        danger: 'hsl(var(--danger) / <alpha-value>)',
        info: 'hsl(var(--info) / <alpha-value>)',
        'chart-1': 'hsl(var(--chart-1) / <alpha-value>)',
        'chart-2': 'hsl(var(--chart-2) / <alpha-value>)',
        'chart-3': 'hsl(var(--chart-3) / <alpha-value>)',
        'chart-4': 'hsl(var(--chart-4) / <alpha-value>)',
        'chart-5': 'hsl(var(--chart-5) / <alpha-value>)',
        'chart-6': 'hsl(var(--chart-6) / <alpha-value>)',
        'chart-7': 'hsl(var(--chart-7) / <alpha-value>)',
        'chart-8': 'hsl(var(--chart-8) / <alpha-value>)',

        // Legacy shadcn aliases — kept so existing components keep working
        // through the migration in #74. Each delegates to a new token so
        // theme switching still applies.
        border: 'hsl(var(--border) / <alpha-value>)',
        input: 'hsl(var(--border) / <alpha-value>)',
        ring: 'hsl(var(--border-strong) / <alpha-value>)',
        background: 'hsl(var(--bg) / <alpha-value>)',
        foreground: 'hsl(var(--fg) / <alpha-value>)',
        primary: {
          DEFAULT: 'hsl(var(--accent) / <alpha-value>)',
          foreground: 'hsl(var(--bg) / <alpha-value>)',
        },
        secondary: {
          DEFAULT: 'hsl(var(--bg-elev-2) / <alpha-value>)',
          foreground: 'hsl(var(--fg) / <alpha-value>)',
        },
        muted: {
          DEFAULT: 'hsl(var(--bg-elev-2) / <alpha-value>)',
          foreground: 'hsl(var(--fg-muted) / <alpha-value>)',
        },
        accent: {
          DEFAULT: 'hsl(var(--accent) / <alpha-value>)',
          foreground: 'hsl(var(--bg) / <alpha-value>)',
        },
        destructive: {
          DEFAULT: 'hsl(var(--danger) / <alpha-value>)',
          foreground: 'hsl(var(--bg) / <alpha-value>)',
        },
      },
    },
  },
  plugins: [],
} satisfies Config;
