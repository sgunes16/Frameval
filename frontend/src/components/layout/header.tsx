import { Link, useLocation } from 'react-router-dom';
import { Button } from '../ui/button';

const titles: Array<{ match: RegExp; title: string; description: string }> = [
  { match: /^\/$/, title: 'Dashboard', description: 'Snapshot of your experiments and evaluation artifacts.' },
  { match: /^\/experiments\/new/, title: 'New experiment', description: 'Configure workspace, context, and run parameters.' },
  { match: /^\/experiments\/[^/]+\/monitor/, title: 'Experiment monitor', description: 'Live progress and logs for the running experiment.' },
  { match: /^\/experiments\/[^/]+\/results/, title: 'Experiment results', description: 'Deterministic metrics and pairwise comparisons.' },
  { match: /^\/experiments/, title: 'Experiments', description: 'All runs and variants for this workspace.' },
  { match: /^\/tasks\/new/, title: 'New task', description: 'Add a custom task to the evaluation library.' },
  { match: /^\/tasks\//, title: 'Task detail', description: 'Prompt, tests, and workspace mode.' },
  { match: /^\/tasks/, title: 'Task library', description: 'Reusable evaluation prompts and test suites.' },
  { match: /^\/artifacts/, title: 'Artifacts', description: 'Context files attached to each variant.' },
  { match: /^\/baselines/, title: 'Baselines', description: 'Reference runs used for comparison.' },
  { match: /^\/settings/, title: 'Settings', description: 'API keys, models, agents, and defaults.' },
];

export function Header() {
  const location = useLocation();
  const match = titles.find((item) => item.match.test(location.pathname));
  const isExperimentsList = location.pathname === '/experiments';
  return (
    <header className="flex items-center justify-between border-b border-slate-200/70 bg-white/70 px-6 py-4 backdrop-blur">
      <div>
        <div className="text-lg font-semibold tracking-tight text-slate-900">{match?.title ?? 'Frameval'}</div>
        <div className="text-xs text-slate-500">{match?.description ?? 'Local-first evaluation for context engineering.'}</div>
      </div>
      <div className="flex items-center gap-2">
        {isExperimentsList && (
          <Link to="/experiments/new">
            <Button size="sm">New experiment</Button>
          </Link>
        )}
      </div>
    </header>
  );
}
