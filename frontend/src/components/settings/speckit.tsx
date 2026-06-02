import { useEffect, useState } from 'react';
import { useSpecKitSettings, useSaveSpecKitSettings } from '../../lib/hooks';
import { Button } from '../ui/button';
import { Card, CardHeader } from '../ui/card';
import { Input } from '../ui/input';

export function SpecKitPanel() {
  const { data: settings, isLoading } = useSpecKitSettings();
  const save = useSaveSpecKitSettings();
  const [version, setVersion] = useState('');

  // Initialize local state once settings load.
  useEffect(() => {
    if (settings) {
      setVersion(settings.version);
    }
  }, [settings]);

  if (isLoading || !settings) {
    return (
      <Card>
        <CardHeader title="spec-kit version" description="Loading…" />
      </Card>
    );
  }

  const dirty = version !== settings.version;

  return (
    <Card>
      <CardHeader
        title="spec-kit version"
        description="spec-kit CLI release tag (e.g. v0.9.1). Installed in the sandbox at run time."
      />
      <div className="space-y-3">
        <div className="grid grid-cols-[120px_1fr] items-center gap-2">
          <label className="text-sm text-fg-muted">Version</label>
          <Input
            value={version}
            onChange={(e) => setVersion(e.target.value)}
            placeholder="v0.9.1"
          />
        </div>

        <div className="flex items-center justify-end gap-2 border-t border-border pt-3 text-xs">
          <Button onClick={() => save.mutate({ version })} disabled={!dirty || save.isPending}>
            {save.isPending ? 'Saving…' : 'Save'}
          </Button>
        </div>
      </div>
    </Card>
  );
}
