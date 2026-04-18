export function cn(...classes: Array<string | false | null | undefined>) {
  return classes.filter(Boolean).join(' ');
}

export function formatCurrency(value?: number | null) {
  if (value === undefined || value === null) return '-';
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(value);
}

export function formatTimeAgo(value?: string | null) {
  if (!value) return '-';
  const timestamp = Date.parse(value);
  if (Number.isNaN(timestamp)) return value;
  const diffSeconds = Math.max(1, Math.floor((Date.now() - timestamp) / 1000));
  if (diffSeconds < 60) return `${diffSeconds}s ago`;
  const minutes = Math.floor(diffSeconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  const months = Math.floor(days / 30);
  if (months < 12) return `${months}mo ago`;
  const years = Math.floor(months / 12);
  return `${years}y ago`;
}

export type BadgeTone = 'neutral' | 'success' | 'warning' | 'danger' | 'info' | 'muted';

export function statusTone(status?: string): BadgeTone {
  switch (status) {
    case 'completed':
      return 'success';
    case 'running':
    case 'queued':
    case 'grading':
      return 'info';
    case 'failed':
    case 'cancelled':
      return 'danger';
    case 'draft':
    case 'pending':
      return 'muted';
    default:
      return 'neutral';
  }
}

export function statusLabel(status?: string): string {
  if (!status) return '-';
  return status.charAt(0).toUpperCase() + status.slice(1);
}
