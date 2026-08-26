import { Clock3, Play, Plus, Save, Trash2 } from 'lucide-react';
import type { ReactNode } from 'react';
import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  deleteSchedule,
  getJourney,
  getSchedule,
  getScript,
  listSchedules,
  saveSchedule,
  triggerSchedule,
} from '../../api/journeyApi';
import { IconButton } from '../../components/Button';
import { DeleteConfirmModal } from '../../components/DeleteConfirmModal';
import type { ScheduleConfiguration } from '../../types/journey';
import { BooleanToggle } from '../flow/components/BooleanToggle';
import { SelectableScalarField } from '../flow/components/SelectableScalarField';
import { ScriptArgsConfigField } from '../flow/components/ScriptArgsConfigField';
import { SubJourneyPropsField } from '../flow/components/SubJourneyPropsField';
import { normalizeJourneyPropDefinitions } from '../flow/utils/journeyStateUtils';

export function SchedulesPage() {
  const { realm = 'alpha', scheduleId = '' } = useParams();
  const navigate = useNavigate();
  const [items, setItems] = useState<ScheduleConfiguration[]>([]);
  const [draft, setDraft] = useState<ScheduleConfiguration>(() => newSchedule());
  const [selectedID, setSelectedID] = useState('');
  const [search, setSearch] = useState('');
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [lastRun, setLastRun] = useState('');
  const [deleteTargetSchedule, setDeleteTargetSchedule] = useState<ScheduleConfiguration | null>(null);

  const filtered = useMemo(() => {
    const query = search.trim().toLowerCase();
    if (!query) return items;
    return items.filter((item) =>
      `${item.name} ${item.description || ''} ${item.target?.type || ''} ${item.id || ''}`.toLowerCase().includes(query)
    );
  }, [items, search]);
  const editingSchedule = Boolean(selectedID);

  useEffect(() => {
    void refresh();
  }, [realm]);
  useEffect(() => {
    if (scheduleId) void selectSchedule(scheduleId, false);
    else startCreate(false);
  }, [realm, scheduleId]);

  async function refresh() {
    setLoading(true);
    setError('');
    try {
      const response = await listSchedules(realm, { limit: 200 });
      setItems(response.result || []);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }

  async function selectSchedule(id: string, shouldNavigate = true) {
    if (!id) return;
    if (shouldNavigate) {
      navigate(`/${encodeURIComponent(realm)}/schedules/${encodeURIComponent(id)}`);
      return;
    }
    setError('');
    setLastRun('');
    try {
      const loaded = await getSchedule(realm, id);
      setSelectedID(loaded.id || '');
      setDraft(normalizeSchedule(loaded));
      setDirty(false);
    } catch (err) {
      setError(errorMessage(err));
    }
  }

  function startCreate(markDirty = true) {
    if (scheduleId) navigate(`/${encodeURIComponent(realm)}/schedules`);
    setSelectedID('');
    setDraft(newSchedule());
    setDirty(markDirty);
    setError('');
    setLastRun('');
  }

  function update(next: Partial<ScheduleConfiguration>) {
    setDraft((current) => ({ ...current, ...next }));
    setDirty(true);
  }

  async function persist() {
    if (!draft.name.trim()) return setError('Schedule name is required.');
    setSaving(true);
    setError('');
    try {
      const saved = await saveSchedule(realm, normalizeSchedule({ ...draft, name: draft.name.trim(), realm }));
      setDraft(saved);
      setSelectedID(saved.id || '');
      setDirty(false);
      if (saved.id)
        navigate(`/${encodeURIComponent(realm)}/schedules/${encodeURIComponent(saved.id)}`, { replace: true });
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  async function confirmDeleteSchedule() {
    const target = deleteTargetSchedule;
    if (!target?.id) return;
    setSaving(true);
    setError('');
    try {
      await deleteSchedule(realm, target.id);
      setDeleteTargetSchedule(null);
      navigate(`/${encodeURIComponent(realm)}/schedules`, { replace: true });
      startCreate(false);
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  async function run() {
    if (!selectedID) return;
    setSaving(true);
    setError('');
    try {
      const result = await triggerSchedule(realm, selectedID, draft.trigger_wait ?? true);
      setLastRun(JSON.stringify(result, null, 2));
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <section className="flex h-full min-h-0 gap-4">
      <aside className="flex w-[min(420px,34vw)] shrink-0 flex-col overflow-hidden rounded-3xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-sm">
        <div className="border-b border-[var(--color-border-soft)] p-5">
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-xs font-bold uppercase tracking-[0.32em] text-[var(--color-blue)]">Schedules</p>
              <h1 className="mt-2 text-2xl font-bold tracking-tight">Scheduler</h1>
              <p className="mt-1 text-sm text-[var(--color-muted)]">
                Run scripts or workflow journeys by cron, interval, or manual trigger.
              </p>
            </div>
            <IconButton onClick={() => startCreate()} label="Create schedule" variant="primary" size="lg">
              <Plus size={18} />
            </IconButton>
          </div>
          <input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search by name, target or UUID..."
            className="mt-4 w-full rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-4 py-3 text-sm font-semibold outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]"
          />
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto p-4">
          {loading && <p className="text-sm text-[var(--color-muted)]">Loading schedules...</p>}
          {!loading &&
            filtered.map((item) => (
              <ScheduleCard
                key={item.id || item.name}
                item={item}
                selected={item.id === selectedID}
                onClick={() => selectSchedule(item.id || '')}
              />
            ))}
          {!loading && filtered.length === 0 && (
            <p className="rounded-2xl bg-[var(--color-surface-muted)] p-4 text-sm text-[var(--color-muted)]">
              No schedules found.
            </p>
          )}
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col overflow-hidden rounded-3xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-sm">
        <div className="flex shrink-0 flex-col gap-4 border-b border-[var(--color-border-soft)] p-5 xl:flex-row xl:items-start xl:justify-between">
          <div className="min-w-0 flex-1">
            <p className="text-xs font-bold uppercase tracking-[0.32em] text-[var(--color-blue)]">
              {editingSchedule ? 'Editing schedule' : 'New schedule'}
            </p>
            <div className="mt-2 flex flex-wrap items-center gap-3">
              <h2 className="truncate text-2xl font-bold text-[var(--color-ink)]">
                {editingSchedule ? draft.name || selectedID || 'Schedule' : 'Create schedule'}
              </h2>
              {editingSchedule && (
                <span className="rounded-full bg-[var(--color-surface-soft)] px-3 py-1 text-xs font-bold uppercase tracking-[0.14em] text-[var(--color-muted)] ring-1 ring-[var(--color-border-soft)]">
                  {draft.target?.type || 'target'}
                </span>
              )}
            </div>
            <p className="mt-2 max-w-3xl text-sm text-[var(--color-muted)]">
              {editingSchedule
                ? 'Edit this persisted scheduler job. Trigger and delete actions apply to this saved schedule only.'
                : 'Create a scheduler job, choose when it starts, and point it to a workflow journey or schedule script.'}
            </p>
            {editingSchedule && selectedID && (
              <p className="mt-2 font-mono text-xs text-[var(--color-muted-soft)]">{selectedID}</p>
            )}
          </div>
          <div className="flex items-center gap-2">
            {dirty && (
              <span className="rounded-full border border-[var(--color-warning-border)] bg-[var(--color-warning-soft)] px-3 py-1 text-xs font-bold uppercase tracking-[0.24em] text-[var(--color-warning)]">
                unsaved
              </span>
            )}
            <IconButton
              onClick={run}
              disabled={!selectedID || saving || !draft.trigger_enabled}
              label="Trigger now"
              variant="success"
            >
              <Play size={16} />
            </IconButton>
            <IconButton
              onClick={() => selectedID && setDeleteTargetSchedule({ ...draft, id: selectedID })}
              disabled={!selectedID || saving}
              label="Delete schedule"
              variant="danger"
            >
              <Trash2 size={16} />
            </IconButton>
            <IconButton onClick={persist} disabled={saving || !dirty} label="Save schedule" variant="primary">
              <Save size={16} />
            </IconButton>
          </div>
        </div>
        {error && (
          <div className="m-5 rounded-2xl border border-[var(--color-red-border)] bg-[var(--color-red-soft)] px-4 py-3 text-sm text-[var(--color-red)]">
            {error}
          </div>
        )}
        <div className="grid min-h-0 flex-1 gap-5 overflow-y-auto p-5 lg:grid-cols-[1fr_420px]">
          <ScheduleForm realm={realm} draft={draft} onChange={update} />
          <aside className="grid content-start gap-4">
            <InfoCard label="Status" value={draft.running ? 'running' : draft.status || 'idle'} />
            <InfoCard label="Run count" value={draft.run_count ?? 0} />
            <InfoCard label="Next run" value={formatTime(estimatedNextRun(draft))} />
            <InfoCard label="Starts in" value={timeUntil(estimatedNextRun(draft))} />
            <InfoCard label="Timezone" value={timezoneSummary(draft.timezone || localTimezone())} />
            <InfoCard label="Last error" value={draft.last_error || '—'} />
            {lastRun && (
              <pre className="max-h-72 overflow-auto rounded-2xl border border-[var(--color-border-soft)] bg-[var(--color-code-bg)] p-4 text-xs text-[var(--color-code-text)]">
                {lastRun}
              </pre>
            )}
          </aside>
        </div>
      </div>
      <DeleteConfirmModal
        open={Boolean(deleteTargetSchedule)}
        itemLabel="Schedule"
        itemName={deleteTargetSchedule?.name?.trim() || deleteTargetSchedule?.id || ''}
        confirming={saving}
        onCancel={() => setDeleteTargetSchedule(null)}
        onConfirm={confirmDeleteSchedule}
      />
    </section>
  );
}

function ScheduleForm({
  realm,
  draft,
  onChange,
}: {
  realm: string;
  draft: ScheduleConfiguration;
  onChange: (next: Partial<ScheduleConfiguration>) => void;
}) {
  const target = normalizeScheduleTarget(draft.target || { type: 'workflow' });
  const [jsonText, setJsonText] = useState(() =>
    JSON.stringify(target.type === 'script' ? target.args || {} : target.initial_data || {}, null, 2)
  );
  const [hasStructuredPayload, setHasStructuredPayload] = useState(false);

  useEffect(() => {
    setJsonText(JSON.stringify(target.type === 'script' ? target.args || {} : target.initial_data || {}, null, 2));
  }, [draft.id, target.type]);

  useEffect(() => {
    let cancelled = false;
    setHasStructuredPayload(false);
    if (target.type === 'script' && target.script_id) {
      getScript(realm, target.script_id)
        .then((script) => {
          if (!cancelled) setHasStructuredPayload(Array.isArray(script.args) && script.args.length > 0);
        })
        .catch(() => {
          if (!cancelled) setHasStructuredPayload(false);
        });
    } else if (target.type === 'workflow' && target.journey_id) {
      getJourney(realm, target.journey_id)
        .then((journey) => {
          if (!cancelled)
            setHasStructuredPayload(normalizeJourneyPropDefinitions(journey.additional_properties?.props).length > 0);
        })
        .catch(() => {
          if (!cancelled) setHasStructuredPayload(false);
        });
    }
    return () => {
      cancelled = true;
    };
  }, [realm, target.type, target.script_id, target.journey_id]);

  function updateTargetJSON(value: string) {
    setJsonText(value);
    try {
      const parsed = JSON.parse(value || '{}');
      onChange({ target: { ...target, ...(target.type === 'script' ? { args: parsed } : { initial_data: parsed }) } });
    } catch {
      // Keep the local editor text while the user is typing incomplete JSON.
    }
  }
  return (
    <div className="grid content-start gap-4">
      <Field label="Name">
        <input value={draft.name} onChange={(e) => onChange({ name: e.target.value })} className={inputClass} />
      </Field>
      <Field label="Description">
        <textarea
          value={draft.description || ''}
          onChange={(e) => onChange({ description: e.target.value })}
          className={`${inputClass} min-h-20`}
        />
      </Field>
      <div className="grid gap-4 md:grid-cols-2">
        <Field label="Kind">
          <select value={draft.kind} onChange={(e) => onChange({ kind: e.target.value })} className={inputClass}>
            <option value="interval">interval</option>
            <option value="cron">cron</option>
          </select>
        </Field>
        <Field label="Target">
          <select
            value={target.type}
            onChange={(e) => onChange({ target: nextTarget(e.target.value, target) })}
            className={inputClass}
          >
            <option value="workflow">workflow journey</option>
            <option value="script">schedule script</option>
          </select>
        </Field>
      </div>
      {draft.kind === 'cron' ? (
        <Field label="Cron">
          <input
            value={draft.cron || ''}
            onChange={(e) => onChange({ cron: e.target.value })}
            placeholder="0 3 * * *"
            className={inputClass}
          />
        </Field>
      ) : (
        <IntervalField
          value={draft.interval_seconds || 0}
          onChange={(value) => onChange({ interval_seconds: value })}
        />
      )}
      <StartAtField value={draft.start_at || 0} onChange={(value) => onChange({ start_at: value })} />
      <div className="grid gap-4 md:grid-cols-3">
        <Field label="Timezone">
          <select
            value={draft.timezone || localTimezone()}
            onChange={(e) => onChange({ timezone: e.target.value })}
            className={inputClass}
          >
            {timezoneOptions().map((timezone) => (
              <option key={timezone} value={timezone}>
                {timezone}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Max runs">
          <input
            type="number"
            value={draft.max_runs || ''}
            onChange={(e) => onChange({ max_runs: numberValue(e.target.value) })}
            placeholder="0 = unlimited"
            className={inputClass}
          />
        </Field>
        <Field
          label="Timeout seconds"
          description="Empty or 0 means no scheduler timeout; the job waits until the target finishes."
        >
          <input
            type="number"
            value={draft.timeout_seconds || ''}
            onChange={(e) => onChange({ timeout_seconds: numberValue(e.target.value) })}
            className={inputClass}
          />
        </Field>
      </div>
      <div className="grid gap-3 md:grid-cols-3">
        {(['active', 'trigger_enabled', 'trigger_wait'] as const).map((key) => (
          <Field key={key} label={key.replaceAll('_', ' ')}>
            <BooleanToggle value={Boolean(draft[key])} onChange={(value) => onChange({ [key]: value })} />
          </Field>
        ))}
      </div>
      {target.type === 'script' ? (
        <SelectableScalarField
          realm={realm}
          label="Schedule script"
          required
          description="Only scripts with type schedule can run directly from the scheduler."
          value={target.script_id || ''}
          source={{ resource: 'scripts', query: { type: 'schedule' }, nameProperty: 'name', valueProperty: 'id' }}
          onChange={(value) =>
            onChange({ target: { ...target, script_id: String(value || ''), script_type: 'schedule' } })
          }
        />
      ) : (
        <SelectableScalarField
          realm={realm}
          label="Workflow journey"
          required
          description="Only workflow journeys can run without an HTTP request."
          value={target.journey_id || ''}
          source={{ resource: 'journeys', query: { type: 'workflow' }, nameProperty: 'name', valueProperty: 'id' }}
          onChange={(value) => onChange({ target: { ...target, journey_id: String(value || '') } })}
        />
      )}
      <PayloadField
        realm={realm}
        target={target}
        hasStructuredPayload={hasStructuredPayload}
        jsonText={jsonText}
        onJSONChange={updateTargetJSON}
        onTargetChange={(nextTarget) => onChange({ target: nextTarget })}
      />
    </div>
  );
}

function IntervalField({ value, onChange }: { value: number; onChange: (value: number) => void }) {
  const presets = [
    { label: '1 min', value: 60 },
    { label: '5 min', value: 300 },
    { label: '15 min', value: 900 },
    { label: '30 min', value: 1800 },
    { label: 'Hourly', value: 3600 },
    { label: '6 hours', value: 21600 },
    { label: '12 hours', value: 43200 },
    { label: 'Daily', value: 86400 },
    { label: 'Weekly', value: 604800 },
    { label: '2 weeks', value: 1209600 },
    { label: 'Monthly', value: 2592000 },
    { label: 'Yearly', value: 31536000 },
  ];
  return (
    <Field
      label="Frequency"
      description="How often the schedule repeats after it starts. Manual seconds are always allowed."
    >
      <div className="grid gap-2">
        <div className="flex gap-2 overflow-x-auto pb-1 thin-scrollbar">
          {presets.map((preset) => (
            <button
              key={preset.value}
              type="button"
              onClick={() => onChange(preset.value)}
              className={[
                'rounded-full px-3 py-2 text-xs font-bold transition',
                value === preset.value
                  ? 'bg-[var(--color-blue)] text-[var(--color-on-blue)]'
                  : 'bg-[var(--color-surface-subtle)] text-[var(--color-muted)] ring-1 ring-[var(--color-border-soft)] hover:text-[var(--color-ink)]',
              ].join(' ')}
            >
              {preset.label}
            </button>
          ))}
        </div>
        <input
          type="number"
          value={value || ''}
          onChange={(e) => onChange(numberValue(e.target.value) || 0)}
          placeholder="Manual seconds"
          className={inputClass}
        />
      </div>
    </Field>
  );
}

function StartAtField({ value, onChange }: { value: number; onChange: (value: number) => void }) {
  const presets = [
    { label: 'Now', value: () => Math.floor(Date.now() / 1000) },
    { label: 'In 5 min', value: () => Math.floor(Date.now() / 1000) + 300 },
    { label: 'In 1 hour', value: () => Math.floor(Date.now() / 1000) + 3600 },
    { label: 'Tomorrow 09:00', value: tomorrowAtNine },
  ];
  return (
    <Field
      label="Starts at"
      description="Optional. Empty means the scheduler starts using the next period after saving."
    >
      <div className="grid gap-2">
        <div className="flex flex-wrap gap-2">
          {presets.map((preset) => (
            <button
              key={preset.label}
              type="button"
              onClick={() => onChange(preset.value())}
              className="rounded-full bg-[var(--color-surface-subtle)] px-3 py-2 text-xs font-bold text-[var(--color-muted)] ring-1 ring-[var(--color-border-soft)] transition hover:text-[var(--color-blue)]"
            >
              {preset.label}
            </button>
          ))}
          <button
            type="button"
            onClick={() => onChange(0)}
            className="rounded-full bg-[var(--color-surface-subtle)] px-3 py-2 text-xs font-bold text-[var(--color-muted)] ring-1 ring-[var(--color-border-soft)] transition hover:text-[var(--color-red)]"
          >
            Clear
          </button>
        </div>
        <input
          type="datetime-local"
          value={value ? dateTimeLocalValue(value) : ''}
          onChange={(event) =>
            onChange(event.target.value ? Math.floor(new Date(event.target.value).getTime() / 1000) : 0)
          }
          className={inputClass}
        />
      </div>
    </Field>
  );
}

function PayloadField({
  realm,
  target,
  hasStructuredPayload,
  jsonText,
  onJSONChange,
  onTargetChange,
}: {
  realm: string;
  target: ScheduleConfiguration['target'];
  hasStructuredPayload: boolean;
  jsonText: string;
  onJSONChange: (value: string) => void;
  onTargetChange: (target: ScheduleConfiguration['target']) => void;
}) {
  if (target.type === 'script' && target.script_id && hasStructuredPayload) {
    return (
      <ScriptArgsConfigField
        realm={realm}
        label="Args"
        scriptID={target.script_id}
        value={target.args || {}}
        onChange={(args) => onTargetChange({ ...target, args: args as Record<string, unknown> })}
      />
    );
  }
  if (target.type === 'workflow' && target.journey_id && hasStructuredPayload) {
    return (
      <SubJourneyPropsField
        realm={realm}
        label="Workflow props"
        journeyID={target.journey_id}
        value={target.initial_data || {}}
        onChange={(initialData) => onTargetChange({ ...target, initial_data: initialData as Record<string, unknown> })}
      />
    );
  }
  return (
    <Field label={target.type === 'script' ? 'Args JSON' : 'Initial data JSON'}>
      <textarea
        value={jsonText}
        onChange={(e) => onJSONChange(e.target.value)}
        className={`${inputClass} min-h-48 font-mono text-xs`}
      />
    </Field>
  );
}

function ScheduleCard({
  item,
  selected,
  onClick,
}: {
  item: ScheduleConfiguration;
  selected: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={[
        'mb-3 w-full rounded-3xl border p-4 text-left transition',
        selected
          ? 'border-[var(--color-blue)] bg-[var(--color-blue-subtle)]'
          : 'border-transparent bg-[var(--color-surface-muted)] hover:border-[var(--color-blue-border)]',
      ].join(' ')}
    >
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="font-bold">{item.name}</p>
          <p className="mt-1 text-xs uppercase tracking-[0.24em] text-[var(--color-muted)]">
            {item.target?.type || 'target'} · {item.kind}
          </p>
        </div>
        <Clock3 size={16} className="text-[var(--color-blue)]" />
      </div>
      <p className="mt-2 truncate font-mono text-xs text-[var(--color-muted)]">{item.id}</p>
    </button>
  );
}

function Field({ label, description, children }: { label: string; description?: string; children: ReactNode }) {
  return (
    <label className="grid gap-2">
      <span className="text-xs font-bold uppercase tracking-[0.32em] text-[var(--color-muted-soft)]">{label}</span>
      {children}
      {description && <span className="text-xs text-[var(--color-muted-soft)]">{description}</span>}
    </label>
  );
}

function InfoCard({ label, value }: { label: string; value: unknown }) {
  return (
    <div className="rounded-2xl border border-[var(--color-border-soft)] bg-[var(--color-surface-subtle)] p-4">
      <p className="text-xs font-bold uppercase tracking-[0.28em] text-[var(--color-muted-soft)]">{label}</p>
      <p className="mt-2 break-words text-sm font-semibold">{String(value)}</p>
    </div>
  );
}

function newSchedule(): ScheduleConfiguration {
  return {
    name: '',
    active: true,
    kind: 'interval',
    interval_seconds: 3600,
    start_at: 0,
    max_runs: 0,
    trigger_enabled: true,
    trigger_wait: true,
    timezone: localTimezone(),
    target: { type: 'workflow', initial_data: {} },
  };
}

function nextTarget(type: string, current: ScheduleConfiguration['target']): ScheduleConfiguration['target'] {
  if (type === 'script')
    return { type: 'script', script_id: current.script_id || '', script_type: 'schedule', args: current.args || {} };
  return { type: 'workflow', journey_id: current.journey_id || '', initial_data: current.initial_data || {} };
}

function normalizeSchedule(schedule: ScheduleConfiguration): ScheduleConfiguration {
  return {
    ...schedule,
    target: normalizeScheduleTarget(schedule.target),
    timezone: schedule.timezone || localTimezone(),
    start_at: schedule.start_at || 0,
  };
}

function normalizeScheduleTarget(target: ScheduleConfiguration['target']): ScheduleConfiguration['target'] {
  if (!target) return { type: 'workflow', initial_data: {} };
  if (String(target.type).toLowerCase() === 'workflow-journey') return { ...target, type: 'workflow' };
  return target;
}

function timezoneOptions() {
  const intl = Intl as typeof Intl & { supportedValuesOf?: (key: string) => string[] };
  if (typeof intl.supportedValuesOf === 'function') return intl.supportedValuesOf('timeZone');
  return ['UTC', 'America/Argentina/Buenos_Aires', 'America/New_York', 'America/Los_Angeles', 'Europe/Madrid'];
}

function localTimezone() {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
}

const inputClass =
  'w-full rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-4 py-3 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]';
const numberValue = (value: string) => (value === '' ? undefined : Number(value));
const formatTime = (value?: number) => (value ? new Date(value * 1000).toLocaleString() : '—');
const errorMessage = (error: unknown) => (error instanceof Error ? error.message : 'Unexpected error');

function estimatedNextRun(schedule: ScheduleConfiguration) {
  if (schedule.next_run_at) return schedule.next_run_at;
  const now = Math.floor(Date.now() / 1000);
  if (schedule.kind === 'interval' && schedule.interval_seconds && schedule.active !== false) {
    return nextIntervalRun(schedule.start_at || 0, schedule.interval_seconds, now);
  }
  if (schedule.start_at && schedule.active !== false) return schedule.start_at;
  return undefined;
}

function nextIntervalRun(startAt: number, intervalSeconds: number, now: number) {
  if (!startAt) return now + intervalSeconds;
  if (startAt > now) return startAt;
  const elapsed = now - startAt;
  const steps = Math.floor(elapsed / intervalSeconds) + 1;
  return startAt + steps * intervalSeconds;
}

function dateTimeLocalValue(epochSeconds: number) {
  const date = new Date(epochSeconds * 1000);
  const offset = date.getTimezoneOffset();
  return new Date(date.getTime() - offset * 60000).toISOString().slice(0, 16);
}

function tomorrowAtNine() {
  const date = new Date();
  date.setDate(date.getDate() + 1);
  date.setHours(9, 0, 0, 0);
  return Math.floor(date.getTime() / 1000);
}

function timeUntil(epochSeconds?: number) {
  if (!epochSeconds) return '—';
  const seconds = Math.max(0, epochSeconds - Math.floor(Date.now() / 1000));
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  if (minutes > 0) return `${minutes}m`;
  return `${seconds}s`;
}

function timezoneSummary(timezone: string) {
  const selected = timezoneOffsetMinutes(timezone);
  const local = timezoneOffsetMinutes(localTimezone());
  if (selected === null || local === null) return timezone;
  const diff = selected - local;
  const relation = diff === 0 ? 'same as local' : `${diff > 0 ? '+' : ''}${diff / 60}h vs local`;
  return `${timezone} · UTC${formatOffset(selected)} · ${relation}`;
}

function timezoneOffsetMinutes(timezone: string) {
  try {
    const part =
      new Intl.DateTimeFormat('en-US', { timeZone: timezone, timeZoneName: 'shortOffset' })
        .formatToParts(new Date())
        .find((item) => item.type === 'timeZoneName')?.value || 'GMT';
    if (part === 'GMT' || part === 'UTC') return 0;
    const match = part.match(/GMT([+-])(\d{1,2})(?::?(\d{2}))?/);
    if (!match) return null;
    const sign = match[1] === '-' ? -1 : 1;
    return sign * (Number(match[2]) * 60 + Number(match[3] || 0));
  } catch {
    return null;
  }
}

function formatOffset(minutes: number) {
  const sign = minutes < 0 ? '-' : '+';
  const absolute = Math.abs(minutes);
  return `${sign}${String(Math.floor(absolute / 60)).padStart(2, '0')}:${String(absolute % 60).padStart(2, '0')}`;
}
