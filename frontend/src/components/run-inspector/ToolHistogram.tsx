import type { ToolHistogramRow } from '../../lib/tool-histogram';

/**
 * ToolHistogram — sidebar bar chart showing per-tool call frequency
 * for the whole run. Tells the user at a glance "this run was 70 %
 * Edit, 20 % Bash, 10 % Read" — a key diagnostic for AgentDx
 * harness comparisons.
 *
 * No Recharts dependency: a horizontal bar is just a token-coloured
 * div with a percent width. Adding Recharts here would balloon the
 * inspector route bundle for a 3-row chart.
 */
export function ToolHistogram({ rows }: { rows: ToolHistogramRow[] }) {
  if (rows.length === 0) {
    return (
      <div className="text-xs text-fg-muted">
        No tool calls in this run yet.
      </div>
    );
  }
  const max = Math.max(...rows.map((row) => row.count), 1);
  return (
    <ul className="space-y-1.5" aria-label="Tool usage histogram">
      {rows.map((row) => {
        const pct = Math.round((row.count / max) * 100);
        return (
          <li key={row.tool} className="space-y-0.5">
            <div className="flex items-baseline justify-between text-xs">
              <span className="font-mono text-fg">{row.tool}</span>
              <span className="font-mono text-fg-muted">×{row.count}</span>
            </div>
            <div
              className="h-1.5 overflow-hidden rounded-full bg-bg-elev-2"
              role="presentation"
            >
              <div
                className="h-full bg-accent"
                style={{ width: `${pct}%` }}
                aria-hidden="true"
              />
            </div>
          </li>
        );
      })}
    </ul>
  );
}
