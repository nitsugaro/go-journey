import { useMemo, useState, type ReactNode } from 'react'
import { Button, IconButton } from '../../../components/Button'
import { ArrowDownMiniIcon, ArrowUpMiniIcon, DeleteIcon, EditMiniIcon, PlusIcon } from '../../../components/Icons'
import { TagsInput } from '../../../components/TagsInput'
import { PlaceholderValueField } from '../../../components/PlaceholderValueField'
import { HTTPStringMapEditor } from './HTTPStringMapEditor'
import { HTTPRouteModal } from './HTTPRouteModal'
import { HTTPUpstreamModal } from './HTTPUpstreamModal'
import { parseRouteTableConfig, sanitizeRouteTableConfig, type HTTPRoute, type HTTPRouteTableConfig, type HTTPRouteUpstream } from './httpRouteTableTypes'

type HTTPRouteTableFormProps = {
  configText: string
  onChange: (value: string) => void
}

export function HTTPRouteTableForm({ configText, onChange }: HTTPRouteTableFormProps) {
  const [routeIndex, setRouteIndex] = useState<number | 'new' | null>(null)
  const [upstreamID, setUpstreamID] = useState<string | 'new' | null>(null)
  const parsed = useMemo(() => safeParse(configText), [configText])

  if (!parsed.ok) {
    return <div className="rounded-3xl border border-[var(--color-red)] bg-[var(--color-red-soft)] p-5 text-sm text-[var(--color-red)]">Invalid route table JSON: {parsed.error}</div>
  }

  const config = parsed.value
  const upstreams = typeof config.upstreams === 'string' ? {} : config.upstreams || {}
  const routes = Array.isArray(config.routes) ? config.routes : []
  const upstreamIDs = Object.keys(upstreams)

  const commit = (next: HTTPRouteTableConfig) => onChange(JSON.stringify(sanitizeRouteTableConfig(next), null, 2))
  const routeDraft = typeof routeIndex === 'number' ? routes[routeIndex] : undefined
  const upstreamDraft = upstreamID && upstreamID !== 'new' ? upstreams[upstreamID] : undefined

  return (
    <>
      <div className="grid gap-5">
        <Panel title="Routing defaults" subtitle="Global values used when a route or upstream does not override them.">
          <div className="grid gap-4 lg:grid-cols-2">
            <Field label="Default HTTP instance" value={config.default_http_instance || ''} onChange={(default_http_instance) => commit({ ...config, default_http_instance })} placeholder="default" />
            <PlaceholderValueField label="Strip headers" value={config.strip_headers} emptyValue={[]} onChange={(strip_headers) => commit({ ...config, strip_headers })}>
              {(stripHeaders) => <TagsInput value={stripHeaders || []} onChange={(strip_headers) => commit({ ...config, strip_headers })} />}
            </PlaceholderValueField>
          </div>
          <PlaceholderValueField label="Add headers" value={config.add_headers} emptyValue={{}} onChange={(add_headers) => commit({ ...config, add_headers })}>
            {(addHeaders) => <HTTPStringMapEditor value={addHeaders} onChange={(add_headers) => commit({ ...config, add_headers })} keyPlaceholder="Header" />}
          </PlaceholderValueField>
        </Panel>
        <Panel title="Upstreams" subtitle="Reusable backend targets. Routes can reference them by id.">
          <PlaceholderValueField label="Upstreams configuration" value={config.upstreams} emptyValue={{}} onChange={(upstreams) => commit({ ...config, upstreams })}>
            {() => <>
              <div className="grid gap-3 lg:grid-cols-2">
                {upstreamIDs.length === 0 && <Empty text="No upstreams configured." />}
                {upstreamIDs.map((id) => (
                  <UpstreamCard
                    key={id}
                    id={id}
                    upstream={upstreams[id]}
                    onEdit={() => setUpstreamID(id)}
                    onDelete={() => commit({ ...config, upstreams: omit(upstreams, id) })}
                  />
                ))}
              </div>
              <Button variant="secondary" size="sm" className="mt-4" onClick={() => setUpstreamID('new')}><PlusIcon size={16} /> Add upstream</Button>
            </>}
          </PlaceholderValueField>
        </Panel>
        <Panel title="Route groups" subtitle="Ordered rules: index 0 has the highest priority. The first matching group owns policy evaluation and routing.">
          <PlaceholderValueField label="Routes configuration" value={config.routes} emptyValue={[]} onChange={(routes) => commit({ ...config, routes })} required>
            {() => <>
              <div className="grid gap-3">
                {routes.length === 0 && <Empty text="No routes configured." />}
                {routes.map((route, index) => (
                  <RouteCard
                    key={index}
                    index={index}
                    route={route}
                    onEdit={() => setRouteIndex(index)}
                    onDelete={() => commit({ ...config, routes: routes.filter((_, itemIndex) => itemIndex !== index) })}
                    onMoveUp={() => commit({ ...config, routes: moveItem(routes, index, index - 1) })}
                    onMoveDown={() => commit({ ...config, routes: moveItem(routes, index, index + 1) })}
                    first={index === 0}
                    last={index === routes.length - 1}
                  />
                ))}
              </div>
              <Button variant="secondary" size="sm" className="mt-4" onClick={() => setRouteIndex('new')}><PlusIcon size={16} /> Add route group</Button>
            </>}
          </PlaceholderValueField>
        </Panel>
      </div>
      {routeIndex !== null && (
        <HTTPRouteModal
          route={routeDraft}
          upstreamIDs={upstreamIDs}
          onClose={() => setRouteIndex(null)}
          onSave={(route) => {
            commit({ ...config, routes: typeof routeIndex === 'number' ? routes.map((item, index) => (index === routeIndex ? route : item)) : [...routes, route] })
            setRouteIndex(null)
          }}
        />
      )}
      {upstreamID !== null && (
        <HTTPUpstreamModal
          upstreamID={upstreamID === 'new' ? undefined : upstreamID}
          upstream={upstreamDraft}
          onClose={() => setUpstreamID(null)}
          onSave={(id, upstream) => {
            commit({ ...config, upstreams: { ...upstreams, [id]: upstream } })
            setUpstreamID(null)
          }}
        />
      )}
    </>
  )
}

function Panel({ title, subtitle, children }: { title: string; subtitle: string; children: ReactNode }) {
  return <section className="rounded-[2rem] border border-[var(--color-border)] bg-[var(--color-surface-subtle)] p-5 shadow-sm"><div className="mb-5"><p className="text-xs font-bold uppercase tracking-[0.28em] text-[var(--color-blue)]">{title}</p><p className="mt-1 text-sm text-[var(--color-muted)]">{subtitle}</p></div><div className="grid gap-4">{children}</div></section>
}

function RouteCard({ route, index, onEdit, onDelete, onMoveUp, onMoveDown, first, last }: { route: HTTPRoute; index: number; onEdit: () => void; onDelete: () => void; onMoveUp: () => void; onMoveDown: () => void; first: boolean; last: boolean }) {
  const uris = Array.isArray(route.uris) ? route.uris : []
  const methods = Array.isArray(route.methods) ? route.methods : []
  const policies = Array.isArray(route.policies) ? route.policies : []
  const uriPlaceholder = typeof route.uris === 'string' ? route.uris : ''
  return <article className="rounded-3xl border border-[var(--color-border-soft)] bg-[var(--color-surface)] p-4"><div className="flex items-start gap-3"><span className="rounded-full bg-[var(--color-blue-soft)] px-2.5 py-1 font-mono text-xs font-bold text-[var(--color-blue)]">#{index}</span><div className="min-w-0 flex-1"><CardHeader title={route.name || uriPlaceholder || uris.join(', ') || 'Route group'} subtitle={`${uriPlaceholder ? 'placeholder' : uris.length} URIs · ${typeof route.policies === 'string' ? 'placeholder' : policies.length} policies`} onEdit={onEdit} onDelete={onDelete} onMoveUp={onMoveUp} onMoveDown={onMoveDown} first={first} last={last} /><ChipRow values={[...methods, ...(typeof route.methods === 'string' ? [route.methods] : []), ...uris, ...(uriPlaceholder ? [uriPlaceholder] : [])]} /></div></div></article>
}

function UpstreamCard({ id, upstream, onEdit, onDelete }: { id: string; upstream: HTTPRouteUpstream; onEdit: () => void; onDelete: () => void }) {
  const stripHeaders = Array.isArray(upstream.strip_headers) ? upstream.strip_headers : typeof upstream.strip_headers === 'string' ? [upstream.strip_headers] : []
  return <article className="rounded-3xl border border-[var(--color-border-soft)] bg-[var(--color-surface)] p-4"><CardHeader title={id} subtitle={upstream.url || 'No URL configured'} onEdit={onEdit} onDelete={onDelete} /><ChipRow values={[upstream.http_instance || '', ...stripHeaders]} /></article>
}

function CardHeader({ title, subtitle, onEdit, onDelete, onMoveUp, onMoveDown, first, last }: { title: string; subtitle: string; onEdit: () => void; onDelete: () => void; onMoveUp?: () => void; onMoveDown?: () => void; first?: boolean; last?: boolean }) {
  return <div className="flex items-start justify-between gap-3"><div className="min-w-0"><h3 className="truncate font-bold text-[var(--color-ink)]">{title}</h3><p className="mt-1 truncate text-xs text-[var(--color-muted)]">{subtitle}</p></div><div className="flex shrink-0 gap-1">{onMoveUp && onMoveDown && <><IconButton label="Move route up" size="sm" variant="ghost" onClick={onMoveUp} disabled={first}><ArrowUpMiniIcon /></IconButton><IconButton label="Move route down" size="sm" variant="ghost" onClick={onMoveDown} disabled={last}><ArrowDownMiniIcon /></IconButton></>}<IconButton label="Edit" size="sm" variant="secondary" onClick={onEdit}><EditMiniIcon /></IconButton><IconButton label="Delete" size="sm" variant="danger" onClick={onDelete}><DeleteIcon size={15} /></IconButton></div></div>
}

function Field({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (value: string) => void; placeholder?: string }) {
  return <label className="grid gap-2"><span className="text-xs font-bold uppercase tracking-[0.22em] text-[var(--color-muted-soft)]">{label}</span><input value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} className="rounded-2xl bg-[var(--color-surface)] px-4 py-3 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:ring-2 focus:ring-[var(--color-blue)]" /></label>
}

function ChipRow({ values }: { values: string[] }) {
  const list = values.filter(Boolean)
  if (list.length === 0) return null
  return <div className="mt-3 flex flex-wrap gap-2">{list.map((value) => <span key={value} className="rounded-full bg-[var(--color-blue-soft)] px-2.5 py-1 text-xs font-semibold text-[var(--color-blue)]">{value}</span>)}</div>
}

function Empty({ text }: { text: string }) {
  return <p className="rounded-2xl bg-[var(--color-surface)] px-4 py-3 text-sm text-[var(--color-muted)]">{text}</p>
}

function omit<T>(record: Record<string, T>, key: string): Record<string, T> {
  return Object.fromEntries(Object.entries(record).filter(([itemKey]) => itemKey !== key))
}

function moveItem<T>(items: T[], from: number, to: number) {
  if (to < 0 || to >= items.length || from === to) return items
  const next = [...items]
  const [item] = next.splice(from, 1)
  next.splice(to, 0, item)
  return next
}

function safeParse(value: string): { ok: true; value: HTTPRouteTableConfig } | { ok: false; error: string } {
  try {
    return { ok: true, value: parseRouteTableConfig(value) }
  } catch (error) {
    return { ok: false, error: error instanceof Error ? error.message : String(error) }
  }
}
