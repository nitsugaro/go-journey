import { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import { Button, IconButton } from '../../../components/Button';
import { CloseMiniIcon } from '../../../components/Icons';
import { TypeOptionPicker } from '../../../components/TypeOptionPicker';
import { journeyTypeOptions } from '../../../config/typeOptions';
import type { JourneyConfiguration } from '../../../types/journey';
import type { JourneyPropDefinition } from '../flowTypes';
import { cloneJourney } from '../utils/journeyRuntimeUtils';
import { cleanJourneyPropDefinitions, normalizeJourneyPropDefinitions } from '../utils/journeyStateUtils';
import { normalizeJourneyType } from '../utils/schemaUtils';
import { JourneyFlag } from './JourneyFlag';
import { JourneyPropsDefinitionEditor } from './JourneyPropsDefinitionEditor';

type JourneySettingsModalProps = {
  open: boolean;
  realm: string;
  journey: JourneyConfiguration | null;
  saving?: boolean;
  error?: string;
  confirmLabel?: string;
  onClose: () => void;
  onSave: (journey: JourneyConfiguration) => void;
};

export function JourneySettingsModal({
  open,
  realm,
  journey,
  saving = false,
  error = '',
  confirmLabel = 'Save settings',
  onClose,
  onSave,
}: JourneySettingsModalProps) {
  const [draft, setDraft] = useState<JourneyConfiguration | null>(journey ? cloneJourney(journey) : null);
  const [propsDraft, setPropsDraft] = useState<JourneyPropDefinition[]>(() => normalizeJourneyPropDefinitions(journey?.additional_properties?.props));

  useEffect(() => {
    if (open && journey) {
      setDraft(cloneJourney(journey));
      setPropsDraft(normalizeJourneyPropDefinitions(journey.additional_properties?.props));
    }
  }, [journey, open]);

  if (!open || !journey || !draft) return null;
  const canSave = Boolean(draft.name.trim()) && !saving;

  function update(patch: Partial<JourneyConfiguration>) {
    setDraft((current) => current ? { ...current, ...patch } : current);
  }

  function saveDraft() {
    const additional = { ...(draft?.additional_properties || {}) };
    const cleanProps = cleanJourneyPropDefinitions(propsDraft);
    if (cleanProps.length > 0) additional.props = cleanProps;
    else delete additional.props;
    onSave({ ...draft!, realm, additional_properties: Object.keys(additional).length > 0 ? additional : null });
  }

  return createPortal(
    <div className="motion-modal-backdrop fixed inset-0 z-[1200] flex items-center justify-center bg-[var(--color-overlay)] p-5" onMouseDown={(event) => { if (event.target === event.currentTarget && !saving) onClose(); }}>
      <section className="motion-modal-surface flex max-h-[92vh] w-[min(1080px,96vw)] flex-col overflow-hidden rounded-[2rem] border border-[var(--color-blue-border)] bg-[var(--color-surface)] shadow-2xl" onMouseDown={(event) => event.stopPropagation()}>
        <header className="flex shrink-0 items-start justify-between gap-4 border-b border-[var(--color-border-soft)] px-6 py-5">
          <div>
            <p className="text-xs font-bold uppercase tracking-[0.28em] text-[var(--color-blue)]">Journey settings</p>
            <h2 className="mt-2 text-2xl font-bold text-[var(--color-ink)]">{draft.name || 'Untitled journey'}</h2>
            <p className="mt-2 text-sm text-[var(--color-muted)]">Edit identity, runtime behavior, security flags and the sub-journey input contract.</p>
          </div>
          <IconButton disabled={saving} onClick={onClose} label="Close journey settings" variant="secondary" size="md"><CloseMiniIcon /></IconButton>
        </header>

        <div className="min-h-0 flex-1 overflow-y-auto px-6 py-5">
          {error && <div className="mb-5 rounded-2xl border border-[var(--color-red)] bg-[var(--color-red-soft)] px-4 py-3 text-sm text-[var(--color-red)]">{error}</div>}
          <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_280px]">
            <div className="grid gap-5">
              <section className="rounded-3xl border border-[var(--color-border-soft)] bg-[var(--color-surface-muted-transparent)] p-5">
                <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_180px]">
                  <SettingsInput label="Name" value={draft.name} onChange={(name) => update({ name })} placeholder="Journey name" />
                  <SettingsInput label="Default expiration" type="number" min={0} value={String(draft.default_exp ?? 1)} onChange={(value) => update({ default_exp: Number(value) || 0 })} />
                  <label className="grid gap-2 md:col-span-2">
                    <span className="text-xs font-bold uppercase tracking-[0.2em] text-[var(--color-muted-soft)]">Description</span>
                    <textarea value={draft.description || ''} onChange={(event) => update({ description: event.target.value })} placeholder="Optional description" className="min-h-24 resize-y rounded-2xl bg-[var(--color-surface-subtle)] px-4 py-3 text-sm text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:ring-2 focus:ring-[var(--color-blue)]" />
                  </label>
                  <div className="md:col-span-2">
                    <TypeOptionPicker label="Journey type" value={normalizeJourneyType(draft.journey_type)} options={journeyTypeOptions} columns="three" onChange={(journey_type) => update({ journey_type })} />
                    <p className="mt-2 text-xs text-[var(--color-muted-soft)]">Changing the type can make existing steps incompatible; backend validation runs when the journey is saved.</p>
                  </div>
                </div>
              </section>

              <section className="rounded-3xl border border-[var(--color-border-soft)] bg-[var(--color-surface-muted-transparent)] p-5">
                <JourneyPropsDefinitionEditor props={propsDraft} onChange={setPropsDraft} />
              </section>
            </div>

            <aside className="h-fit rounded-3xl border border-[var(--color-blue-border)] bg-[var(--color-blue-soft)] p-5">
              <p className="text-xs font-bold uppercase tracking-[0.22em] text-[var(--color-blue)]">Runtime options</p>
              <div className="mt-4 grid gap-2">
                <JourneyFlag label="Active" checked={Boolean(draft.active)} onChange={(active) => update({ active })} />
                <JourneyFlag label="Debug" checked={Boolean(draft.debug)} onChange={(debug) => update({ debug })} />
                <JourneyFlag label="Confidential" checked={Boolean(draft.confidential)} onChange={(confidential) => update({ confidential })} />
                <JourneyFlag label="Encrypted inputs" checked={Boolean(draft.encrypted_client_inputs)} onChange={(encrypted_client_inputs) => update({ encrypted_client_inputs })} />
              </div>
              <div className="mt-5 rounded-2xl bg-[var(--color-surface-translucent)] px-4 py-3 ring-1 ring-[var(--color-blue-border)]">
                <p className="text-xs font-bold uppercase tracking-[0.18em] text-[var(--color-blue)]">Realm</p>
                <p className="mt-1 truncate font-mono text-sm font-bold text-[var(--color-ink)]">{realm}</p>
              </div>
              {draft.id && <p className="mt-4 break-all font-mono text-xs text-[var(--color-muted-soft)]">{draft.id}</p>}
            </aside>
          </div>
        </div>

        <footer className="flex shrink-0 justify-end gap-3 border-t border-[var(--color-border-soft)] px-6 py-5">
          <Button disabled={saving} variant="ghost" onClick={onClose}>Cancel</Button>
          <Button disabled={!canSave} variant="primary" onClick={saveDraft}>{saving ? 'Saving…' : confirmLabel}</Button>
        </footer>
      </section>
    </div>,
    document.body,
  );
}

function SettingsInput({ label, value, onChange, placeholder, type = 'text', min }: { label: string; value: string; onChange: (value: string) => void; placeholder?: string; type?: string; min?: number }) {
  return (
    <label className="grid gap-2">
      <span className="text-xs font-bold uppercase tracking-[0.2em] text-[var(--color-muted-soft)]">{label}</span>
      <input type={type} min={min} value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} className="rounded-2xl bg-[var(--color-surface-subtle)] px-4 py-3 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:ring-2 focus:ring-[var(--color-blue)]" />
    </label>
  );
}
