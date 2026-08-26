import { useEffect, useMemo, useState } from 'react'
import { FileCode2, Trash2 } from 'lucide-react'
import { IconButton } from '../../components/Button'
import { ChangesIcon, SaveIcon, SavedIcon, SpinnerIcon } from '../../components/Icons'
import type { DeveloperSchema } from '../../types/journey'
import { SchemaBuilder } from './SchemaBuilder'
import { SchemaJsonEditor } from './SchemaJsonEditor'
import type { SchemaBuilderDraft, SchemaEditorMode } from './schemaTypes'
import { builderToSchema, parseSchemaJSON, schemaToBuilder } from './schemaUtils'

export function SchemaEditorPanel({
  draft,
  dirty,
  saving,
  error,
  selectedID,
  onDraftChange,
  onSave,
  onDelete,
}: {
  draft: DeveloperSchema
  dirty: boolean
  saving: boolean
  error?: string
  selectedID?: string
  onDraftChange: (next: Partial<DeveloperSchema>) => void
  onSave: () => void
  onDelete?: () => void
}) {
  const [mode, setMode] = useState<SchemaEditorMode>('builder')
  const [builder, setBuilder] = useState<SchemaBuilderDraft>(() => schemaToBuilder(draft.schema || {}))
  const [schemaText, setSchemaText] = useState(() => pretty(draft.schema || {}))

  useEffect(() => {
    const nextBuilder = schemaToBuilder(draft.schema || {})
    setBuilder(nextBuilder)
    setSchemaText(pretty(draft.schema || builderToSchema(nextBuilder)))
    setMode('builder')
  }, [draft.id])

  const rawError = useMemo(() => {
    if (mode !== 'json') return ''
    try {
      parseSchemaJSON(schemaText)
      return ''
    } catch (err) {
      return err instanceof Error ? err.message : 'Invalid JSON Schema.'
    }
  }, [mode, schemaText])

  function updateBuilder(next: SchemaBuilderDraft) {
    setBuilder(next)
    const schema = builderToSchema(next)
    setSchemaText(pretty(schema))
    onDraftChange({ schema })
  }

  function updateSchemaText(next: string) {
    setSchemaText(next)
    try {
      onDraftChange({ schema: parseSchemaJSON(next) })
    } catch {
      // Keep editing locally until JSON is valid.
    }
  }

  function switchMode(next: SchemaEditorMode) {
    if (next === 'builder') {
      try {
        const schema = parseSchemaJSON(schemaText)
        setBuilder(schemaToBuilder(schema))
        onDraftChange({ schema })
      } catch {
        return
      }
    }
    setMode(next)
  }

  const editingSchema = Boolean(draft.id || selectedID)

  return (
    <main className="flex min-w-0 flex-1 flex-col overflow-hidden rounded-3xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-sm">
      <header className="flex shrink-0 flex-col gap-5 border-b border-[var(--color-border-soft)] p-5 xl:flex-row xl:items-start xl:justify-between">
        <div className="min-w-0 flex-1">
          <p className="text-xs font-bold uppercase tracking-[0.32em] text-[var(--color-blue)]">
            {editingSchema ? 'Editing schema' : 'New schema'}
          </p>
          <div className="mt-2 flex flex-wrap items-center gap-3">
            <h1 className="truncate text-2xl font-bold text-[var(--color-ink)]">
              {editingSchema ? draft.name || selectedID || 'Schema' : 'Create validation schema'}
            </h1>
            <span className="rounded-full bg-[var(--color-surface-soft)] px-3 py-1 font-mono text-xs font-bold text-[var(--color-muted)] ring-1 ring-[var(--color-border-soft)]">
              {draft.draft || 'draft-07'}
            </span>
          </div>
          <p className="mt-2 max-w-3xl text-sm text-[var(--color-muted)]">
            {editingSchema
              ? 'Edit this persisted JSON Schema. Use the builder for structured changes or JSON mode for advanced cases.'
              : 'Create a reusable JSON Schema for request validation or context validation.'}
          </p>
          {editingSchema && selectedID && (
            <p className="mt-2 font-mono text-xs text-[var(--color-muted-soft)]">{selectedID}</p>
          )}
          <div className="mt-5 grid gap-3 md:grid-cols-[1fr_220px]">
            <TextInput label="Name" value={draft.name} onChange={(name) => onDraftChange({ name })} placeholder="request-user-body" />
            <TextInput label="Draft" value={draft.draft || 'draft-07'} onChange={(draftValue) => onDraftChange({ draft: draftValue })} placeholder="draft-07" />
            <label className="grid gap-1 md:col-span-2">
              <span className="text-xs font-bold uppercase tracking-[0.24em] text-[var(--color-muted-soft)]">Description</span>
              <textarea value={draft.description || ''} onChange={(event) => onDraftChange({ description: event.target.value })} placeholder="What this schema validates..." className="min-h-20 resize-none rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-4 py-3 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]" />
            </label>
          </div>
        </div>
        <div className="flex shrink-0 items-center justify-end gap-2">
          <ModeButton mode="builder" active={mode === 'builder'} onClick={() => switchMode('builder')} />
          <ModeButton mode="json" active={mode === 'json'} onClick={() => switchMode('json')} />
          <span title={dirty ? 'Unsaved changes' : 'Saved'} className={['inline-flex h-9 w-9 items-center justify-center rounded-xl border', dirty ? 'border-[var(--color-warning-border)] bg-[var(--color-warning-soft)] text-[var(--color-warning)]' : 'border-[var(--color-border-soft)] bg-[var(--color-surface-soft)] text-[var(--color-muted)]'].join(' ')}>{dirty ? <ChangesIcon /> : <SavedIcon />}</span>
          {onDelete && <IconButton onClick={onDelete} disabled={!selectedID || saving} label="Delete schema" variant="danger" size="md"><Trash2 size={16} /></IconButton>}
          <IconButton onClick={onSave} disabled={saving || !dirty || Boolean(rawError)} label="Save schema" variant="primary" size="md">{saving ? <SpinnerIcon /> : <SaveIcon />}</IconButton>
        </div>
      </header>
      {error && <div className="m-5 rounded-2xl border border-[var(--color-red)] bg-[var(--color-red-soft)] px-4 py-3 text-sm text-[var(--color-red)]">{error}</div>}
      {rawError && <div className="mx-5 mt-5 rounded-2xl border border-[var(--color-warning)] bg-[var(--color-warning-soft)] px-4 py-3 text-sm text-[var(--color-warning)]">{rawError}</div>}
      <div className="min-h-0 flex-1 overflow-auto p-5">
        {mode === 'builder' ? <SchemaBuilder value={builder} onChange={updateBuilder} /> : <SchemaJsonEditor value={schemaText} onChange={updateSchemaText} />}
      </div>
    </main>
  )
}

function ModeButton({ mode, active, onClick }: { mode: SchemaEditorMode; active: boolean; onClick: () => void }) {
  return <IconButton label={mode === 'builder' ? 'Builder mode' : 'JSON mode'} active={active} variant="secondary" size="md" onClick={onClick}>{mode === 'builder' ? <span className="text-xs">UI</span> : <FileCode2 size={16} />}</IconButton>
}

function TextInput({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (value: string) => void; placeholder?: string }) {
  return (
    <label className="grid gap-1">
      <span className="text-xs font-bold uppercase tracking-[0.24em] text-[var(--color-muted-soft)]">{label}</span>
      <input value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} className="rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-4 py-3 text-base font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]" />
    </label>
  )
}

function pretty(value: unknown) {
  return JSON.stringify(value || {}, null, 2)
}
