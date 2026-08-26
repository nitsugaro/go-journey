import { useEffect, useState, type ReactNode } from 'react'
import { Button } from './Button'

export type PlaceholderOr<T> = T | string

export function PlaceholderValueField<T>({
  label,
  value,
  emptyValue,
  onChange,
  children,
  required,
  description,
}: {
  label: string
  value: PlaceholderOr<T> | undefined
  emptyValue: T
  onChange: (value: PlaceholderOr<T>) => void
  children: (value: T) => ReactNode
  required?: boolean
  description?: string
}) {
  const [placeholderMode, setPlaceholderMode] = useState(() => isFullPlaceholder(stringValue(value)))

  useEffect(() => {
    if (isFullPlaceholder(stringValue(value))) setPlaceholderMode(true)
  }, [value])

  const toggle = () => {
    const nextMode = !placeholderMode
    setPlaceholderMode(nextMode)
    onChange(nextMode ? '' : emptyValue)
  }

  return (
    <div className="grid gap-2">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-xs font-bold uppercase tracking-[0.22em] text-[var(--color-muted-soft)]">
            {label}
            {required ? ' *' : ''}
          </p>
          {description && <p className="mt-1 text-xs text-[var(--color-muted)]">{description}</p>}
        </div>
        <Button type="button" size="xs" variant={placeholderMode ? 'warning' : 'secondary'} onClick={toggle}>
          {'${}'}
        </Button>
      </div>
      {placeholderMode ? (
        <input
          value={stringValue(value)}
          onChange={(event) => onChange(event.target.value)}
          placeholder="${env.value}"
          className="rounded-2xl bg-[var(--color-surface)] px-4 py-3 font-mono text-sm font-semibold text-[var(--color-warning)] outline-none ring-1 ring-[var(--color-border-soft)] focus:ring-2 focus:ring-[var(--color-blue)]"
        />
      ) : children(value === undefined || typeof value === 'string' ? emptyValue : value as T)}
    </div>
  )
}

function isFullPlaceholder(value: string) {
  return /^\$\{[^{}.]+\.[^{}]+\}$/.test(value.trim())
}

function stringValue(value: unknown) {
  return typeof value === 'string' ? value : ''
}
