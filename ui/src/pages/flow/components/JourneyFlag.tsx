export function JourneyFlag({
  label,
  checked,
  onChange,
}: {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <label
      className={[
        'flex cursor-pointer items-center justify-between rounded-xl px-3 py-2 text-sm font-semibold transition',
        checked
          ? 'bg-[var(--color-blue-soft)] text-[var(--color-blue)]'
          : 'bg-[var(--color-surface)] text-[var(--color-muted)] ring-1 ring-[var(--color-border-soft)] hover:bg-[var(--color-surface-subtle)]',
      ].join(' ')}
    >
      <span>{label}</span>
      <input
        type="checkbox"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
        className="h-4 w-4 accent-[var(--color-blue)]"
      />
    </label>
  );
}

