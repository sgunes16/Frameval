import { Badge } from '../ui/badge';
import { Input } from '../ui/input';

const OPTIONS = [
  {
    id: 'task_codebase',
    label: 'Builtin task workspace',
    description: 'Use the codebase that ships with the selected task.',
  },
  {
    id: 'local_path',
    label: 'Local folder',
    description: 'Point Frameval at a repo already on this machine.',
  },
  {
    id: 'git_url',
    label: 'Git URL',
    description: 'Clone a public repository and optionally check out a ref.',
  },
  {
    id: 'empty',
    label: 'Empty workspace',
    description: 'Start from a clean directory, ideal for greenfield prompts.',
  },
];

const GREENFIELD_STARTERS: Array<{
  name: string;
  description: string;
  url: string;
  ref?: string;
  tags: string[];
}> = [
  {
    name: 'Vite + React + TypeScript',
    description: 'Minimal SPA starter with Vite, React, and TypeScript.',
    url: 'https://github.com/vitejs/vite-ts-template.git',
    tags: ['frontend', 'typescript'],
  },
  {
    name: 'Next.js app router',
    description: 'Official Next.js starter with the app router and Tailwind.',
    url: 'https://github.com/vercel/next.js.git',
    ref: 'canary',
    tags: ['frontend', 'fullstack'],
  },
  {
    name: 'Express TypeScript API',
    description: 'Express server scaffold with TypeScript and ts-node-dev.',
    url: 'https://github.com/microsoft/TypeScript-Node-Starter.git',
    tags: ['backend', 'typescript'],
  },
  {
    name: 'FastAPI starter',
    description: 'FastAPI backend with pytest and uvicorn wired up.',
    url: 'https://github.com/tiangolo/full-stack-fastapi-postgresql.git',
    tags: ['backend', 'python'],
  },
  {
    name: 'Go module template',
    description: 'Opinionated Go module layout with Makefile and golangci-lint.',
    url: 'https://github.com/golang-standards/project-layout.git',
    tags: ['backend', 'go'],
  },
  {
    name: 'Rust CLI starter',
    description: 'Cargo workspace with clap + anyhow scaffolding.',
    url: 'https://github.com/rust-cli/cli-template.git',
    tags: ['cli', 'rust'],
  },
];

export function StepWorkspaceSource({
  workspaceSourceType,
  localPath,
  gitURL,
  gitRef,
  onChange,
}: {
  workspaceSourceType: string;
  localPath: string;
  gitURL: string;
  gitRef: string;
  onChange: (next: { workspaceSourceType: string; localPath: string; gitURL: string; gitRef: string }) => void;
}) {
  function updateWorkspace(patch: Partial<{ workspaceSourceType: string; localPath: string; gitURL: string; gitRef: string }>) {
    onChange({ workspaceSourceType, localPath, gitURL, gitRef, ...patch });
  }

  return (
    <div className="space-y-4">
      <div className="grid gap-3 sm:grid-cols-2">
        {OPTIONS.map((option) => {
          const selected = option.id === workspaceSourceType;
          return (
            <button
              key={option.id}
              type="button"
              onClick={() => updateWorkspace({ workspaceSourceType: option.id })}
              className={
                selected
                  ? 'rounded-lg border border-slate-900 bg-slate-900/5 p-3 text-left'
                  : 'rounded-lg border border-slate-200 bg-white p-3 text-left transition hover:border-slate-300'
              }
            >
              <div className="text-sm font-medium text-slate-900">{option.label}</div>
              <div className="mt-1 text-xs text-slate-500">{option.description}</div>
            </button>
          );
        })}
      </div>

      {workspaceSourceType === 'local_path' && (
        <div className="space-y-2">
          <Input
            value={localPath}
            onChange={(event) => updateWorkspace({ localPath: event.target.value })}
            placeholder="/Users/me/projects/my-repo"
          />
          <div className="text-[11px] text-slate-500">
            When running in Docker, make sure the engine container can see this path (bind-mount or host filesystem).
          </div>
        </div>
      )}

      {workspaceSourceType === 'git_url' && (
        <div className="space-y-3">
          <div className="grid gap-3 sm:grid-cols-2">
            <Input
              value={gitURL}
              onChange={(event) => updateWorkspace({ gitURL: event.target.value })}
              placeholder="https://github.com/org/repo.git"
            />
            <Input
              value={gitRef}
              onChange={(event) => updateWorkspace({ gitRef: event.target.value })}
              placeholder="branch, tag, or commit SHA (optional)"
            />
          </div>
          <div className="rounded-lg border border-slate-200 bg-slate-50/60 p-3">
            <div className="mb-2 flex items-center justify-between">
              <div className="text-xs font-medium text-slate-700">Greenfield starter templates</div>
              <Badge tone="muted">Click to autofill</Badge>
            </div>
            <div className="grid gap-2 sm:grid-cols-2">
              {GREENFIELD_STARTERS.map((starter) => (
                <button
                  key={starter.url}
                  type="button"
                  onClick={() => updateWorkspace({ gitURL: starter.url, gitRef: starter.ref ?? '' })}
                  className="rounded-lg border border-slate-200 bg-white p-3 text-left text-xs transition hover:border-slate-300"
                >
                  <div className="text-sm font-medium text-slate-900">{starter.name}</div>
                  <div className="mt-1 text-[11px] text-slate-500">{starter.description}</div>
                  <div className="mt-2 flex flex-wrap gap-1">
                    {starter.tags.map((tag) => (
                      <Badge key={tag} tone="neutral">
                        {tag}
                      </Badge>
                    ))}
                  </div>
                </button>
              ))}
            </div>
          </div>
        </div>
      )}

      {workspaceSourceType === 'empty' && (
        <div className="rounded-lg border border-dashed border-slate-200 bg-slate-50/60 p-3 text-xs text-slate-500">
          The sandbox will start with an empty directory. Pair with a greenfield task to evaluate how the agent
          scaffolds the project from scratch.
        </div>
      )}
    </div>
  );
}
