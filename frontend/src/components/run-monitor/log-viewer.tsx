import type { ReactNode } from 'react';
import { useEffect, useRef } from 'react';
import type { Run } from '../../lib/types';

export type AgentLogEvent = {
  id: number;
  line: string;
  runId?: string;
  runNumber?: number;
  timestamp?: string;
  stage?: string;
};

export function LogViewer({ lines }: { lines: string[] }) {
  const ref = useRef<HTMLPreElement | null>(null);

  useEffect(() => {
    if (ref.current) {
      ref.current.scrollTop = ref.current.scrollHeight;
    }
  }, [lines]);

  return (
    <pre ref={ref} className="min-h-48 max-h-[360px] overflow-auto rounded-lg bg-slate-950 p-4 font-mono text-[11px] leading-5 text-slate-100">
      {lines.length ? lines.join('\n') : 'Waiting for logs...'}
    </pre>
  );
}

export function AgentEventViewer({ events, runs }: { events: AgentLogEvent[]; runs: Run[] }) {
  const ref = useRef<HTMLDivElement | null>(null);
  const parsed = parseAgentEvents(events, runs);

  useEffect(() => {
    if (ref.current) {
      ref.current.scrollTop = ref.current.scrollHeight;
    }
  }, [parsed.items]);

  return (
    <div className="space-y-3">
      <UsageSummary usage={parsed.usage} />
      <div ref={ref} className="max-h-[640px] space-y-2 overflow-auto rounded-xl border border-slate-200 bg-white p-4">
        {parsed.items.length ? (
          parsed.items.map((item) => <TimelineEntry key={item.id} item={item} />)
        ) : (
          <div className="rounded-lg border border-dashed border-slate-300 p-6 text-center text-xs text-slate-400">
            Waiting for agent events...
          </div>
        )}
      </div>
    </div>
  );
}

type ToolMeta = {
  toolKind: string;
  callId?: string;
  path?: string;
  linesAdded?: number;
  linesRemoved?: number;
  command?: string;
  description?: string;
  exitCode?: number;
  pattern?: string;
  matchCount?: number;
  diffString?: string;
  output?: string;
  content?: string;
  totalLines?: number;
};

type TimelineItem = {
  id: string;
  kind: 'assistant' | 'thinking' | 'tool' | 'result' | 'raw';
  title: string;
  body: string;
  toolMeta?: ToolMeta;
  status?: string;
  runNumber?: number;
  timestamp?: string;
};

type Usage = {
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  liveOutputTokens: number;
  totalTokens: number;
  durationMs: number;
};

function UsageSummary({ usage }: { usage: Usage }) {
  const cells = [
    ['Input', usage.inputTokens],
    ['Output', usage.outputTokens],
    ['Live out', usage.liveOutputTokens],
    ['Cache R', usage.cacheReadTokens],
    ['Cache W', usage.cacheWriteTokens],
    ['Total', usage.totalTokens],
  ];
  return (
    <div className="grid gap-2 sm:grid-cols-6">
      {cells.map(([label, value]) => (
        <div key={label} className="rounded-lg border border-slate-200 bg-white px-3 py-2">
          <div className="text-[10px] uppercase tracking-wide text-slate-400">{label}</div>
          <div className="mt-0.5 text-sm font-semibold tabular-nums text-slate-900">{Number(value).toLocaleString()}</div>
        </div>
      ))}
    </div>
  );
}

function TimelineEntry({ item }: { item: TimelineItem }) {
  if (item.kind === 'thinking') return <ThinkingCard item={item} />;
  if (item.kind === 'tool') return <ToolCard item={item} />;
  if (item.kind === 'assistant') return <AssistantCard item={item} />;
  if (item.kind === 'result') return <ResultCard item={item} />;
  return <RawCard item={item} />;
}

function ThinkingCard({ item }: { item: TimelineItem }) {
  const wordCount = item.body.split(/\s+/).filter(Boolean).length;
  const isDone = item.status === 'completed';
  const label = isDone ? `Thought for ${wordCount} words` : 'Thinking...';
  return (
    <details className="group">
      <summary className="flex cursor-pointer list-none items-center gap-2 rounded-lg px-3 py-1.5 text-xs text-slate-500 transition hover:bg-slate-50">
        <ChevronIcon />
        <span>{label}</span>
        {!isDone && <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-amber-400" />}
      </summary>
      <pre className="mx-3 mt-1 mb-2 max-h-60 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-slate-50 p-3 font-mono text-[11px] leading-5 text-slate-500">
        {item.body}
      </pre>
    </details>
  );
}

function AssistantCard({ item }: { item: TimelineItem }) {
  return (
    <div className="py-1.5">
      <MarkdownLite content={item.body} />
    </div>
  );
}

function ResultCard({ item }: { item: TimelineItem }) {
  const isError = item.status === 'failed';
  return (
    <div className={`rounded-lg border px-3 py-2 text-xs ${isError ? 'border-red-200 bg-red-50 text-red-700' : 'border-emerald-200 bg-emerald-50 text-emerald-700'}`}>
      <span className="font-medium">{isError ? 'Error' : 'Result'}</span>
      {item.body && (
        <pre className="mt-1 whitespace-pre-wrap break-words font-mono text-[11px] leading-5 opacity-80">{truncate(item.body, 2000)}</pre>
      )}
    </div>
  );
}

function RawCard({ item }: { item: TimelineItem }) {
  const parsed = parseJSONLine(item.body);
  if (parsed) return <JSONLite data={parsed} />;
  return (
    <pre className="whitespace-pre-wrap break-words rounded-lg bg-slate-50 px-3 py-2 font-mono text-[11px] leading-5 text-slate-600">
      {item.body}
    </pre>
  );
}

function ToolCard({ item }: { item: TimelineItem }) {
  const meta = item.toolMeta;
  if (!meta) {
    return (
      <details className="group">
        <summary className="flex cursor-pointer list-none items-center gap-2 rounded-lg px-3 py-1.5 text-xs text-slate-600 transition hover:bg-slate-50">
          <ChevronIcon />
          <ToolIcon kind="tool" />
          <span className="font-medium">{item.title}</span>
          <StatusDot status={item.status} />
        </summary>
        <pre className="mx-3 mt-1 mb-2 max-h-72 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-slate-50 p-3 font-mono text-[11px] leading-5 text-slate-600">
          {item.body}
        </pre>
      </details>
    );
  }

  if (meta.toolKind === 'edit') return <EditToolCard item={item} meta={meta} />;
  if (meta.toolKind === 'shell') return <ShellToolCard item={item} meta={meta} />;
  if (meta.toolKind === 'search') return <SearchToolCard item={item} meta={meta} />;
  if (meta.toolKind === 'read') return <ReadToolCard item={item} meta={meta} />;

  return (
    <details className="group">
      <summary className="flex cursor-pointer list-none items-center gap-2 rounded-lg px-3 py-1.5 text-xs text-slate-600 transition hover:bg-slate-50">
        <ChevronIcon />
        <ToolIcon kind={meta.toolKind} />
        <span className="font-medium">{item.title}</span>
        <StatusDot status={item.status} />
      </summary>
      <pre className="mx-3 mt-1 mb-2 max-h-72 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-slate-50 p-3 font-mono text-[11px] leading-5 text-slate-600">
        {item.body}
      </pre>
    </details>
  );
}

function EditToolCard({ item, meta }: { item: TimelineItem; meta: ToolMeta }) {
  const fileName = meta.path?.split('/').pop() ?? meta.path ?? 'file';
  const added = meta.linesAdded ?? 0;
  const removed = meta.linesRemoved ?? 0;
  const diffText = meta.diffString ?? meta.content ?? item.body;

  return (
    <details className="group overflow-hidden rounded-lg border border-slate-200">
      <summary className="flex cursor-pointer list-none items-center gap-2 bg-slate-50 px-3 py-2 text-xs text-slate-700 transition hover:bg-slate-100">
        <ChevronIcon />
        <ToolIcon kind="edit" />
        <span className="font-mono font-medium">{fileName}</span>
        {(added > 0 || removed > 0) && (
          <span className="ml-auto flex items-center gap-1.5 text-[11px]">
            {added > 0 && <span className="font-medium text-emerald-600">+{added}</span>}
            {removed > 0 && <span className="font-medium text-red-500">-{removed}</span>}
          </span>
        )}
        <StatusDot status={item.status} />
      </summary>
      {meta.path && (
        <div className="border-b border-slate-200 bg-slate-50 px-3 py-1 font-mono text-[10px] text-slate-400">
          {meta.path}
        </div>
      )}
      <DiffView diff={diffText} />
    </details>
  );
}

function ShellToolCard({ item, meta }: { item: TimelineItem; meta: ToolMeta }) {
  const desc = meta.description ?? meta.command ?? 'shell';
  const exitCode = meta.exitCode;
  const output = meta.output ?? item.body;
  const isError = exitCode !== undefined && exitCode !== 0;

  return (
    <details className="group overflow-hidden rounded-lg border border-slate-200">
      <summary className="flex cursor-pointer list-none items-center gap-2 bg-slate-50 px-3 py-2 text-xs text-slate-700 transition hover:bg-slate-100">
        <ChevronIcon />
        <ToolIcon kind="shell" />
        <span className="font-medium">{desc}</span>
        {exitCode !== undefined && (
          <span className={`ml-auto font-mono text-[11px] ${isError ? 'text-red-500' : 'text-emerald-600'}`}>
            exit {exitCode}
          </span>
        )}
        <StatusDot status={item.status} />
      </summary>
      {meta.command && (
        <div className="border-b border-slate-800 bg-slate-900 px-3 py-1.5 font-mono text-[11px] text-emerald-400">
          $ {meta.command}
        </div>
      )}
      <pre className="max-h-72 overflow-auto whitespace-pre-wrap break-words bg-slate-950 p-3 font-mono text-[11px] leading-5 text-slate-300">
        {output || 'No output.'}
      </pre>
    </details>
  );
}

function SearchToolCard({ item, meta }: { item: TimelineItem; meta: ToolMeta }) {
  const pattern = meta.pattern ?? '';
  const matchCount = meta.matchCount;

  return (
    <details className="group overflow-hidden rounded-lg border border-slate-200">
      <summary className="flex cursor-pointer list-none items-center gap-2 bg-slate-50 px-3 py-2 text-xs text-slate-700 transition hover:bg-slate-100">
        <ChevronIcon />
        <ToolIcon kind="search" />
        <span className="font-medium">Search</span>
        <code className="rounded bg-slate-200 px-1.5 py-0.5 font-mono text-[11px] text-slate-600">{pattern}</code>
        {matchCount !== undefined && (
          <span className="ml-auto text-[11px] text-slate-400">{matchCount} match{matchCount !== 1 ? 'es' : ''}</span>
        )}
        <StatusDot status={item.status} />
      </summary>
      {meta.path && (
        <div className="border-b border-slate-200 bg-slate-50 px-3 py-1 font-mono text-[10px] text-slate-400">
          {meta.path}
        </div>
      )}
      <pre className="max-h-72 overflow-auto whitespace-pre-wrap break-words bg-slate-50 p-3 font-mono text-[11px] leading-5 text-slate-600">
        {meta.content || item.body || 'No matches.'}
      </pre>
    </details>
  );
}

function ReadToolCard({ item, meta }: { item: TimelineItem; meta: ToolMeta }) {
  const fileName = meta.path?.split('/').pop() ?? 'file';
  const totalLines = meta.totalLines;

  return (
    <details className="group overflow-hidden rounded-lg border border-slate-200">
      <summary className="flex cursor-pointer list-none items-center gap-2 bg-slate-50 px-3 py-2 text-xs text-slate-700 transition hover:bg-slate-100">
        <ChevronIcon />
        <ToolIcon kind="read" />
        <span className="font-mono font-medium">{fileName}</span>
        {totalLines !== undefined && (
          <span className="ml-auto text-[11px] text-slate-400">{totalLines} lines</span>
        )}
        <StatusDot status={item.status} />
      </summary>
      {meta.path && (
        <div className="border-b border-slate-200 bg-slate-50 px-3 py-1 font-mono text-[10px] text-slate-400">
          {meta.path}
        </div>
      )}
      <pre className="max-h-72 overflow-auto whitespace-pre-wrap break-words bg-white p-3 font-mono text-[11px] leading-5 text-slate-700">
        {meta.content ?? (item.body || 'Empty.')}
      </pre>
    </details>
  );
}

function DiffView({ diff }: { diff: string }) {
  const lines = diff.split('\n');
  return (
    <div className="max-h-80 overflow-auto bg-slate-950 font-mono text-[11px] leading-5">
      {lines.map((line, i) => {
        let cls = 'text-slate-400';
        let bg = '';
        if (line.startsWith('+') && !line.startsWith('+++')) {
          cls = 'text-emerald-300';
          bg = 'bg-emerald-500/10';
        } else if (line.startsWith('-') && !line.startsWith('---')) {
          cls = 'text-red-400';
          bg = 'bg-red-500/10';
        } else if (line.startsWith('@@')) {
          cls = 'text-blue-400';
          bg = 'bg-blue-500/5';
        }
        return (
          <div key={i} className={`whitespace-pre-wrap break-words px-3 ${cls} ${bg}`}>
            {line || '\u00A0'}
          </div>
        );
      })}
    </div>
  );
}

function MarkdownLite({ content }: { content: string }) {
  const lines = content.split('\n');
  const elements: ReactNode[] = [];
  let codeBuffer: string[] = [];
  let inCode = false;

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    if (/^\s*```/.test(line)) {
      if (inCode) {
        elements.push(
          <pre key={`code-${i}`} className="my-1 overflow-auto rounded-lg bg-slate-950 p-3 font-mono text-[11px] leading-5 text-slate-200">
            {codeBuffer.join('\n')}
          </pre>
        );
        codeBuffer = [];
      }
      inCode = !inCode;
      continue;
    }
    if (inCode) {
      codeBuffer.push(line);
      continue;
    }
    if (line.startsWith('### ')) {
      elements.push(<div key={i} className="pt-2 text-sm font-semibold text-slate-900">{line.slice(4)}</div>);
    } else if (line.startsWith('## ')) {
      elements.push(<div key={i} className="pt-2 text-base font-semibold text-slate-900">{line.slice(3)}</div>);
    } else if (line.startsWith('# ')) {
      elements.push(<div key={i} className="pt-3 text-lg font-semibold text-slate-900">{line.slice(2)}</div>);
    } else if (line.trim().startsWith('- ') || line.trim().startsWith('* ')) {
      elements.push(<div key={i} className="pl-4 text-sm leading-6 text-slate-700">• {line.trim().slice(2)}</div>);
    } else if (/^\d+\.\s/.test(line.trim())) {
      elements.push(<div key={i} className="pl-4 text-sm leading-6 text-slate-700">{line.trim()}</div>);
    } else if (!line.trim()) {
      elements.push(<div key={i} className="h-1.5" />);
    } else {
      elements.push(<p key={i} className="text-sm leading-6 text-slate-700">{renderInlineCode(line)}</p>);
    }
  }

  if (codeBuffer.length > 0) {
    elements.push(
      <pre key="code-end" className="my-1 overflow-auto rounded-lg bg-slate-950 p-3 font-mono text-[11px] leading-5 text-slate-200">
        {codeBuffer.join('\n')}
      </pre>
    );
  }

  return <div className="space-y-0.5">{elements}</div>;
}

function renderInlineCode(text: string) {
  const parts = text.split(/(`[^`]+`)/g);
  return parts.map((part, i) => {
    if (part.startsWith('`') && part.endsWith('`')) {
      return (
        <code key={i} className="rounded bg-slate-100 px-1 py-0.5 font-mono text-[12px] text-indigo-600">
          {part.slice(1, -1)}
        </code>
      );
    }
    return <span key={i}>{part}</span>;
  });
}

function JSONLite({ data }: { data: Record<string, unknown> }) {
  return (
    <div className="space-y-0.5 rounded-lg bg-slate-50 p-2 font-mono text-[11px] text-slate-700">
      {Object.entries(data).slice(0, 12).map(([key, value]) => (
        <div key={key} className="flex gap-2 rounded px-2 py-0.5">
          <span className="shrink-0 text-slate-400">{key}:</span>
          <span className="break-words text-slate-700">{formatJSONValue(value)}</span>
        </div>
      ))}
    </div>
  );
}

function ChevronIcon() {
  return (
    <svg className="h-3 w-3 shrink-0 text-slate-400 transition group-open:rotate-90" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
    </svg>
  );
}

function ToolIcon({ kind }: { kind: string }) {
  const icons: Record<string, string> = {
    edit: '✎',
    shell: '▸',
    search: '⌕',
    read: '📄',
    tool: '⚙',
  };
  return <span className="text-xs text-slate-400">{icons[kind] ?? icons.tool}</span>;
}

function StatusDot({ status }: { status?: string }) {
  if (!status) return null;
  if (status === 'completed') return <span className="ml-auto h-1.5 w-1.5 shrink-0 rounded-full bg-emerald-500" title="done" />;
  if (status === 'failed') return <span className="ml-auto h-1.5 w-1.5 shrink-0 rounded-full bg-red-500" title="failed" />;
  return <span className="ml-auto h-1.5 w-1.5 shrink-0 animate-pulse rounded-full bg-amber-400" title="running" />;
}

export function parseAgentEvents(events: AgentLogEvent[], runs: Run[]): { items: TimelineItem[]; usage: Usage } {
  const items: TimelineItem[] = [];
  const usage: Usage = {
    inputTokens: 0,
    outputTokens: 0,
    cacheReadTokens: 0,
    cacheWriteTokens: 0,
    liveOutputTokens: 0,
    totalTokens: runs.reduce((sum, run) => sum + (run.grade?.total_tokens ?? run.transcript?.total_tokens ?? 0), 0),
    durationMs: 0,
  };

  for (const event of events) {
    const parsed = parseJSONLine(event.line);
    if (!parsed) {
      appendItem(items, {
        id: String(event.id),
        kind: 'raw',
        title: 'raw',
        body: event.line,
        runNumber: event.runNumber,
        timestamp: event.timestamp,
      });
      continue;
    }

    const eventUsage = readUsage(parsed);
    if (eventUsage) {
      usage.inputTokens += eventUsage.inputTokens;
      usage.outputTokens += eventUsage.outputTokens;
      usage.cacheReadTokens += eventUsage.cacheReadTokens;
      usage.cacheWriteTokens += eventUsage.cacheWriteTokens;
      usage.totalTokens += eventUsage.inputTokens + eventUsage.outputTokens + eventUsage.cacheReadTokens + eventUsage.cacheWriteTokens;
      usage.durationMs += eventUsage.durationMs;
    }

    const item = toTimelineItem(parsed, event);
    if (item) {
      appendItem(items, item);
    }
  }

  usage.liveOutputTokens = items.reduce((sum, item) => item.kind === 'assistant' ? sum + estimateTokens(item.body) : sum, 0);
  if (usage.totalTokens === 0) {
    usage.totalTokens = usage.inputTokens + usage.outputTokens + usage.cacheReadTokens + usage.cacheWriteTokens + usage.liveOutputTokens;
  }
  return { items, usage };
}

function appendItem(items: TimelineItem[], item: TimelineItem) {
  const previous = items[items.length - 1];

  if (
    previous &&
    previous.kind === item.kind &&
    (item.kind === 'assistant' || item.kind === 'thinking') &&
    previous.runNumber === item.runNumber
  ) {
    previous.body = mergeStreamingBody(previous.body, item.body);
    previous.status = item.status ?? previous.status;
    previous.timestamp = item.timestamp ?? previous.timestamp;
    return;
  }

  // Plain-text executors (Aider) emit non-JSON lines that fall through to
  // the `raw` kind. Each line otherwise becomes its own stub card, producing
  // dozens of single-line cards in the timeline. Concatenate consecutive
  // raw lines from the same run into a single multi-line card so RawCard's
  // <pre whitespace-pre-wrap> renders them as a clean block.
  if (
    previous &&
    previous.kind === 'raw' &&
    item.kind === 'raw' &&
    previous.runNumber === item.runNumber
  ) {
    previous.body = previous.body ? previous.body + '\n' + item.body : item.body;
    previous.timestamp = item.timestamp ?? previous.timestamp;
    return;
  }

  if (
    item.kind === 'tool' &&
    item.toolMeta?.callId
  ) {
    const existing = items.find(
      (prev) => prev.kind === 'tool' && prev.toolMeta?.callId === item.toolMeta?.callId
    );
    if (existing) {
      existing.status = item.status ?? existing.status;
      if (item.toolMeta) {
        existing.toolMeta = { ...existing.toolMeta, ...item.toolMeta };
      }
      existing.body = item.body || existing.body;
      return;
    }
  }

  items.push(item);
}

function mergeStreamingBody(previousBody: string, nextBody: string) {
  if (!nextBody) return previousBody;
  if (!previousBody) return nextBody;
  if (previousBody === nextBody) return previousBody;
  if (nextBody.startsWith(previousBody)) return nextBody;
  if (previousBody.endsWith(nextBody)) return previousBody;
  return previousBody + nextBody;
}

function parseJSONLine(line: string): Record<string, unknown> | null {
  try {
    const parsed = JSON.parse(line) as unknown;
    return parsed && typeof parsed === 'object' ? (parsed as Record<string, unknown>) : null;
  } catch {
    return null;
  }
}

function toTimelineItem(event: Record<string, unknown>, source: AgentLogEvent): TimelineItem | null {
  const type = String(event.type ?? 'raw');
  if (type === 'assistant') {
    return {
      id: String(source.id),
      kind: 'assistant',
      title: 'assistant',
      body: extractAssistantText(event),
      runNumber: source.runNumber,
      timestamp: source.timestamp,
    };
  }
  if (type === 'thinking') {
    return {
      id: String(source.id),
      kind: 'thinking',
      title: 'thinking',
      status: String(event.subtype ?? 'streaming'),
      body: String(event.text ?? ''),
      runNumber: source.runNumber,
      timestamp: source.timestamp,
    };
  }
  if (type === 'tool_call') {
    const callId = str(event.call_id);
    const meta = extractToolMeta(event);
    if (callId) meta.callId = callId;
    return {
      id: callId || String(source.id),
      kind: 'tool',
      title: meta.toolKind,
      status: String(event.subtype ?? 'running'),
      body: toolBodyFallback(event),
      toolMeta: meta,
      runNumber: source.runNumber,
      timestamp: source.timestamp,
    };
  }
  if (type === 'result') {
    return {
      id: String(source.id),
      kind: 'result',
      title: 'result',
      status: bool(event.is_error) ? 'failed' : String(event.subtype ?? 'completed'),
      body: String(event.result ?? ''),
      runNumber: source.runNumber,
      timestamp: source.timestamp,
    };
  }
  return null;
}

function extractAssistantText(event: Record<string, unknown>): string {
  const message = obj(event.message);
  const content = Array.isArray(message?.content) ? message.content : [];
  return content.map((part) => (obj(part)?.text ? String(obj(part)?.text) : '')).join('');
}

function extractToolMeta(event: Record<string, unknown>): ToolMeta {
  const call = obj(event.tool_call);
  if (!call) return { toolKind: 'tool' };

  const shell = obj(call.shellToolCall);
  if (shell) {
    const args = obj(shell.args);
    const result = obj(shell.result);
    const success = obj(result?.success);
    const failure = obj(result?.failure);
    return {
      toolKind: 'shell',
      command: str(args?.command),
      description: str(shell.description),
      exitCode: num(success?.exitCode ?? failure?.exitCode),
      output: str(success?.interleavedOutput ?? failure?.interleavedOutput ?? success?.stdout ?? failure?.stdout ?? ''),
    };
  }

  const edit = obj(call.editToolCall);
  if (edit) {
    const args = obj(edit.args);
    const result = obj(edit.result);
    const success = obj(result?.success);
    return {
      toolKind: 'edit',
      path: str(success?.path ?? args?.path),
      linesAdded: num(success?.linesAdded),
      linesRemoved: num(success?.linesRemoved),
      diffString: str(success?.diffString ?? ''),
      content: str(args?.streamContent ?? ''),
    };
  }

  const grep = obj(call.grepToolCall);
  if (grep) {
    const args = obj(grep.args);
    const result = obj(grep.result);
    const success = obj(result?.success);
    const workspaceResults = obj(success?.workspaceResults);
    let matchCount = 0;
    let body = '';
    if (workspaceResults) {
      const entries = Object.entries(workspaceResults);
      const parts: string[] = [];
      for (const [, wsVal] of entries) {
        const wsObj = obj(wsVal);
        const content = obj(wsObj?.content);
        const matches = Array.isArray(content?.matches) ? content.matches : [];
        matchCount += matches.length;
        for (const m of matches.slice(0, 8)) {
          const mObj = obj(m);
          const file = str(mObj?.file ?? '');
          const lineMatches = Array.isArray(mObj?.matches) ? mObj.matches : [];
          parts.push(file);
          for (const lm of lineMatches.slice(0, 4)) {
            const lmObj = obj(lm);
            parts.push(`  ${lmObj?.lineNumber ?? '?'}: ${str(lmObj?.content ?? '').trim()}`);
          }
        }
      }
      body = parts.join('\n');
    }
    return {
      toolKind: 'search',
      pattern: str(args?.pattern),
      path: str(args?.path),
      matchCount,
      content: body,
    };
  }

  const read = obj(call.readToolCall);
  if (read) {
    const args = obj(read.args);
    const result = obj(read.result);
    const success = obj(result?.success);
    return {
      toolKind: 'read',
      path: str(success?.path ?? args?.path),
      totalLines: num(success?.totalLines),
      content: str(success?.content ?? ''),
    };
  }

  const toolName = Object.keys(call).find((key) => key.endsWith('ToolCall'));
  return { toolKind: toolName ? toolName.replace(/ToolCall$/, '').replace(/([A-Z])/g, ' $1').trim().toLowerCase() : 'tool' };
}

function toolBodyFallback(event: Record<string, unknown>): string {
  return JSON.stringify(event, null, 2);
}

function readUsage(event: Record<string, unknown>): Usage | null {
  const usage = obj(event.usage);
  if (!usage) return null;
  return {
    inputTokens: num(usage.inputTokens),
    outputTokens: num(usage.outputTokens),
    cacheReadTokens: num(usage.cacheReadTokens),
    cacheWriteTokens: num(usage.cacheWriteTokens),
    liveOutputTokens: 0,
    totalTokens: 0,
    durationMs: num(event.duration_ms),
  };
}

function obj(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : null;
}

function bool(value: unknown) {
  return value === true;
}

function num(value: unknown) {
  return typeof value === 'number' ? value : 0;
}

function str(value: unknown) {
  if (value === undefined || value === null) return '';
  return String(value);
}

function truncate(value: string, max: number) {
  return value.length > max ? `${value.slice(0, max)}\n...` : value;
}

function formatJSONValue(value: unknown) {
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  if (value === null || value === undefined) return 'null';
  return truncate(JSON.stringify(value, null, 2), 400);
}

function estimateTokens(text: string) {
  const compactText = text.trim().replace(/\s+/g, ' ');
  return compactText ? Math.ceil(compactText.length / 4) : 0;
}
