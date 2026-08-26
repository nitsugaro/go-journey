import { Button } from '../../../components/Button';
import { TypeOptionPicker } from '../../../components/TypeOptionPicker';
import { journeyTypeOptions } from '../../../config/typeOptions';
import type { NewJourneyForm } from '../flowTypes';
import { normalizeJourneyType } from '../utils/schemaUtils';
import { JourneyFlag } from './JourneyFlag';
import { JourneyPropsDefinitionEditor } from './JourneyPropsDefinitionEditor';

export function NewJourneyPanel({
  realm,
  form,
  creating,
  onChange,
  onCancel,
  onCreate,
}: {
  realm: string;
  form: NewJourneyForm;
  creating: boolean;
  onChange: (updater: (current: NewJourneyForm) => NewJourneyForm) => void;
  onCancel: () => void;
  onCreate: () => void;
}) {
  return (
    <div className="motion-panel overflow-hidden rounded-3xl border border-[var(--color-blue-border)] bg-gradient-to-br from-[var(--color-surface)] via-[var(--color-surface)] to-[var(--color-surface-muted-transparent)] shadow-sm">
      <div className="grid gap-0 lg:grid-cols-[1fr_280px]">
        <div className="p-5">
          <div className="flex items-center gap-3">
            <span className="grid h-10 w-10 place-items-center rounded-2xl bg-[var(--color-blue)] text-lg font-bold text-[var(--color-white)] shadow-sm shadow-[var(--color-blue-border)]">
              +
            </span>
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.2em] text-[var(--color-blue)]">New journey</p>
              <h2 className="text-xl font-semibold tracking-tight">Create in {realm}</h2>
            </div>
          </div>

          <div className="mt-5 grid gap-3 lg:grid-cols-[1.1fr_0.5fr]">
            <label className="group text-xs font-semibold uppercase tracking-[0.12em] text-[var(--color-muted)]">
              Name
              <input
                value={form.name}
                onChange={(event) => onChange((current) => ({ ...current, name: event.target.value }))}
                className="mt-1 w-full rounded-2xl border border-transparent bg-[var(--color-surface)] px-4 py-3 text-base font-semibold normal-case tracking-normal text-[var(--color-ink)] shadow-sm outline-none ring-1 ring-[var(--color-blue-border)] transition focus:ring-2 focus:ring-[var(--color-blue)]"
                placeholder="login-flow"
              />
            </label>
            <label className="group text-xs font-semibold uppercase tracking-[0.12em] text-[var(--color-muted)]">
              Exp
              <input
                type="number"
                min={0}
                value={form.default_exp}
                onChange={(event) =>
                  onChange((current) => ({ ...current, default_exp: Number(event.target.value) || 0 }))
                }
                className="mt-1 w-full rounded-2xl border border-transparent bg-[var(--color-surface)] px-4 py-3 text-base font-semibold normal-case tracking-normal text-[var(--color-ink)] shadow-sm outline-none ring-1 ring-[var(--color-blue-border)] transition focus:ring-2 focus:ring-[var(--color-blue)]"
              />
            </label>
            <div className="lg:col-span-2">
              <TypeOptionPicker
                label="Type"
                value={normalizeJourneyType(form.journey_type)}
                options={journeyTypeOptions}
                onChange={(journeyType) => onChange((current) => ({ ...current, journey_type: journeyType }))}
                columns="three"
              />
            </div>
            <label className="text-xs font-semibold uppercase tracking-[0.12em] text-[var(--color-muted)] lg:col-span-2">
              Description
              <textarea
                value={form.description}
                onChange={(event) => onChange((current) => ({ ...current, description: event.target.value }))}
                className="mt-1 min-h-20 w-full resize-none rounded-2xl border border-transparent bg-[var(--color-surface)] px-4 py-3 text-sm normal-case tracking-normal text-[var(--color-ink)] shadow-sm outline-none ring-1 ring-[var(--color-blue-border)] transition focus:ring-2 focus:ring-[var(--color-blue)]"
                placeholder="Optional description"
              />
            </label>
          </div>
          <JourneyPropsDefinitionEditor props={form.props} onChange={(props) => onChange((current) => ({ ...current, props }))} />
        </div>

        <aside className="flex flex-col justify-between border-t border-[var(--color-blue-border)] bg-[var(--color-surface-translucent)] p-5 lg:border-l lg:border-t-0">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-[var(--color-muted-soft)]">Options</p>
            <div className="mt-3 grid gap-2">
              <JourneyFlag
                label="Active"
                checked={form.active}
                onChange={(active) => onChange((current) => ({ ...current, active }))}
              />
              <JourneyFlag
                label="Confidential"
                checked={form.confidential}
                onChange={(confidential) => onChange((current) => ({ ...current, confidential }))}
              />
              <JourneyFlag
                label="Encrypted inputs"
                checked={form.encrypted_client_inputs}
                onChange={(encryptedClientInputs) =>
                  onChange((current) => ({ ...current, encrypted_client_inputs: encryptedClientInputs }))
                }
              />
              <JourneyFlag
                label="Debug"
                checked={form.debug}
                onChange={(debug) => onChange((current) => ({ ...current, debug }))}
              />
            </div>
          </div>

          <div className="mt-5 flex gap-2">
            <Button onClick={onCancel} variant="ghost" className="flex-1">
              Cancel
            </Button>
            <Button disabled={creating || !form.name.trim()} onClick={onCreate} variant="primary" className="flex-1">
              {creating ? 'Creating…' : 'Create'}
            </Button>
          </div>
        </aside>
      </div>
    </div>
  );
}
