import { useMemo, useState, type ReactNode } from 'react'
import { Button, IconButton } from '../../../components/Button'
import { CloseMiniIcon, DeleteIcon, EditMiniIcon, PlusIcon } from '../../../components/Icons'
import { TagsInput } from '../../../components/TagsInput'
import { PlaceholderValueField } from '../../../components/PlaceholderValueField'
import { HTTPStringMapEditor } from './HTTPStringMapEditor'
import { HTTPPolicyModal } from './HTTPPolicyModal'
import { httpMethods, sanitizePolicy, type HTTPRoute, type HTTPRoutePolicy } from './httpRouteTableTypes'

type HTTPRouteModalProps = {
  route?: HTTPRoute
  upstreamIDs: string[]
  onClose: () => void
  onSave: (route: HTTPRoute) => void
}

export function HTTPRouteModal({ route, upstreamIDs, onClose, onSave }: HTTPRouteModalProps) {
  const [draft, setDraft] = useState<HTTPRoute>(route || { methods: ['GET'], uris: ['*://*/**'] })
  const [policyIndex, setPolicyIndex] = useState<number | 'new' | null>(null)
  const [metadataText, setMetadataText] = useState(JSON.stringify(draft.metadata || {}, null, 2))
  const [error, setError] = useState('')
  const title = route ? 'Edit route group' : 'Add route group'
  const policies = Array.isArray(draft.policies) ? draft.policies : []
  const canSave = typeof draft.uris === 'string' ? Boolean(draft.uris.trim()) : (draft.uris || []).length > 0

  const selectedPolicy = useMemo(() => (
    typeof policyIndex === 'number' ? policies[policyIndex] : undefined
  ), [policyIndex, policies])

  const save = () => {
    try {
      const metadata = typeof draft.metadata === 'string'
        ? draft.metadata
        : metadataText.trim() ? JSON.parse(metadataText) : {}
      setError('')
      onSave({ ...draft, metadata })
    } catch {
      setError('Metadata must be valid JSON.')
    }
  }

  const savePolicy = (policy: HTTPRoutePolicy) => {
    const next = sanitizePolicy(policy)
    if (typeof policyIndex === 'number') {
      setDraft({ ...draft, policies: policies.map((item, index) => (index === policyIndex ? next : item)) })
    } else {
      setDraft({ ...draft, policies: [...policies, next] })
    }
    setPolicyIndex(null)
  }

  return (
    <>
      <div className="motion-modal-backdrop fixed inset-0 z-[1300] flex items-center justify-center bg-[var(--color-overlay)] p-5" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose() }}>
        <section className="motion-modal-surface max-h-[90vh] w-[min(1120px,96vw)] overflow-hidden rounded-[2rem] border border-[var(--color-border)] bg-[var(--color-surface)] shadow-2xl" onMouseDown={(event) => event.stopPropagation()}>
          <header className="flex items-start justify-between gap-4 border-b border-[var(--color-border-soft)] p-6">
            <div>
              <p className="text-xs font-bold uppercase tracking-[0.32em] text-[var(--color-blue)]">HTTP route table</p>
              <h2 className="mt-2 text-xl font-bold text-[var(--color-ink)]">{title}</h2>
            </div>
            <IconButton label="Close" size="sm" variant="secondary" onClick={onClose}><CloseMiniIcon /></IconButton>
          </header>
          <div className="grid max-h-[calc(90vh-152px)] gap-5 overflow-auto p-6">
            {error && <p className="rounded-2xl border border-[var(--color-red)] bg-[var(--color-red-soft)] px-4 py-3 text-sm text-[var(--color-red)]">{error}</p>}
            <Section title="Match" subtitle="Complete URI patterns and methods that select this policy group.">
              <Field label="Name" value={draft.name || ''} onChange={(name) => setDraft({ ...draft, name })} placeholder="Public user API" />
              <PlaceholderValueField label="URIs" required value={draft.uris} emptyValue={[]} onChange={(uris) => setDraft({ ...draft, uris })}>
                {(uris) => <TagsInput value={uris} onChange={(next) => setDraft({ ...draft, uris: next })} placeholder="https://api.example.com/users/{id}" />}
              </PlaceholderValueField>
              <PlaceholderValueField label="Methods" value={draft.methods} emptyValue={[]} onChange={(methods) => setDraft({ ...draft, methods })}>
                {(methods) => <div className="flex flex-wrap gap-2">
                  {httpMethods.map((method) => (
                    <Button key={method} size="sm" variant="ghost" active={methods.includes(method)} onClick={() => toggleMethod(method, draft, setDraft)}>
                      {method}
                    </Button>
                  ))}
                </div>}
              </PlaceholderValueField>
            </Section>
            <Section title="Target" subtitle="Optional legacy target overrides. The HTTPProxy step base_url and rewrite take precedence.">
              <div className="grid gap-4 md:grid-cols-2">
                <label className="grid gap-2">
                  <span className="text-xs font-bold uppercase tracking-[0.24em] text-[var(--color-muted-soft)]">Upstream</span>
                  <input list="http-route-upstream-ids" value={draft.upstream || ''} onChange={(event) => setDraft({ ...draft, upstream: event.target.value })} placeholder="Upstream id, absolute URL or ${env.upstream}" className="rounded-2xl bg-[var(--color-surface-subtle)] px-4 py-3 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:ring-2 focus:ring-[var(--color-blue)]" />
                  <datalist id="http-route-upstream-ids">{upstreamIDs.map((id) => <option key={id} value={id} />)}</datalist>
                </label>
                <Field label="HTTP instance" value={draft.http_instance || ''} onChange={(http_instance) => setDraft({ ...draft, http_instance })} placeholder="route override" />
              </div>
              <Field label="Rewrite" value={draft.rewrite || ''} onChange={(rewrite) => setDraft({ ...draft, rewrite })} placeholder="/v1/{path} or ${env.rewrite}" />
            </Section>
            <Section title="Headers" subtitle="Route-level header transforms before proxying.">
              <div className="grid gap-4 lg:grid-cols-2">
                <PlaceholderValueField label="Add headers" value={draft.add_headers} emptyValue={{}} onChange={(add_headers) => setDraft({ ...draft, add_headers })}>
                  {(addHeaders) => <HTTPStringMapEditor value={addHeaders} onChange={(add_headers) => setDraft({ ...draft, add_headers })} />}
                </PlaceholderValueField>
                <PlaceholderValueField label="Strip headers" value={draft.strip_headers} emptyValue={[]} onChange={(strip_headers) => setDraft({ ...draft, strip_headers })}>
                  {(stripHeaders) => <TagsInput value={stripHeaders} onChange={(strip_headers) => setDraft({ ...draft, strip_headers })} placeholder="Header to remove…" />}
                </PlaceholderValueField>
              </div>
            </Section>
            <Section title="Policies" subtitle="Authorization and matching guards evaluated before proxying.">
              <PlaceholderValueField label="Policies configuration" value={draft.policies} emptyValue={[]} onChange={(nextPolicies) => setDraft({ ...draft, policies: nextPolicies })}>
                {() => <>
                  <div className="grid gap-3">
                    {policies.length === 0 && <p className="rounded-2xl bg-[var(--color-surface)] px-4 py-3 text-sm text-[var(--color-muted)]">No policies configured.</p>}
                    {policies.map((policy, index) => (
                      <PolicyCard key={index} policy={policy} onEdit={() => setPolicyIndex(index)} onDelete={() => setDraft({ ...draft, policies: policies.filter((_, itemIndex) => itemIndex !== index) })} />
                    ))}
                  </div>
                  <div className="mt-4"><Button variant="secondary" size="sm" onClick={() => setPolicyIndex('new')}><PlusIcon size={16} /> Add policy</Button></div>
                </>}
              </PlaceholderValueField>
            </Section>
            <Section title="Metadata" subtitle="Route metadata returned to the proxy step for developer-specific decisions.">
              <PlaceholderValueField label="Metadata configuration" value={draft.metadata} emptyValue={{}} onChange={(metadata) => {
                setDraft({ ...draft, metadata })
                if (typeof metadata === 'string') setMetadataText(metadata)
                else setMetadataText(JSON.stringify(metadata, null, 2))
              }}>
                {() => <textarea value={metadataText} onChange={(event) => setMetadataText(event.target.value)} rows={8} spellCheck={false} className="w-full resize-y rounded-2xl bg-[var(--color-surface)] p-4 font-mono text-sm text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:ring-2 focus:ring-[var(--color-blue)]" />}
              </PlaceholderValueField>
            </Section>
          </div>
          <footer className="flex justify-end gap-3 border-t border-[var(--color-border-soft)] p-5">
            <Button variant="ghost" onClick={onClose}>Cancel</Button>
            <Button variant="primary" disabled={!canSave} onClick={save}>Save route</Button>
          </footer>
        </section>
      </div>
      {policyIndex !== null && <HTTPPolicyModal policy={selectedPolicy} onClose={() => setPolicyIndex(null)} onSave={savePolicy} />}
    </>
  )
}

function Section({ title, subtitle, children }: { title: string; subtitle: string; children: ReactNode }) {
  return <section className="grid gap-4 rounded-3xl border border-[var(--color-border-soft)] bg-[var(--color-surface-soft)] p-5"><div><p className="text-xs font-bold uppercase tracking-[0.24em] text-[var(--color-muted-soft)]">{title}</p><p className="mt-1 text-xs text-[var(--color-muted)]">{subtitle}</p></div>{children}</section>
}

function Field({ label, value, onChange, placeholder = '', type = 'text' }: { label: string; value: string; onChange: (value: string) => void; placeholder?: string; type?: string }) {
  return <label className="grid gap-2"><span className="text-xs font-bold uppercase tracking-[0.24em] text-[var(--color-muted-soft)]">{label}</span><input type={type} value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} className="rounded-2xl bg-[var(--color-surface-subtle)] px-4 py-3 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:ring-2 focus:ring-[var(--color-blue)]" /></label>
}

function PolicyCard({ policy, onEdit, onDelete }: { policy: HTTPRoutePolicy; onEdit: () => void; onDelete: () => void }) {
  const configLabel = typeof policy.config === 'string' ? 'placeholder config' : `${Object.keys(policy.config || {}).length} config fields`
  return <article className="flex items-center justify-between gap-3 rounded-2xl bg-[var(--color-surface)] px-4 py-3 ring-1 ring-[var(--color-border-soft)]"><div><p className="font-semibold text-[var(--color-ink)]">{policy.name || policy.type || 'policy'}</p><p className="text-xs text-[var(--color-muted)]">{configLabel}</p></div><div className="flex gap-2"><IconButton label="Edit policy" size="sm" variant="secondary" onClick={onEdit}><EditMiniIcon /></IconButton><IconButton label="Delete policy" size="sm" variant="danger" onClick={onDelete}><DeleteIcon size={15} /></IconButton></div></article>
}

function toggleMethod(method: string, draft: HTTPRoute, setDraft: (route: HTTPRoute) => void) {
  const methods = Array.isArray(draft.methods) ? draft.methods : []
  setDraft({ ...draft, methods: methods.includes(method) ? methods.filter((item) => item !== method) : [...methods, method] })
}
