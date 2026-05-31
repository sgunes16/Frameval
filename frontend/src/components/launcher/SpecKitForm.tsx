import { useEffect } from 'react';
import { useSpecKitCatalog } from '../../lib/hooks';
import type { SpecKitExtensionPublic } from '../../lib/types';

interface FormValue {
  extension_ids?: string[];
}

interface FormProps {
  value: FormValue | undefined;
  onChange: (next: { extension_ids: string[] }) => void;
}

/**
 * SpecKitForm — multi-select chip list of curated spec-kit extensions.
 *
 * The launcher tracks `extension_ids` (multi-select). Matrix expansion
 * inside launch.tsx narrows that list to one id per launch cell before
 * posting (so the per-variant wire shape stays `{ extension_id: <one> }`).
 *
 * Seeds `{ extension_ids: ['canonical'] }` on first render with an
 * undefined value so the gate doesn't trip the moment the user picks
 * the speckit harness chip.
 */
export function SpecKitForm({ value, onChange }: FormProps) {
  const query = useSpecKitCatalog();
  const selected = value?.extension_ids ?? [];

  useEffect(() => {
    if (value === undefined) {
      onChange({ extension_ids: ['canonical'] });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const toggle = (id: string) => {
    const next = selected.includes(id)
      ? selected.filter((x) => x !== id)
      : [...selected, id];
    onChange({ extension_ids: next });
  };

  if (query.isError) {
    return (
      <div className="mt-2 rounded-md border border-border bg-bg-elev-1 p-3 text-xs text-fg-muted">
        Could not load spec-kit catalog. Check the engine logs.
      </div>
    );
  }
  const catalog = query.data ?? [];
  if (catalog.length === 0) {
    return (
      <div className="mt-2 rounded-md border border-border bg-bg-elev-1 p-3 text-xs text-fg-muted">
        Could not load spec-kit catalog.
      </div>
    );
  }

  return (
    <div className="mt-2 rounded-md border border-border bg-bg-elev-1 p-3">
      <div className="mb-2 text-xs uppercase tracking-wider text-fg-muted">
        Spec-kit extensions
      </div>
      <p className="mb-3 text-xs text-fg-muted">
        Each selected extension becomes its own variant. N selections × tasks × executors × models = N×… experiments.
      </p>
      <div className="flex flex-wrap gap-2">
        {catalog.map((ext) => renderChip(ext, selected.includes(ext.id), () => toggle(ext.id)))}
      </div>
    </div>
  );
}

function renderChip(ext: SpecKitExtensionPublic, isSelected: boolean, onClick: () => void) {
  const stateClasses = isSelected
    ? 'border-fg bg-bg-elev-2 text-fg'
    : 'border-border bg-bg-elev-1 text-fg-muted hover:border-border-strong';
  return (
    <button
      key={ext.id}
      type="button"
      onClick={onClick}
      title={ext.description}
      className={`flex items-center gap-2 rounded-md border px-2.5 py-1.5 text-xs transition ${stateClasses}`}
    >
      <span className="font-medium">{ext.name}</span>
      {ext.multi_agent && (
        <span className="rounded-full border border-border bg-bg-elev-2 px-1.5 py-0.5 text-xs uppercase tracking-wider text-fg-muted">
          Multi-agent
        </span>
      )}
    </button>
  );
}
