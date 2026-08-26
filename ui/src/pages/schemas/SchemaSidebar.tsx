import { PlusIcon } from '../../components/Icons'
import { IconButton } from '../../components/Button'
import type { DeveloperSchema } from '../../types/journey'

export function SchemaSidebar({
  schemas,
  selectedID,
  search,
  loading,
  onSearch,
  onCreate,
  onSelect,
}: {
  schemas: DeveloperSchema[]
  selectedID: string
  search: string
  loading: boolean
  onSearch: (value: string) => void
  onCreate: () => void
  onSelect: (schema: DeveloperSchema) => void
}) {
  return (
    <aside className="flex w-[min(420px,34vw)] shrink-0 flex-col overflow-hidden rounded-3xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-sm">
      <div className="border-b border-[var(--color-border-soft)] p-5">
        <div className="flex items-start justify-between gap-3">
          <div>
            <p className="text-xs font-bold uppercase tracking-[0.32em] text-[var(--color-blue)]">Schemas</p>
            <h1 className="mt-2 text-2xl font-bold tracking-tight text-[var(--color-ink)]">Validation schemas</h1>
            <p className="mt-1 text-sm text-[var(--color-muted)]">Create reusable JSON Schemas for requests and context data.</p>
          </div>
          <IconButton onClick={onCreate} label="Create schema" variant="primary" size="lg">
            <PlusIcon />
          </IconButton>
        </div>
        <input
          value={search}
          onChange={(event) => onSearch(event.target.value)}
          placeholder="Search by name, description or UUID..."
          className="mt-4 w-full rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-4 py-3 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]"
        />
      </div>
      <div className="min-h-0 flex-1 overflow-auto p-3">
        {loading && <p className="rounded-2xl bg-[var(--color-surface-subtle)] px-4 py-3 text-sm text-[var(--color-muted)]">Loading schemas…</p>}
        {!loading && schemas.length === 0 && <p className="rounded-2xl bg-[var(--color-surface-subtle)] px-4 py-3 text-sm text-[var(--color-muted)]">No schemas found.</p>}
        <div className="grid gap-2">
          {schemas.map((schema) => {
            const selected = selectedID === schema.id
            return (
              <button
                key={schema.id || schema.name}
                type="button"
                onClick={() => onSelect(schema)}
                className={[
                  'rounded-3xl border p-4 text-left transition',
                  selected
                    ? 'border-[var(--color-blue)] bg-[var(--color-blue-subtle)] shadow-sm'
                    : 'border-[var(--color-border-soft)] bg-[var(--color-surface-muted-transparent)] hover:border-[var(--color-blue-border)] hover:bg-[var(--color-surface-subtle)]',
                ].join(' ')}
              >
                <h2 className="truncate text-sm font-bold text-[var(--color-ink)]">{schema.name || 'Unnamed schema'}</h2>
                {schema.description && <p className="mt-1 line-clamp-2 text-xs text-[var(--color-muted)]">{schema.description}</p>}
                {schema.id && <p className="mt-3 truncate font-mono text-[11px] text-[var(--color-muted-soft)]">{schema.id}</p>}
              </button>
            )
          })}
        </div>
      </div>
    </aside>
  )
}
