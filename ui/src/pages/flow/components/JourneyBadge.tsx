export function JourneyBadge({ label, tone }: { label: string; tone: 'green' | 'blue' | 'amber' | 'slate' }) {
  const classes = {
    green: 'bg-[var(--color-green-soft)] text-[var(--color-green)] ring-[var(--color-green-border)]',
    blue: 'bg-[var(--color-blue-subtle)] text-[var(--color-blue)] ring-[var(--color-blue-border)]',
    amber: 'bg-[var(--color-warning-soft)] text-[var(--color-warning)] ring-[var(--color-warning-border)]',
    slate: 'bg-[var(--color-surface-soft)] text-[var(--color-muted)] ring-[var(--color-border-subtle)]',
  }[tone];
  return (
    <span className={`rounded-lg px-3 py-1 text-xs font-semibold uppercase tracking-[0.08em] ring-1 ${classes}`}>
      {label}
    </span>
  );
}

