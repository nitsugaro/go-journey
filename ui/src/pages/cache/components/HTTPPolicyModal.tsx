import { useMemo, useState } from 'react'
import { Button, IconButton } from '../../../components/Button'
import { CloseMiniIcon } from '../../../components/Icons'
import { PlaceholderValueField } from '../../../components/PlaceholderValueField'
import { HTTPStringMapEditor } from './HTTPStringMapEditor'
import { routePolicyTypes, type HTTPRoutePolicy } from './httpRouteTableTypes'

type HTTPPolicyModalProps = {
  policy?: HTTPRoutePolicy
  onClose: () => void
  onSave: (policy: HTTPRoutePolicy) => void
}

export function HTTPPolicyModal({ policy, onClose, onSave }: HTTPPolicyModalProps) {
  const [draft, setDraft] = useState<HTTPRoutePolicy>(policy || { type: routePolicyTypes[0], config: {} })
  const isCustom = useMemo(() => Boolean(draft.name && !draft.type), [draft.name, draft.type])

  return (
    <div className="motion-modal-backdrop fixed inset-0 z-[1300] flex items-center justify-center bg-[var(--color-overlay)] p-5" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose() }}>
      <section className="motion-modal-surface w-[min(760px,94vw)] overflow-hidden rounded-[2rem] border border-[var(--color-border)] bg-[var(--color-surface)] shadow-2xl" onMouseDown={(event) => event.stopPropagation()}>
        <header className="flex items-start justify-between gap-4 border-b border-[var(--color-border-soft)] p-6">
          <div>
            <p className="text-xs font-bold uppercase tracking-[0.32em] text-[var(--color-blue)]">Route policy</p>
            <h2 className="mt-2 text-xl font-bold text-[var(--color-ink)]">{policy ? 'Edit policy' : 'Add policy'}</h2>
          </div>
          <IconButton label="Close" size="sm" variant="secondary" onClick={onClose}><CloseMiniIcon /></IconButton>
        </header>
        <div className="grid gap-5 p-6">
          <div className="inline-flex w-fit rounded-2xl bg-[var(--color-surface-subtle)] p-1 ring-1 ring-[var(--color-border-soft)]">
            <Button size="sm" variant="ghost" active={!isCustom} onClick={() => setDraft({ ...draft, name: undefined, type: draft.type || routePolicyTypes[0] })}>Built-in</Button>
            <Button size="sm" variant="ghost" active={isCustom} onClick={() => setDraft({ ...draft, type: undefined, name: draft.name || 'custom_policy' })}>Custom</Button>
          </div>
          {isCustom ? (
            <LabeledInput label="Policy name" value={draft.name || ''} onChange={(name) => setDraft({ ...draft, name, type: undefined })} />
          ) : (
            <label className="grid gap-2">
              <span className="text-xs font-bold uppercase tracking-[0.24em] text-[var(--color-muted-soft)]">Built-in type</span>
              <input list="http-route-policy-types" value={draft.type || routePolicyTypes[0]} onChange={(event) => setDraft({ ...draft, type: event.target.value, name: undefined })} placeholder="Policy type or ${env.policy_type}" className="rounded-2xl bg-[var(--color-surface-subtle)] px-4 py-3 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:ring-2 focus:ring-[var(--color-blue)]" />
              <datalist id="http-route-policy-types">{routePolicyTypes.map((type) => <option key={type} value={type} />)}</datalist>
            </label>
          )}
          <PlaceholderValueField label="Policy config" value={draft.config} emptyValue={{}} onChange={(config) => setDraft({ ...draft, config })}>
            {(config) => <HTTPStringMapEditor value={stringConfig(config)} onChange={(next) => setDraft({ ...draft, config: next })} keyPlaceholder="param" valuePlaceholder="expected value" />}
          </PlaceholderValueField>
        </div>
        <footer className="flex justify-end gap-3 border-t border-[var(--color-border-soft)] p-5">
          <Button variant="ghost" onClick={onClose}>Cancel</Button>
          <Button variant="primary" onClick={() => onSave(draft)}>Save policy</Button>
        </footer>
      </section>
    </div>
  )
}

function LabeledInput({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return (
    <label className="grid gap-2">
      <span className="text-xs font-bold uppercase tracking-[0.24em] text-[var(--color-muted-soft)]">{label}</span>
      <input value={value} onChange={(event) => onChange(event.target.value)} className="rounded-2xl bg-[var(--color-surface-subtle)] px-4 py-3 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:ring-2 focus:ring-[var(--color-blue)]" />
    </label>
  )
}

function stringConfig(value?: Record<string, unknown>): Record<string, string> {
  if (!value) return {}
  return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, String(item)]))
}
