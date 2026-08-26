export function BooleanToggle({ value, onChange }: { value: boolean; onChange: (value: boolean) => void }) {
  return (
    <button
      type="button"
      onClick={(event) => {
        event.preventDefault();
        onChange(!value);
      }}
      className={[
        'mt-1 flex h-[42px] w-full items-center rounded-2xl p-1 text-xs font-bold normal-case tracking-normal ring-1 transition',
        value
          ? 'bg-[var(--color-blue-subtle)] text-[var(--color-blue)] ring-[var(--color-blue-border)]'
          : 'bg-[var(--color-surface-subtle)] text-[var(--color-muted-soft)] ring-[var(--color-border-soft)]',
      ].join(' ')}
      aria-pressed={value}
    >
      <span
        className={[
          'grid h-8 flex-1 place-items-center rounded-xl transition',
          value ? 'bg-[var(--color-surface)] shadow-sm' : '',
        ].join(' ')}
      >
        On
      </span>
      <span
        className={[
          'grid h-8 flex-1 place-items-center rounded-xl transition',
          !value ? 'bg-[var(--color-surface)] text-[var(--color-muted)] shadow-sm' : '',
        ].join(' ')}
      >
        Off
      </span>
    </button>
  );
}
