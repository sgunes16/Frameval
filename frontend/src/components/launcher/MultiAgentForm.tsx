import { useEffect } from 'react';
import type { MultiAgentConfig, MultiAgentRole } from '../../lib/types';

const MAX_ROLES = 5;

const SEED: MultiAgentConfig = {
  roles: [
    {
      name: 'planner',
      prompt: [
        'You are the PLANNER role in a sequential workflow. Output must be a written plan only — do not write code or modify files yet.',
        'Produce a markdown response with three sections:',
        '',
        '## Approach',
        '(one paragraph: what overall strategy will solve this task?)',
        '',
        '## Files to change',
        '(bulleted list of file paths the implementer should touch)',
        '',
        '## Test strategy',
        '(bulleted list of how the implementer should verify correctness)',
        '',
        'Task:',
        '{{TASK}}',
      ].join('\n'),
    },
    {
      name: 'coder',
      prompt: [
        'You are the CODER role. A previous role has produced an implementation plan; follow it where reasonable, deviate with justification if you find it is wrong.',
        '',
        '## Plan from previous role',
        '{{PREV_OUTPUT}}',
        '',
        '## Task',
        '{{TASK}}',
      ].join('\n'),
    },
  ],
};

interface FormProps {
  value: MultiAgentConfig | undefined;
  onChange: (next: MultiAgentConfig) => void;
}

/**
 * MultiAgentForm — N-role configuration panel rendered inside
 * HarnessConfigPanel when `multiagent` is selected. Each role row has
 * a name input, a prompt textarea, reorder buttons, and a remove
 * button. The form enforces the 1..5 role cap visually (Add disabled
 * at 5; Remove disabled at 1). It does NOT do format validation
 * inline — validateMultiAgentConfig gates the Launch button instead.
 *
 * On first render with undefined value, the form emits a default
 * planner+coder pair via onChange so the parent picks up the seed.
 */
export function MultiAgentForm({ value, onChange }: FormProps) {
  useEffect(() => {
    if (value === undefined) {
      onChange(SEED);
    }
    // Only fire on the mount; once `value` is defined the parent owns it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const roles = value?.roles ?? SEED.roles;

  const update = (next: MultiAgentRole[]) => onChange({ roles: next });
  const editName = (i: number, name: string) =>
    update(roles.map((r, j) => (i === j ? { ...r, name } : r)));
  const editPrompt = (i: number, prompt: string) =>
    update(roles.map((r, j) => (i === j ? { ...r, prompt } : r)));
  const remove = (i: number) => update(roles.filter((_, j) => j !== i));
  const add = () =>
    update([...roles, { name: `role${roles.length + 1}`, prompt: '' }]);
  const move = (i: number, dir: 'up' | 'down') => {
    const j = dir === 'up' ? i - 1 : i + 1;
    if (j < 0 || j >= roles.length) return;
    const next = [...roles];
    [next[i], next[j]] = [next[j], next[i]];
    update(next);
  };

  return (
    <div className="mt-2 rounded-md border border-border bg-bg-elev-1 p-3">
      <div className="mb-2 text-xs uppercase tracking-wider text-fg-muted">
        Multi-agent roles
      </div>
      <p className="mb-3 text-xs text-fg-muted">
        Sequential roles — each runs after the previous one finishes. Available substitutions in prompts:{' '}
        <code className="font-mono text-fg">{'{{TASK}}'}</code> and{' '}
        <code className="font-mono text-fg">{'{{PREV_OUTPUT}}'}</code>.
      </p>
      <div className="space-y-3">
        {roles.map((r, i) => (
          <div key={i} className="rounded-md border border-border bg-bg p-3">
            <div className="mb-2 flex items-center gap-2">
              <span className="text-xs uppercase tracking-wider text-fg-muted">Role {i + 1}</span>
              <input
                aria-label={`Role ${i + 1} name`}
                className="flex-1 rounded-md border border-border bg-bg-elev-1 px-2 py-1 font-mono text-xs text-fg"
                placeholder="planner"
                value={r.name}
                onChange={(e) => editName(i, e.target.value)}
              />
              <button
                type="button"
                aria-label={`Move role ${i + 1} up`}
                disabled={i === 0}
                onClick={() => move(i, 'up')}
                className="rounded-md border border-border bg-bg-elev-1 px-2 py-1 text-xs text-fg disabled:opacity-40"
              >
                ↑
              </button>
              <button
                type="button"
                aria-label={`Move role ${i + 1} down`}
                disabled={i === roles.length - 1}
                onClick={() => move(i, 'down')}
                className="rounded-md border border-border bg-bg-elev-1 px-2 py-1 text-xs text-fg disabled:opacity-40"
              >
                ↓
              </button>
              <button
                type="button"
                aria-label={`Remove role ${i + 1}`}
                disabled={roles.length === 1}
                onClick={() => remove(i)}
                className="rounded-md border border-border bg-bg-elev-1 px-2 py-1 text-xs text-fg disabled:opacity-40"
              >
                Remove
              </button>
            </div>
            <textarea
              aria-label={`Role ${i + 1} prompt`}
              className="min-h-32 w-full rounded-md border border-border bg-bg-elev-1 p-2 font-mono text-xs text-fg"
              placeholder="Prompt template — use {{TASK}} and {{PREV_OUTPUT}}"
              value={r.prompt}
              onChange={(e) => editPrompt(i, e.target.value)}
            />
          </div>
        ))}
      </div>
      <button
        type="button"
        aria-label="Add role"
        onClick={add}
        disabled={roles.length >= MAX_ROLES}
        className="mt-3 rounded-md border border-border bg-bg-elev-2 px-3 py-1.5 text-xs text-fg transition hover:bg-bg-elev-1 disabled:opacity-40"
      >
        {roles.length >= MAX_ROLES ? `Max ${MAX_ROLES} roles` : '+ Add role'}
      </button>
    </div>
  );
}
