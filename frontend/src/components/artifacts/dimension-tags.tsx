import { Badge } from '../ui/badge';

export function DimensionTags({ dimensions }: { dimensions?: Record<string, unknown> }) {
  if (!dimensions) return null;
  return <div className="flex flex-wrap gap-2">{Object.entries(dimensions).map(([key, value]) => <Badge key={key}>{key}: {String(value)}</Badge>)}</div>;
}
