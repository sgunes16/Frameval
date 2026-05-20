import { useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

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

/** ThinkingBlock — agent's internal monologue. Slightly muted +
 * italic so it reads as "context" rather than action. Markdown is
 * rendered so headers, lists, inline code in the model's reasoning
 * display the way they look in the agent's CLI. */
export function ThinkingBlock({ block }: { block: ParsedTurn }) {
  return (
    <div className="italic">
      <MarkdownProse body={block.content} muted />
    </div>
  );
}

/** ToolUseBlock — one tool invocation inside an agent decision.
 * Layout: a single tight line of `<tool-name> <inline summary>` where
 * tool-name is monospace accent and the summary is muted mono. The
 * group rail in TurnGroupCard supplies the visual anchor — this row
 * intentionally has no ● dot of its own. The body (diff, code,
 * terminal) only appears for tools whose payload doesn't fit inline. */
export function ToolUseBlock({ block }: { block: ParsedTurn }) {
  const input = parseToolInput(block.content);
  const tool = (block.tool_name ?? '').toLowerCase();
  const headerSummary = toolHeaderSummary(tool, input, block.files_touched);
  const body = renderToolBody(tool, input, block.content);
  const isError = block.stage === 'error';

  return (
    <div className="space-y-1.5">
      <div className="flex items-baseline gap-2 font-mono text-sm leading-5">
        <span
          className={cn(
            'font-medium',
            isError ? 'text-danger-fg' : 'text-accent',
          )}
        >
          {block.tool_name || 'tool'}
        </span>
        {headerSummary && (
          <span className={cn('text-xs', isError ? 'text-danger-fg/80' : 'text-fg-muted')}>
            {headerSummary}
          </span>
        )}
        {block.files_touched && block.files_touched.length > 0 && !headerSummary && (
          <span className="flex flex-wrap gap-1">
            {block.files_touched.map((path) => (
              <FilePath key={path} path={path} />
            ))}
          </span>
        )}
      </div>
      {body && <div className="ml-0.5 border-l border-border pl-3">{body}</div>}
      {block.tool_output && <ToolOutputSnippet output={block.tool_output} toolName={tool} />}
    </div>
  );
}

/** ToolOutputSnippet shows what the agent actually saw after a tool
 * call. Walks the output as a sequence of XML-tagged segments + raw
 * text and renders each piece in its proper visual style:
 *
 *   <path>...</path>           → metadata chip
 *   <type>...</type>           → joined with <path> into one chip
 *   <content>1: …(End of file…) → code block, numbered, with line-count summary
 *   <entries>… (N entries)…</entries>  → entry list with count summary
 *   <shell_metadata>…</shell_metadata> → small amber notice
 *   <error>…</error>           → small red notice
 *   anything else              → unwrap, render body as plain text
 *   raw text between/outside   → stdout, expandable
 *
 * Unknown tags don't disappear — they're stripped of their wrapping
 * but the inner body still renders, so we never silently swallow
 * agent-visible output. */
function ToolOutputSnippet({ output, toolName }: { output: string; toolName: string }) {
  const [expanded, setExpanded] = useState(false);
  const segments = parseToolOutput(output);

  // Walk the segments and pre-compute the path/type header chip; we
  // collapse <path> + <type> into a single chip rather than rendering
  // them as two separate boxes, since opencode always emits them
  // together.
  let headerChip: string | null = null;
  const pathSeg = segments.find((s) => s.tag === 'path');
  const typeSeg = segments.find((s) => s.tag === 'type');
  if (pathSeg) headerChip = pathSeg.body;
  if (pathSeg && typeSeg) headerChip = `${typeSeg.body} · ${pathSeg.body}`;
  else if (typeSeg) headerChip = typeSeg.body;

  // The expand-vs-collapse rule applies per snippet, not per segment;
  // we measure the textual mass of segments that render as
  // long-form bodies (content, entries, raw stdout) and gate the
  // collapse behaviour off that.
  const longBodies = segments.filter(
    (s) =>
      s.tag === 'content' ||
      s.tag === 'entries' ||
      (!s.tag && s.body.trim().length > 0),
  );
  const totalLines = longBodies.reduce(
    (n, s) => n + s.body.split('\n').length,
    0,
  );
  const INLINE_LIMIT = toolName.toLowerCase() === 'read' ? 6 : 4;
  const overflowing = totalLines > INLINE_LIMIT;
  // When we have <content> / <entries> tags the limit applies to the
  // structured body. For loose stdout we just clip the raw text.
  // Either way, expanded === true shows everything.

  // Segments we actively want to suppress when not expanded (e.g.
  // very long content). Tracking which lines we've already "spent"
  // lets us clip across segments consistently.
  let remaining = expanded ? Infinity : INLINE_LIMIT;
  function takeLines(body: string): { visible: string; hiddenLines: number } {
    if (remaining === Infinity) return { visible: body, hiddenLines: 0 };
    const lines = body.split('\n');
    if (lines.length <= remaining) {
      const out = { visible: lines.join('\n'), hiddenLines: 0 };
      remaining -= lines.length;
      return out;
    }
    const visible = lines.slice(0, remaining).join('\n');
    const hidden = lines.length - remaining;
    remaining = 0;
    return { visible, hiddenLines: hidden };
  }

  return (
    <div
      className="ml-0.5 space-y-1 border-l border-success/40 pl-3"
      onClick={(e) => e.stopPropagation()}
    >
      {headerChip && (
        <div className="px-2 pt-1 text-xs uppercase tracking-wider text-fg-subtle">
          {headerChip}
        </div>
      )}
      {segments.map((seg, i) => {
        if (seg.tag === 'path' || seg.tag === 'type') return null;
        if (seg.tag === 'shell_metadata') {
          return (
            <div
              key={i}
              className="rounded-sm border border-warning/40 bg-warning/10 px-2 py-1 text-xs leading-[1.55] text-warning-fg"
            >
              {seg.body}
            </div>
          );
        }
        if (seg.tag === 'error') {
          return (
            <div
              key={i}
              className="rounded-sm border border-danger/40 bg-danger/10 px-2 py-1 font-mono text-xs leading-[1.55] text-danger-fg"
            >
              {seg.body}
            </div>
          );
        }
        if (seg.tag === 'content' || seg.tag === 'entries') {
          const { body, footer } = splitTrailingCount(seg.body);
          const { visible } = takeLines(body);
          return (
            <div key={i}>
              {footer && (
                <div className="px-2 text-xs uppercase tracking-wider text-fg-subtle">
                  {footer}
                </div>
              )}
              <pre className="overflow-auto whitespace-pre-wrap bg-success/5 px-2 py-1 font-mono text-xs leading-[1.55] text-fg">
                {visible}
              </pre>
            </div>
          );
        }
        // Untagged stdout / unknown tag. Body is still rendered
        // (never silently dropped); when the tag is unrecognized
        // we surface a small "unhandled tag" pill so future-us
        // notices it and adds a bespoke renderer to
        // KNOWN_TOOL_OUTPUT_TAGS + this switch.
        const { visible } = takeLines(seg.body);
        if (!visible.trim() && !seg.tag) return null;
        const isUnknownTag = !!seg.tag && !KNOWN_TOOL_OUTPUT_TAGS.has(seg.tag);
        return (
          <div key={i} className="space-y-0.5">
            {isUnknownTag && (
              <span
                title={`Opencode emitted <${seg.tag}> but the Inspector has no bespoke renderer for it yet. Body is shown below verbatim; add a case in blocks.tsx ToolOutputSnippet to style it.`}
                className="inline-flex items-center gap-1 rounded-sm border border-warning/40 bg-warning/10 px-1.5 py-0.5 font-mono text-xs text-warning-fg"
              >
                <span aria-hidden>⚠</span>
                <span>unhandled tag: &lt;{seg.tag}&gt;</span>
              </span>
            )}
            {visible.trim() && (
              <pre className="overflow-auto whitespace-pre-wrap bg-success/5 px-2 py-1 font-mono text-xs leading-[1.55] text-fg-muted">
                {visible}
              </pre>
            )}
          </div>
        );
      })}
      {overflowing && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            setExpanded((v) => !v);
          }}
          className="px-2 text-xs font-medium text-fg-subtle underline-offset-2 hover:text-accent hover:underline"
        >
          {expanded
            ? '− collapse'
            : `+ ${totalLines - INLINE_LIMIT} more line${totalLines - INLINE_LIMIT === 1 ? '' : 's'}`}
        </button>
      )}
    </div>
  );
}

/** parseToolOutput walks opencode's tool output as a sequence of
 * tagged blocks + interleaved raw text. Tag names are matched as
 * `[A-Za-z_]\w*` so both single-word (`<path>`) and snake_case
 * (`<shell_metadata>`) shapes work. Nested same-name tags are
 * non-greedy so the closing match is the nearest one. */
function parseToolOutput(raw: string): Array<{ tag?: string; body: string }> {
  const out: Array<{ tag?: string; body: string }> = [];
  const re = /<([A-Za-z_]\w*)>([\s\S]*?)<\/\1>/g;
  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(raw)) !== null) {
    if (m.index > last) {
      out.push({ body: raw.slice(last, m.index) });
    }
    out.push({ tag: m[1], body: m[2] });
    last = m.index + m[0].length;
  }
  if (last < raw.length) out.push({ body: raw.slice(last) });
  return out
    .map((s) => ({ tag: s.tag, body: stripOuterNewlines(s.body) }))
    .filter((s) => s.tag || s.body.trim().length > 0);
}

function stripOuterNewlines(s: string): string {
  return s.replace(/^\n+/, '').replace(/\n+$/, '');
}

/** Pulls out opencode's trailing count line — "(End of file - total
 * 37 lines)" or "(4 entries)" — and returns the body without it
 * plus a normalized summary chip. Both forms appear naturally inside
 * <content> and <entries> segments. */
function splitTrailingCount(body: string): { body: string; footer?: string } {
  const fileFooter = body.match(/\n*\(End of file\s*[—–-]\s*total (\d+) lines\)\s*$/);
  if (fileFooter) {
    return {
      body: body.slice(0, fileFooter.index).replace(/\n+$/, ''),
      footer: `${fileFooter[1]} lines`,
    };
  }
  const dirFooter = body.match(/\n*\((\d+) entries\)\s*$/);
  if (dirFooter) {
    return {
      body: body.slice(0, dirFooter.index).replace(/\n+$/, ''),
      footer: `${dirFooter[1]} entries`,
    };
  }
  return { body };
}

// KNOWN_TOOL_OUTPUT_TAGS is the set of opencode tool-output XML tags
// we render with bespoke styling. Anything OUTSIDE this set still
// renders (body isn't dropped), but it's surfaced with an
// "⚠ unhandled tag" pill so we notice it and add a renderer.
// Add new tag names here AND a matching branch in ToolOutputSnippet
// — the pill is a forcing function so we never silently drop info.
const KNOWN_TOOL_OUTPUT_TAGS = new Set([
  'path',
  'type',
  'content',
  'entries',
  'shell_metadata',
  'error',
]);

// Compact inline summary for the tool-card header so a single-line
// agentic feed reads at a glance: "● read app/user_service.py",
// "● glob <pattern> in /workspace", "● grep \"lock\" in app/".
// Returns empty when the tool's input doesn't lend itself to a
// one-liner — the body renderer takes over from there.
function toolHeaderSummary(
  tool: string,
  input: Record<string, unknown> | null,
  filesTouched: string[] | undefined,
): string {
  if (!input) return '';
  switch (tool) {
    case 'read': {
      const path = stringField(input, 'filePath', 'file_path', 'path');
      return path || (filesTouched?.[0] ?? '');
    }
    case 'glob': {
      const pattern = stringField(input, 'pattern');
      const path = stringField(input, 'path');
      if (pattern && path) return `${pattern} in ${path}`;
      return pattern || path;
    }
    case 'grep': {
      const pattern = stringField(input, 'pattern');
      const path = stringField(input, 'path', 'include');
      if (pattern && path) return `"${pattern}" in ${path}`;
      return pattern ? `"${pattern}"` : path;
    }
    case 'edit':
    case 'write': {
      const path = stringField(input, 'filePath', 'file_path', 'path');
      return path || (filesTouched?.[0] ?? '');
    }
    default:
      return '';
  }
}

/** Pulls the tool's `arguments` envelope out of a content payload.
 * Handles two shapes opencode emits in practice: a bare arguments
 * object (real tool_use events) or the `{"name", "arguments"}`
 * envelope (Ollama-style text-as-tool-call that we promote to
 * tool_use server-side). Returns null for non-JSON content so the
 * caller can fall back to a raw <pre>. */
function parseToolInput(content: string): Record<string, unknown> | null {
  const trimmed = content.trim();
  if (!trimmed.startsWith('{')) return null;
  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch {
    return null;
  }
  if (!parsed || typeof parsed !== 'object') return null;
  const obj = parsed as Record<string, unknown>;
  if (
    typeof obj.name === 'string' &&
    obj.arguments &&
    typeof obj.arguments === 'object'
  ) {
    return obj.arguments as Record<string, unknown>;
  }
  return obj;
}

function stringField(input: Record<string, unknown> | null, ...keys: string[]): string {
  if (!input) return '';
  for (const k of keys) {
    const v = input[k];
    if (typeof v === 'string' && v) return v;
  }
  return '';
}

function renderToolBody(
  tool: string,
  input: Record<string, unknown> | null,
  fallback: string,
) {
  // edit — render a GitHub-style unified diff (red − / green + lines
  // with unchanged context) so the user sees the actual delta instead
  // of two raw blobs.
  if (tool === 'edit' && input) {
    const oldStr = stringField(input, 'oldString', 'old_string');
    const newStr = stringField(input, 'newString', 'new_string');
    if (oldStr || newStr) {
      return <UnifiedDiff oldStr={oldStr} newStr={newStr} />;
    }
  }
  // write — code-block style preview of what the agent dumped.
  if (tool === 'write' && input) {
    const content = stringField(input, 'content', 'text');
    if (content) {
      return <CodeBlock body={content} />;
    }
  }
  // read — filePath chip in the header is the whole story; suppress
  // the body so the turn stays compact.
  if (tool === 'read') {
    return null;
  }
  // bash / shell — single-line `$ command` rendered as a dark terminal
  // block. Multi-line scripts stay legible (wrap, no horizontal scroll).
  if (tool === 'bash' || tool === 'shell') {
    const cmd = stringField(input, 'command', 'cmd', 'script');
    if (cmd) {
      return <TerminalBlock command={cmd} />;
    }
  }
  // glob / grep — fully expressed in the header summary
  // (`● glob **/*.py in /workspace`). No body needed.
  if (tool === 'glob' || tool === 'grep') {
    return null;
  }
  // Fallback: pretty-print whatever JSON we parsed; if parsing
  // failed, dump the raw string. Never lose information.
  const body = input ? JSON.stringify(input, null, 2) : fallback;
  return (
    <pre className="overflow-auto whitespace-pre-wrap rounded-sm bg-code-bg p-2 font-mono text-xs text-fg">
      {body}
    </pre>
  );
}

/** Line-level diff between oldStr and newStr. Uses common-prefix +
 * common-suffix trimming (no full LCS) — good enough for the small,
 * localized edits opencode's `edit` tool produces, and dependency-free. */
function diffLines(
  oldStr: string,
  newStr: string,
): Array<{ kind: '+' | '-' | ' '; text: string }> {
  const o = oldStr.split('\n');
  const n = newStr.split('\n');
  let p = 0;
  while (p < o.length && p < n.length && o[p] === n[p]) p++;
  let so = o.length;
  let sn = n.length;
  while (so > p && sn > p && o[so - 1] === n[sn - 1]) {
    so--;
    sn--;
  }
  const out: Array<{ kind: '+' | '-' | ' '; text: string }> = [];
  for (let i = 0; i < p; i++) out.push({ kind: ' ', text: o[i] });
  for (let i = p; i < so; i++) out.push({ kind: '-', text: o[i] });
  for (let i = p; i < sn; i++) out.push({ kind: '+', text: n[i] });
  for (let i = so; i < o.length; i++) out.push({ kind: ' ', text: o[i] });
  return out;
}

function UnifiedDiff({ oldStr, newStr }: { oldStr: string; newStr: string }) {
  const lines = diffLines(oldStr, newStr);
  // Stamp old/new line numbers as we walk. Removed lines advance only
  // the old counter, added only the new counter, context advances both.
  // A `·` placeholder shows where a side has no number — GitHub does
  // the same. Tabular nums keep the gutter columns aligned even when
  // digits change width.
  let oldNo = 1;
  let newNo = 1;
  const rows = lines.map((l) => {
    const row = {
      kind: l.kind,
      text: l.text,
      old: l.kind === '+' ? null : oldNo,
      new: l.kind === '-' ? null : newNo,
    };
    if (l.kind !== '+') oldNo++;
    if (l.kind !== '-') newNo++;
    return row;
  });
  return (
    <div className="max-h-80 overflow-auto bg-code-bg/60 font-mono text-xs">
      {rows.map((r, i) => (
        <div
          key={i}
          className={cn(
            'flex leading-[1.55]',
            r.kind === '+' && 'bg-success/10 text-success-fg',
            r.kind === '-' && 'bg-danger/10 text-danger-fg',
            r.kind === ' ' && 'text-fg-muted',
          )}
        >
          <span className="w-9 select-none border-r border-border/40 px-1 text-right text-xs tabular-nums text-fg-subtle">
            {r.old ?? '·'}
          </span>
          <span className="w-9 select-none border-r border-border/40 px-1 text-right text-xs tabular-nums text-fg-subtle">
            {r.new ?? '·'}
          </span>
          <span className="w-3 select-none text-center opacity-60">{r.kind}</span>
          <span className="whitespace-pre-wrap break-words pl-1 pr-2">{r.text || ' '}</span>
        </div>
      ))}
    </div>
  );
}

function TerminalBlock({ command }: { command: string }) {
  return (
    <pre className="overflow-auto whitespace-pre-wrap bg-code-bg/60 px-1 py-0.5 font-mono text-xs leading-[1.55] text-fg">
      <span className="select-none text-fg-subtle">$ </span>
      {command}
    </pre>
  );
}

function CodeBlock({ body }: { body: string }) {
  return (
    <pre className="max-h-72 overflow-auto whitespace-pre-wrap bg-code-bg/60 px-1 py-0.5 font-mono text-xs leading-[1.55] text-fg">
      {body}
    </pre>
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

/** TextBlock — assistant prose. Rendered as Markdown (GFM) so the
 * model's bold, inline code, headers, lists, and fenced code blocks
 * surface the way they look in opencode/Claude Code's TUI. */
export function TextBlock({ block }: { block: ParsedTurn }) {
  return <MarkdownProse body={block.content} />;
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

/** MarkdownProse renders agent prose through react-markdown + GFM,
 * styled to fit the dense Inspector timeline rather than a generic
 * docs page: tight leading, monospace inline-code with a faint tint,
 * compact lists, fenced code blocks as inline insets. */
function MarkdownProse({ body, muted = false }: { body: string; muted?: boolean }) {
  return (
    <div
      className={cn(
        'space-y-2 text-sm leading-relaxed [&>*:first-child]:mt-0 [&>*:last-child]:mb-0',
        muted ? 'text-fg-muted' : 'text-fg',
      )}
    >
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          p: ({ children }) => <p className="leading-relaxed">{children}</p>,
          h1: ({ children }) => (
            <h1 className="text-base font-semibold text-fg">{children}</h1>
          ),
          h2: ({ children }) => (
            <h2 className="text-sm font-semibold text-fg">{children}</h2>
          ),
          h3: ({ children }) => (
            <h3 className="text-xs font-semibold uppercase tracking-wider text-fg-muted">
              {children}
            </h3>
          ),
          ul: ({ children }) => <ul className="ml-4 list-disc space-y-1">{children}</ul>,
          ol: ({ children }) => <ol className="ml-4 list-decimal space-y-1">{children}</ol>,
          li: ({ children }) => <li className="leading-relaxed">{children}</li>,
          a: ({ href, children }) => (
            <a
              href={href}
              target="_blank"
              rel="noreferrer"
              className="text-accent underline-offset-2 hover:underline"
            >
              {children}
            </a>
          ),
          code: ({ className, children }) => {
            const isBlock = typeof className === 'string' && className.startsWith('language-');
            if (isBlock) return <code className="font-mono">{children}</code>;
            return (
              <code className="rounded-sm bg-bg-elev-2 px-1 py-0.5 font-mono text-xs text-fg">
                {children}
              </code>
            );
          },
          pre: ({ children }) => (
            <pre className="overflow-auto whitespace-pre-wrap bg-code-bg/60 px-2 py-1.5 font-mono text-xs leading-[1.55] text-fg">
              {children}
            </pre>
          ),
          blockquote: ({ children }) => (
            <blockquote className="border-l-2 border-border pl-3 text-fg-muted">{children}</blockquote>
          ),
          strong: ({ children }) => <strong className="font-semibold text-fg">{children}</strong>,
          em: ({ children }) => <em className="italic">{children}</em>,
          table: ({ children }) => <table className="w-full border-collapse text-xs">{children}</table>,
          th: ({ children }) => (
            <th className="border-b border-border px-2 py-1 text-left font-medium">{children}</th>
          ),
          td: ({ children }) => (
            <td className="border-b border-border/40 px-2 py-1">{children}</td>
          ),
        }}
      >
        {body}
      </ReactMarkdown>
    </div>
  );
}
