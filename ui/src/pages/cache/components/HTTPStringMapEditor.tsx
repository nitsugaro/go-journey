import { IconButton } from '../../../components/Button'
import { DeleteIcon, PlusIcon } from '../../../components/Icons'
import { useKeyValueRows } from '../../../components/useKeyValueRows'

type HTTPStringMapEditorProps = {
  value?: Record<string, string>
  onChange: (value: Record<string, string>) => void
  keyPlaceholder?: string
  valuePlaceholder?: string
}

export function HTTPStringMapEditor({
  value = {},
  onChange,
  keyPlaceholder = 'Header',
  valuePlaceholder = 'Value',
}: HTTPStringMapEditorProps) {
  const { rows, add, update, remove } = useKeyValueRows(value, onChange)

  return (
    <div className="rounded-3xl border border-[var(--color-border-soft)] bg-[var(--color-surface-subtle)] p-4">
      <div className="mb-3 flex items-center justify-between gap-3">
        <p className="text-xs font-bold uppercase tracking-[0.22em] text-[var(--color-muted-soft)]">Fields</p>
        <IconButton label="Add field" size="sm" variant="secondary" onClick={() => add(nextAvailableKey(rows.map((row) => row.key)), '')}>
          <PlusIcon size={15} />
        </IconButton>
      </div>
      <div className="grid gap-2">
        {rows.length === 0 && <p className="rounded-2xl bg-[var(--color-surface)] px-4 py-3 text-sm text-[var(--color-muted)]">No fields configured.</p>}
        {rows.map((row) => (
          <div key={row.id} className="grid gap-2 rounded-2xl bg-[var(--color-surface)] p-2 sm:grid-cols-[1fr_1fr_auto]">
            <input
              value={row.key}
              onChange={(event) => update(row.id, (currentRow) => ({ ...currentRow, key: event.target.value }))}
              placeholder={keyPlaceholder}
              className="rounded-xl bg-[var(--color-surface-subtle)] px-3 py-2 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:ring-2 focus:ring-[var(--color-blue)]"
            />
            <input
              value={row.value}
              onChange={(event) => update(row.id, (currentRow) => ({ ...currentRow, value: event.target.value }))}
              placeholder={valuePlaceholder}
              className="rounded-xl bg-[var(--color-surface-subtle)] px-3 py-2 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:ring-2 focus:ring-[var(--color-blue)]"
            />
            <IconButton label="Remove field" size="sm" variant="danger" onClick={() => remove(row.id)}>
              <DeleteIcon size={15} />
            </IconButton>
          </div>
        ))}
      </div>
    </div>
  )
}

function nextAvailableKey(keys: string[]) {
  const used = new Set(keys.map((key) => key.trim()))
  let index = keys.length + 1
  while (used.has(`key_${index}`)) index += 1
  return `key_${index}`
}
