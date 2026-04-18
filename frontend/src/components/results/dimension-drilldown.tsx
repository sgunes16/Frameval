import type { ArtifactVersion } from '../../lib/types';
import { Badge } from '../ui/badge';

export function DimensionDrilldown({ artifact }: { artifact?: ArtifactVersion }) {
  if (!artifact?.dimensions) return null;
  return <div className="flex flex-wrap gap-2">{Object.entries(artifact.dimensions).map(([key, value]) => <Badge key={key}>{key}: {String(value)}</Badge>)}</div>;
}
