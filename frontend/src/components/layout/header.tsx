import { useLocation } from 'react-router-dom';
import { ThemeToggle } from './theme-toggle';

const titles: Array<{ match: RegExp; title: string; description: string }> = [
  { match: /^\/$/, title: 'Dashboard', description: 'Snapshot of your experiments and evaluation artifacts.' },
  { match: /^\/experiments\/[^/]+\/monitor/, title: 'Experiment monitor', description: 'Live progress and logs for the running experiment.' },
  { match: /^\/experiments/, title: 'Experiments', description: 'All runs and variants for this workspace.' },
  { match: /^\/runs\/[^/]+\/inspect/, title: 'Run inspector', description: 'Turn-grouped transcript with per-turn diffs.' },
  { match: /^\/tasks\/new/, title: 'New task', description: 'Add a custom task to the evaluation library.' },
  { match: /^\/tasks\//, title: 'Task detail', description: 'Prompt, tests, and workspace mode.' },
  { match: /^\/tasks/, title: 'Task library', description: 'Reusable evaluation prompts and test suites.' },
  { match: /^\/artifacts/, title: 'Artifacts', description: 'Context files attached to each variant.' },
  { match: /^\/settings/, title: 'Settings', description: 'API keys, models, agents, and defaults.' },
];

export function Header() {
  const location = useLocation();
  const match = titles.find((item) => item.match.test(location.pathname));
  return (
    <header className="flex items-center justify-between border-b border-border bg-bg-elev-1/70 px-6 py-4 backdrop-blur">
      <div>
        <div className="text-lg font-semibold tracking-tight text-fg">{match?.title ?? 'Frameval'}</div>
        <div className="text-xs text-fg-muted">{match?.description ?? 'Local-first evaluation for context engineering.'}</div>
      </div>
      <ThemeToggle />
    </header>
  );
}
