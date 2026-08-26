type TagsInputProps = {
  value: unknown[]
  onChange: (value: string[]) => void
  placeholder?: string
  className?: string
}

export function TagsInput({
  value,
  onChange,
  placeholder = 'Type value and press Enter…',
  className = '',
}: TagsInputProps) {
  const tags = value.map((item) => String(item)).filter(Boolean)
  let draft = ''

  const addTag = (nextValue: string, reset: () => void) => {
    const next = nextValue.trim()
    if (!next) return
    if (!tags.includes(next)) onChange([...tags, next])
    reset()
  }

  const removeTag = (tag: string) => onChange(tags.filter((item) => item !== tag))

  return (
    <div className={['flex flex-wrap gap-2 rounded-2xl bg-[var(--color-surface)] p-3 ring-1 ring-[var(--color-border-soft)]', className].filter(Boolean).join(' ')}>
      {tags.map((tag) => (
        <span
          key={tag}
          className="inline-flex max-w-full items-center gap-2 rounded-full bg-[var(--color-blue-subtle)] px-3 py-1.5 text-sm font-semibold text-[var(--color-blue)] ring-1 ring-[var(--color-blue-border)]"
        >
          <span className="truncate">{tag}</span>
          <button
            type="button"
            onClick={() => removeTag(tag)}
            className="grid h-5 w-5 shrink-0 place-items-center rounded-full text-[var(--color-muted)] transition hover:bg-[var(--color-surface)] hover:text-[var(--color-red)]"
            aria-label={`Remove ${tag}`}
          >
            ×
          </button>
        </span>
      ))}
      <input
        defaultValue=""
        onChange={(event) => {
          draft = event.target.value
        }}
        onKeyDown={(event) => {
          if (event.key === 'Enter' || event.key === ',') {
            event.preventDefault()
            const input = event.currentTarget
            addTag(input.value || draft, () => {
              draft = ''
              input.value = ''
            })
          }
          if (event.key === 'Backspace' && !event.currentTarget.value && tags.length > 0) {
            removeTag(tags[tags.length - 1])
          }
        }}
        onBlur={(event) => {
          const input = event.currentTarget
          addTag(input.value || draft, () => {
            draft = ''
            input.value = ''
          })
        }}
        placeholder={tags.length ? 'Add another value…' : placeholder}
        className="min-w-44 flex-1 bg-transparent px-2 py-1.5 text-sm font-semibold text-[var(--color-ink)] outline-none placeholder:text-[var(--color-muted-soft)]"
      />
    </div>
  )
}
