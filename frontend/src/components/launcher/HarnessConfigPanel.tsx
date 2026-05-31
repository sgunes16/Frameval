export type HarnessConfigValue = Record<string, unknown>;

interface PanelProps {
  harnessId: string;
  value: HarnessConfigValue | undefined;
  onChange: (next: HarnessConfigValue) => void;
}

/**
 * Per-harness config form. The launcher renders one of these below
 * every selected harness chip. Harnesses that don't need config
 * (bare, ralph) render nothing. Future harnesses (multiagent,
 * speckit) add their own switch case here without touching the
 * launcher page itself.
 */
export function HarnessConfigPanel({ harnessId, value, onChange }: PanelProps) {
  switch (harnessId) {
    case 'agent_instructions':
      return (
        <AgentInstructionsForm
          value={value as { content?: string } | undefined}
          onChange={onChange}
        />
      );
    default:
      return null;
  }
}

function AgentInstructionsForm({
  value,
  onChange,
}: {
  value: { content?: string } | undefined;
  onChange: (next: HarnessConfigValue) => void;
}) {
  return (
    <div className="mt-2 rounded-md border border-border bg-bg-elev-1 p-3">
      <label
        htmlFor="agent-instructions-content"
        className="block text-xs uppercase tracking-wider text-fg-muted"
      >
        Agent instructions (laid down as CLAUDE.md)
      </label>
      <textarea
        id="agent-instructions-content"
        className="mt-1 min-h-32 w-full rounded-md border border-border bg-bg p-2 font-mono text-xs text-fg"
        placeholder={'# Project rules\n\nKeep changes focused...'}
        value={value?.content ?? ''}
        onChange={(e) => onChange({ content: e.target.value })}
      />
    </div>
  );
}
