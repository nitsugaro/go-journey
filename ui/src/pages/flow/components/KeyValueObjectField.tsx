import { Button, IconButton } from '../../../components/Button';
import { useKeyValueRows } from '../../../components/useKeyValueRows';
import { asRecord, formatEditableValue, parseEditableValue } from '../utils/schemaUtils';

export function KeyValueObjectField({
  label,
  required,
  description,
  value,
  onChange,
}: {
  label: string;
  required?: boolean;
  description?: string;
  value: unknown;
  onChange: (value: unknown) => void;
}) {
  const current = asRecord(value);
  const { rows, add, update, remove } = useKeyValueRows(current, onChange);

  return (
    <div className="rounded-3xl bg-[var(--color-surface-subtle)] p-4 ring-1 ring-[var(--color-border-soft)] lg:col-span-2">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.14em] text-[var(--color-muted-soft)]">
            {label}
            {required ? ' *' : ''}
          </p>
          {description && <p className="mt-1 text-xs text-[var(--color-muted-soft)]">{description}</p>}
        </div>
        <Button
          onClick={() => add(nextAvailableKey(rows.map((row) => row.key)), '')}
          variant="secondary"
          size="sm"
        >
          + field
        </Button>
      </div>
      <div className="mt-3 grid gap-2">
        {rows.length === 0 && (
          <p className="rounded-2xl bg-[var(--color-surface)] px-3 py-2 text-sm text-[var(--color-muted-soft)]">
            No fields configured.
          </p>
        )}
        {rows.map((row) => (
          <div
            key={row.id}
            className="grid gap-2 rounded-2xl bg-[var(--color-surface)] p-2 ring-1 ring-[var(--color-border-soft)] sm:grid-cols-[0.7fr_1fr_auto]"
          >
            <input
              value={row.key}
              onChange={(event) => update(row.id, (currentRow) => ({ ...currentRow, key: event.target.value }))}
              className="rounded-xl bg-[var(--color-surface-subtle)] px-3 py-2 text-sm font-semibold text-[var(--color-ink)] outline-none focus:ring-2 focus:ring-[var(--color-blue)]"
              placeholder="key"
            />
            <input
              value={formatEditableValue(row.value)}
              onChange={(event) => update(row.id, (currentRow) => ({ ...currentRow, value: parseEditableValue(event.target.value) }))}
              className="rounded-xl bg-[var(--color-surface-subtle)] px-3 py-2 text-sm text-[var(--color-ink)] outline-none focus:ring-2 focus:ring-[var(--color-blue)]"
              placeholder="value"
            />
            <IconButton
              onClick={() => remove(row.id)}
              label="Remove field"
              variant="ghost"
              size="sm"
            >
              ×
            </IconButton>
          </div>
        ))}
      </div>
    </div>
  );
}

function nextAvailableKey(keys: string[]) {
  const used = new Set(keys.map((key) => key.trim()));
  let index = keys.length + 1;
  while (used.has(`key_${index}`)) index += 1;
  return `key_${index}`;
}
