import { useMemo, useState } from 'react'
import { Search } from 'lucide-react'
import type { TypeOption } from '../config/typeOptions'

type TypeOptionPickerProps = {
  label: string
  value: string
  options: TypeOption[]
  onChange?: (value: string) => void
  readOnly?: boolean
  columns?: 'auto' | 'two' | 'three'
  searchable?: boolean
  pageSize?: number
}

const columnsClass = {
  auto: 'sm:grid-cols-2 xl:grid-cols-3',
  two: 'sm:grid-cols-2',
  three: 'sm:grid-cols-2 xl:grid-cols-3',
}

export function TypeOptionPicker({
  label,
  value,
  options,
  onChange,
  readOnly = false,
  columns = 'auto',
  searchable = false,
  pageSize = 6,
}: TypeOptionPickerProps) {
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(0)
  const filteredOptions = useMemo(() => {
    if (readOnly) return options.filter((option) => option.value === value)
    const clean = query.trim().toLowerCase()
    if (!clean) return options
    return options.filter((option) => `${option.title} ${option.value} ${option.description} ${option.group || ''}`.toLowerCase().includes(clean))
  }, [options, query, readOnly, value])
  const needsPaging = !readOnly && filteredOptions.length > pageSize
  const pageCount = Math.max(1, Math.ceil(filteredOptions.length / pageSize))
  const safePage = Math.min(page, pageCount - 1)
  const visibleOptions = needsPaging ? filteredOptions.slice(safePage * pageSize, safePage * pageSize + pageSize) : filteredOptions
  const fallback = options[0]

  return (
    <section className="grid gap-2">
      <div className="flex items-center justify-between gap-3">
        <p className="text-xs font-bold uppercase tracking-[0.24em] text-[var(--color-muted-soft)]">{label}</p>
        {needsPaging && <p className="text-xs font-semibold text-[var(--color-muted-soft)]">{safePage + 1} / {pageCount}</p>}
      </div>
      {!readOnly && searchable && options.length > pageSize && (
        <label className="flex items-center gap-2 rounded-2xl border border-[var(--color-border-soft)] bg-[var(--color-surface-subtle)] px-3 py-2 text-[var(--color-muted-soft)]">
          <Search size={15} />
          <input
            value={query}
            onChange={(event) => {
              setQuery(event.target.value)
              setPage(0)
            }}
            placeholder="Search type..."
            className="min-w-0 flex-1 bg-transparent text-sm font-semibold text-[var(--color-ink)] outline-none placeholder:text-[var(--color-muted-soft)]"
          />
        </label>
      )}
      <div className={['grid gap-3', columnsClass[columns]].join(' ')} role={readOnly ? undefined : 'radiogroup'}>
        {(visibleOptions.length ? visibleOptions : fallback ? [fallback] : []).map((option) => (
          <TypeOptionCard
            key={option.value}
            option={option}
            selected={option.value === value}
            readOnly={readOnly}
            onSelect={() => onChange?.(option.value)}
          />
        ))}
      </div>
      {needsPaging && (
        <div className="flex justify-end gap-2">
          <button type="button" disabled={safePage === 0} onClick={() => setPage((current) => Math.max(0, current - 1))} className="rounded-xl bg-[var(--color-surface-soft)] px-3 py-2 text-xs font-bold text-[var(--color-muted)] disabled:opacity-40">Prev</button>
          <button type="button" disabled={safePage >= pageCount - 1} onClick={() => setPage((current) => Math.min(pageCount - 1, current + 1))} className="rounded-xl bg-[var(--color-surface-soft)] px-3 py-2 text-xs font-bold text-[var(--color-muted)] disabled:opacity-40">Next</button>
        </div>
      )}
    </section>
  )
}

function TypeOptionCard({
  option,
  selected,
  readOnly,
  onSelect,
}: {
  option: TypeOption
  selected: boolean
  readOnly: boolean
  onSelect: () => void
}) {
  const Icon = option.icon
  const Component = readOnly ? 'div' : 'button'
  const cardTone = readOnly
    ? 'border-[var(--color-border-soft)] bg-[var(--color-surface-subtle)] shadow-none'
    : selected
      ? 'border-[var(--color-blue)] bg-[var(--color-blue-subtle)] shadow-sm shadow-[var(--color-muted-faint)]'
      : 'border-[var(--color-border-soft)] bg-[var(--color-surface-muted-transparent)] hover:border-[var(--color-blue-border)] hover:bg-[var(--color-surface-subtle)]'
  const iconTone = readOnly
    ? 'border-[var(--color-border-soft)] bg-[var(--color-surface-soft)] text-[var(--color-blue)]'
    : selected
      ? 'border-[var(--color-blue-border)] bg-[var(--color-blue)] text-[var(--color-white)]'
      : 'border-[var(--color-border-soft)] bg-[var(--color-surface-soft)] text-[var(--color-blue)] group-hover:border-[var(--color-blue-border)]'

  return (
    <Component
      type={readOnly ? undefined : 'button'}
      role={readOnly ? undefined : 'radio'}
      aria-checked={readOnly ? undefined : selected}
      title={readOnly ? 'Type cannot be changed after creation' : option.title}
      onClick={readOnly ? undefined : onSelect}
      className={[
        'group relative rounded-3xl border text-left transition',
        readOnly ? 'min-h-[76px] p-3' : 'min-h-[118px] p-4',
        cardTone,
        readOnly ? 'cursor-default' : 'cursor-pointer',
      ].join(' ')}
    >
      <div className="flex items-start gap-3">
        <span
          className={[
            'grid h-10 w-10 shrink-0 place-items-center rounded-2xl border transition',
            iconTone,
          ].join(' ')}
        >
          <Icon size={20} strokeWidth={2.2} aria-hidden="true" />
        </span>
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="text-sm font-bold text-[var(--color-ink)]">{option.title}</h3>
            {option.group && (
              <span className="rounded-full bg-[var(--color-surface-soft)] px-2 py-0.5 text-[10px] font-bold uppercase tracking-[0.18em] text-[var(--color-muted-soft)]">
                {option.group}
              </span>
            )}
          </div>
          {!readOnly && <p className="mt-1.5 text-xs leading-5 text-[var(--color-muted)]">{option.description}</p>}
        </div>
      </div>
      {!readOnly && <p className="mt-3 truncate font-mono text-[11px] font-semibold text-[var(--color-blue)]">{option.value}</p>}
    </Component>
  )
}
