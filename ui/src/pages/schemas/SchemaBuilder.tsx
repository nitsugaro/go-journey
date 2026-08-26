import { Trash2 } from 'lucide-react'
import { IconButton } from '../../components/Button'
import { TagsInput } from '../../components/TagsInput'
import { PlusIcon } from '../../components/Icons'
import type { SchemaBuilderDraft, SchemaPropertyDraft, SchemaPropertyType } from './schemaTypes'
import { schemaStringFormats, schemaStringRules, schemaTypes } from './schemaUtils'

export function SchemaBuilder({ value, onChange }: { value: SchemaBuilderDraft; onChange: (value: SchemaBuilderDraft) => void }) {
  return (
    <div className="grid gap-4">
      <div className="rounded-3xl bg-[var(--color-surface-subtle)] p-4 ring-1 ring-[var(--color-border-soft)]">
        <label className="flex items-center justify-between gap-4">
          <span>
            <span className="block text-xs font-bold uppercase tracking-[0.24em] text-[var(--color-muted-soft)]">Additional properties</span>
            <span className="text-xs text-[var(--color-muted)]">Allow keys not declared below.</span>
          </span>
          <input
            type="checkbox"
            checked={value.additionalProperties}
            onChange={(event) => onChange({ ...value, additionalProperties: event.target.checked })}
            className="h-5 w-5 accent-[var(--color-blue)]"
          />
        </label>
      </div>
      <div className="rounded-3xl bg-[var(--color-surface-subtle)] p-4 ring-1 ring-[var(--color-border-soft)]">
        <SchemaFieldsList
          title="Properties"
          description="Typed fields that this schema validates."
          properties={value.properties}
          onChange={(properties) => onChange({ ...value, properties })}
        />
      </div>
    </div>
  )
}

function SchemaFieldsList({
  title,
  description,
  properties,
  onChange,
}: {
  title: string
  description: string
  properties: SchemaPropertyDraft[]
  onChange: (properties: SchemaPropertyDraft[]) => void
}) {
  const updateField = (id: string, patch: Partial<SchemaPropertyDraft>) => {
    onChange(properties.map((field) => (field.id === id ? { ...field, ...patch } : field)))
  }
  return (
    <>
      <div className="flex items-center justify-between gap-3">
        <div>
          <p className="text-xs font-bold uppercase tracking-[0.24em] text-[var(--color-muted-soft)]">{title}</p>
          <p className="text-xs text-[var(--color-muted)]">{description}</p>
        </div>
        <IconButton label="Add property" variant="primary" size="sm" onClick={() => onChange([...properties, newField()])}>
          <PlusIcon size={16} />
        </IconButton>
      </div>
      <div className="mt-4 grid gap-3">
        {properties.length === 0 && <p className="rounded-2xl bg-[var(--color-surface)] px-4 py-3 text-sm text-[var(--color-muted)]">No properties configured.</p>}
        {properties.map((field) => (
          <SchemaFieldCard
            key={field.id}
            field={field}
            onChange={(patch) => updateField(field.id, patch)}
            onRemove={() => onChange(properties.filter((item) => item.id !== field.id))}
          />
        ))}
      </div>
    </>
  )
}

function SchemaFieldCard({
  field,
  onChange,
  onRemove,
  nested = false,
  hideName = false,
  hideRequired = false,
  hideRemove = false,
}: {
  field: SchemaPropertyDraft
  onChange: (patch: Partial<SchemaPropertyDraft>) => void
  onRemove: () => void
  nested?: boolean
  hideName?: boolean
  hideRequired?: boolean
  hideRemove?: boolean
}) {
  const setType = (type: SchemaPropertyType) => {
    onChange({
      type,
      ...(type === 'object' ? { additionalProperties: field.additionalProperties ?? false, properties: field.properties || [] } : {}),
      ...(type === 'array' ? { items: field.items || newField('item') } : {}),
    })
  }
  const gridClass = hideName
    ? hideRequired && hideRemove ? 'grid gap-3 lg:grid-cols-[160px]' : 'grid gap-3 lg:grid-cols-[160px_92px_40px]'
    : 'grid gap-3 lg:grid-cols-[1fr_150px_92px_40px]'
  return (
    <div className={`rounded-3xl ${nested ? 'bg-[var(--color-surface-subtle)]' : 'bg-[var(--color-surface)]'} p-4 ring-1 ring-[var(--color-border-soft)]`}>
      <div className={gridClass}>
        {!hideName && <TextField label="Name" value={field.name} onChange={(name) => onChange({ name })} placeholder="email, age, payload..." />}
        <label className="grid gap-1">
          <span className="text-xs font-bold uppercase tracking-[0.18em] text-[var(--color-muted-soft)]">Type</span>
          <select value={field.type} onChange={(event) => setType(event.target.value as SchemaPropertyType)} className={inputClass}>
            {schemaTypes.map((type) => <option key={type} value={type}>{type}</option>)}
          </select>
        </label>
        {!hideRequired && (
          <label className="flex items-center justify-center gap-2 rounded-2xl bg-[var(--color-surface-subtle)] px-3 py-2 text-xs font-bold text-[var(--color-muted)] ring-1 ring-[var(--color-border-soft)]">
            <input type="checkbox" checked={field.required} onChange={(event) => onChange({ required: event.target.checked })} className="accent-[var(--color-blue)]" />
            Required
          </label>
        )}
        {!hideRemove && <IconButton label="Remove property" variant="danger" size="md" onClick={onRemove}><Trash2 size={16} /></IconButton>}
      </div>
      {field.type === 'string' && (
        <div className="mt-3 grid gap-3 lg:grid-cols-2">
          <TextField label="Min length" value={field.minLength || ''} onChange={(minLength) => onChange({ minLength })} type="number" />
          <TextField label="Max length" value={field.maxLength || ''} onChange={(maxLength) => onChange({ maxLength })} type="number" />
          <label className="grid gap-1">
            <span className="text-xs font-bold uppercase tracking-[0.18em] text-[var(--color-muted-soft)]">Format</span>
            <select value={field.format || ''} onChange={(event) => onChange({ format: event.target.value })} className={inputClass}>
              {schemaStringFormats.map((format) => <option key={format || 'none'} value={format}>{format || 'No format'}</option>)}
            </select>
          </label>
          <label className="grid gap-1">
            <span className="text-xs font-bold uppercase tracking-[0.18em] text-[var(--color-muted-soft)]">String rule</span>
            <select
              value={field.stringRule || ''}
              onChange={(event) => {
                const nextRule = event.target.value
                const preset = schemaStringRules.find((rule) => rule.value === nextRule)
                onChange({ stringRule: nextRule, pattern: preset?.pattern || (nextRule === 'custom' ? field.pattern || '' : '') })
              }}
              className={inputClass}
            >
              {schemaStringRules.map((rule) => <option key={rule.value || 'any'} value={rule.value}>{rule.label}</option>)}
            </select>
          </label>
          {field.stringRule === 'custom' && (
            <TextField label="Pattern" value={field.pattern || ''} onChange={(pattern) => onChange({ pattern })} placeholder="^[A-Za-z]+$" />
          )}
          <div>
            <p className="mb-1 text-xs font-bold uppercase tracking-[0.18em] text-[var(--color-muted-soft)]">Enum</p>
            <TagsInput value={field.enum || []} onChange={(next) => onChange({ enum: next })} placeholder="Type value and press Enter…" />
          </div>
        </div>
      )}
      {(field.type === 'integer' || field.type === 'number') && (
        <div className="mt-3 grid gap-3 lg:grid-cols-2">
          <TextField label="Minimum" value={field.minimum || ''} onChange={(minimum) => onChange({ minimum })} type="number" />
          <TextField label="Maximum" value={field.maximum || ''} onChange={(maximum) => onChange({ maximum })} type="number" />
        </div>
      )}
      {field.type === 'object' && (
        <div className="mt-4 rounded-3xl bg-[var(--color-surface-subtle)] p-4 ring-1 ring-[var(--color-border-soft)]">
          <div className="mb-4 grid gap-3 lg:grid-cols-2">
            <TextField label="Min properties" value={field.minProperties || ''} onChange={(minProperties) => onChange({ minProperties })} type="number" />
            <TextField label="Max properties" value={field.maxProperties || ''} onChange={(maxProperties) => onChange({ maxProperties })} type="number" />
          </div>
          <label className="mb-4 flex items-center justify-between gap-4">
            <span>
              <span className="block text-xs font-bold uppercase tracking-[0.18em] text-[var(--color-muted-soft)]">Nested additional properties</span>
              <span className="text-xs text-[var(--color-muted)]">Allow undeclared keys inside this object.</span>
            </span>
            <input
              type="checkbox"
              checked={field.additionalProperties === true}
              onChange={(event) => onChange({ additionalProperties: event.target.checked })}
              className="h-5 w-5 accent-[var(--color-blue)]"
            />
          </label>
          <SchemaFieldsList
            title="Nested properties"
            description="Fields inside this object."
            properties={field.properties || []}
            onChange={(properties) => onChange({ properties })}
          />
        </div>
      )}
      {field.type === 'array' && (
        <div className="mt-4 rounded-3xl bg-[var(--color-surface-subtle)] p-4 ring-1 ring-[var(--color-border-soft)]">
          <div className="mb-4 grid gap-3 lg:grid-cols-[1fr_1fr_150px]">
            <TextField label="Min items" value={field.minItems || ''} onChange={(minItems) => onChange({ minItems })} type="number" />
            <TextField label="Max items" value={field.maxItems || ''} onChange={(maxItems) => onChange({ maxItems })} type="number" />
            <label className="flex items-center justify-center gap-2 rounded-2xl bg-[var(--color-surface)] px-3 py-2 text-xs font-bold text-[var(--color-muted)] ring-1 ring-[var(--color-border-soft)]">
              <input type="checkbox" checked={field.uniqueItems === true} onChange={(event) => onChange({ uniqueItems: event.target.checked })} className="accent-[var(--color-blue)]" />
              Unique
            </label>
          </div>
          <p className="mb-3 text-xs font-bold uppercase tracking-[0.18em] text-[var(--color-muted-soft)]">Array items</p>
          <SchemaFieldCard
            field={field.items || newField('item')}
            onChange={(patch) => onChange({ items: { ...(field.items || newField('item')), ...patch } })}
            onRemove={() => onChange({ items: newField('item') })}
            nested
            hideName
            hideRequired
            hideRemove
          />
        </div>
      )}
    </div>
  )
}

function TextField({ label, value, onChange, placeholder, type = 'text' }: { label: string; value: string; onChange: (value: string) => void; placeholder?: string; type?: string }) {
  return (
    <label className="grid gap-1">
      <span className="text-xs font-bold uppercase tracking-[0.18em] text-[var(--color-muted-soft)]">{label}</span>
      <input type={type} value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} className={inputClass} />
    </label>
  )
}

const inputClass = 'rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-4 py-3 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]'

function newField(name = ''): SchemaPropertyDraft {
  return { id: crypto.randomUUID(), name, type: 'string', required: false, enum: [] }
}
