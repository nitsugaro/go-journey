import { useState, type ReactNode } from 'react'
import { Button, IconButton } from '../../../components/Button'
import { CloseMiniIcon } from '../../../components/Icons'
import { TagsInput } from '../../../components/TagsInput'
import { PlaceholderValueField } from '../../../components/PlaceholderValueField'
import { HTTPStringMapEditor } from './HTTPStringMapEditor'
import type { HTTPRouteUpstream } from './httpRouteTableTypes'

type HTTPUpstreamModalProps = {
  upstreamID?: string
  upstream?: HTTPRouteUpstream
  onClose: () => void
  onSave: (upstreamID: string, upstream: HTTPRouteUpstream) => void
}

export function HTTPUpstreamModal({ upstreamID, upstream, onClose, onSave }: HTTPUpstreamModalProps) {
  const [id, setID] = useState(upstreamID || '')
  const [draft, setDraft] = useState<HTTPRouteUpstream>(upstream || {})
  const canSave = id.trim() && draft.url?.trim()

  return (
    <div className="motion-modal-backdrop fixed inset-0 z-[1300] flex items-center justify-center bg-[var(--color-overlay)] p-5" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose() }}>
      <section className="motion-modal-surface max-h-[88vh] w-[min(860px,94vw)] overflow-hidden rounded-[2rem] border border-[var(--color-border)] bg-[var(--color-surface)] shadow-2xl" onMouseDown={(event) => event.stopPropagation()}>
        <header className="flex items-start justify-between gap-4 border-b border-[var(--color-border-soft)] p-6">
          <div>
            <p className="text-xs font-bold uppercase tracking-[0.32em] text-[var(--color-blue)]">Upstream</p>
            <h2 className="mt-2 text-xl font-bold text-[var(--color-ink)]">{upstreamID ? 'Edit upstream' : 'Add upstream'}</h2>
          </div>
          <IconButton label="Close" size="sm" variant="secondary" onClick={onClose}><CloseMiniIcon /></IconButton>
        </header>
        <div className="grid max-h-[calc(88vh-150px)] gap-5 overflow-auto p-6">
          <div className="grid gap-4 md:grid-cols-[260px_1fr]">
            <Field label="Upstream id" value={id} onChange={setID} disabled={Boolean(upstreamID)} placeholder="users-api" />
            <Field label="URL" value={draft.url || ''} onChange={(url) => setDraft({ ...draft, url })} placeholder="https://api.example.com" />
          </div>
          <Field label="HTTP instance" value={draft.http_instance || ''} onChange={(http_instance) => setDraft({ ...draft, http_instance })} placeholder="default client if empty" />
          <Section title="Headers" subtitle="Headers added to every proxied request using this upstream.">
            <PlaceholderValueField label="Add headers" value={draft.add_headers} emptyValue={{}} onChange={(add_headers) => setDraft({ ...draft, add_headers })}>
              {(addHeaders) => <HTTPStringMapEditor value={addHeaders} onChange={(add_headers) => setDraft({ ...draft, add_headers })} keyPlaceholder="Header" />}
            </PlaceholderValueField>
          </Section>
          <Section title="Strip headers" subtitle="Headers removed before sending the request upstream.">
            <PlaceholderValueField label="Strip headers" value={draft.strip_headers} emptyValue={[]} onChange={(strip_headers) => setDraft({ ...draft, strip_headers })}>
              {(stripHeaders) => <TagsInput value={stripHeaders} onChange={(strip_headers) => setDraft({ ...draft, strip_headers })} placeholder="Type header name and press Enter…" />}
            </PlaceholderValueField>
          </Section>
        </div>
        <footer className="flex justify-end gap-3 border-t border-[var(--color-border-soft)] p-5">
          <Button variant="ghost" onClick={onClose}>Cancel</Button>
          <Button variant="primary" disabled={!canSave} onClick={() => onSave(id.trim(), draft)}>Save upstream</Button>
        </footer>
      </section>
    </div>
  )
}

function Section({ title, subtitle, children }: { title: string; subtitle: string; children: ReactNode }) {
  return (
    <section className="rounded-3xl border border-[var(--color-border-soft)] bg-[var(--color-surface-soft)] p-5">
      <p className="text-xs font-bold uppercase tracking-[0.24em] text-[var(--color-muted-soft)]">{title}</p>
      <p className="mt-1 text-xs text-[var(--color-muted)]">{subtitle}</p>
      <div className="mt-4">{children}</div>
    </section>
  )
}

function Field({ label, value, onChange, placeholder = '', disabled = false }: { label: string; value: string; onChange: (value: string) => void; placeholder?: string; disabled?: boolean }) {
  return (
    <label className="grid gap-2">
      <span className="text-xs font-bold uppercase tracking-[0.24em] text-[var(--color-muted-soft)]">{label}</span>
      <input disabled={disabled} value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} className="rounded-2xl bg-[var(--color-surface-subtle)] px-4 py-3 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:ring-2 focus:ring-[var(--color-blue)] disabled:text-[var(--color-muted)]" />
    </label>
  )
}
