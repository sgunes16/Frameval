import { cn } from '../../lib/utils';
import { FilePath } from '../system';
import type { ParsedTurn } from '../../lib/types';

/**
 * Block renderers — one component per BlockKind, each rendering a
 * single ParsedTurn payload in the right visual style.
 *
 * Why separate per-kind components instead of a switch inside one
 * monolith: the renderers diverge structurally over time (thinking
 * may collapse, tool_use wants a syntax-highlighted code block,
 * tool_result wants a copy button, etc.). Splitting them up keeps
 * each file focused on one concern.
 */

/** ThinkingBlock — agent's internal monologue. Slightly muted so it
 * reads as "context" rather than action. */
export function ThinkingBlock({ block }: { block: ParsedTurn }) {
  return (
    <div className="space-y-1 text-sm text-fg-muted italic leading-relaxed">
      <pre className="whitespace-pre-wrap font-sans text-sm">{block.content}</pre>
    </div>
  );
}

/** ToolUseBlock — the agent invoked a tool. Renders the tool name +
 * its input as a monospace block; FilePath chips for any files this
 * turn touched so the user sees the scope at a glance. */
export function ToolUseBlock({ block }: { block: ParsedTurn }) {
  return (
    <div className="space-y-2">
      {block.tool_name && (
        <div className="flex items-center gap-2 text-xs text-fg-muted">
          <span className="font-mono font-medium text-fg">{block.tool_name}</span>
          {block.files_touched && block.files_touched.length > 0 && (
            <span className="flex flex-wrap gap-1">
              {block.files_touched.map((path) => (
                <FilePath key={path} path={path} />
              ))}
            </span>
          )}
        </div>
      )}
      <pre className="overflow-auto whitespace-pre-wrap rounded-sm bg-code-bg p-2 font-mono text-xs text-fg">
        {block.content}
      </pre>
    </div>
  );
}

/** ToolResultBlock — tool's response. Same layout as ToolUseBlock but
 * with a different left accent so users can scan input vs output. */
export function ToolResultBlock({ block }: { block: ParsedTurn }) {
  return (
    <pre
      className={cn(
        'overflow-auto whitespace-pre-wrap rounded-sm border-l-2 border-chart-2 bg-code-bg p-2 font-mono text-xs text-fg',
      )}
    >
      {block.content}
    </pre>
  );
}

/** TextBlock — assistant prose. Standard paragraph styling. */
export function TextBlock({ block }: { block: ParsedTurn }) {
  return (
    <pre className="whitespace-pre-wrap font-sans text-sm leading-relaxed text-fg">
      {block.content}
    </pre>
  );
}

/** SystemBlock — meta-events (harness setup, sandbox stderr, etc.).
 * Rendered in a quiet hairline-bordered frame so it doesn't steal
 * attention from the agent's own output. */
export function SystemBlock({ block }: { block: ParsedTurn }) {
  return (
    <pre className="whitespace-pre-wrap rounded-sm border border-border bg-bg-elev-2 p-2 font-mono text-xs text-fg-subtle">
      {block.content}
    </pre>
  );
}

/** renderBlock dispatches to the right block component by kind. The
 * default (empty or unknown kind) falls back to TextBlock — legacy
 * transcripts without grouping stamps render as plain prose. */
export function renderBlock(block: ParsedTurn) {
  switch (block.block_kind) {
    case 'thinking':
      return <ThinkingBlock block={block} />;
    case 'tool_use':
      return <ToolUseBlock block={block} />;
    case 'tool_result':
      return <ToolResultBlock block={block} />;
    case 'system':
      return <SystemBlock block={block} />;
    case 'text':
    default:
      return <TextBlock block={block} />;
  }
}
