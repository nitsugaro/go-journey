import { useEffect, useState } from 'react';
import { Button, IconButton } from '../../../components/Button';
import type { CacheInfo } from '../../../types/journey';
import { defaultConfigText, formatBytes, isGeneratedTemplate } from '../cacheUtils';
import { CacheJsonEditor } from './CacheJsonEditor';
import { CacheSchemaForm } from './CacheSchemaForm';
import { HTTPRouteTableForm } from './HTTPRouteTableForm';
import { ChangesIcon, DeleteIcon, SaveIcon, SavedIcon, SpinnerIcon } from '../../../components/Icons';

type CacheEditorPanelProps = {
  realm: string;
  caches: CacheInfo[];
  selectedCache?: CacheInfo;
  selectedID: string;
  draftCacheKey: string;
  draftInstanceID: string;
  configText: string;
  dirty: boolean;
  saving: boolean;
  error: string;
  isNew: boolean;
  onDraftCacheKey: (key: string) => void;
  onDraftInstanceID: (id: string) => void;
  onConfigText: (value: string) => void;
  onDirty: () => void;
  onDelete: () => void;
  onSave: () => void;
};

export function CacheEditorPanel(props: CacheEditorPanelProps) {
  const [mode, setMode] = useState<'form' | 'json'>(props.selectedCache?.schema ? 'form' : 'json');
  const canUseForm = Boolean(props.selectedCache?.schema) || props.selectedCache?.key === 'http_route_table';

  useEffect(() => {
    setMode(props.selectedCache?.schema || props.selectedCache?.key === 'http_route_table' ? 'form' : 'json');
  }, [props.selectedCache?.key, props.selectedCache?.schema]);

  return (
    <main className="flex min-w-0 flex-1 flex-col overflow-hidden rounded-3xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-sm">
      <CacheEditorHeader {...props} />
      {props.error && <div className="mx-4 mt-4 rounded-2xl border border-[var(--color-red)] bg-[var(--color-red-soft)] px-4 py-3 text-sm text-[var(--color-red)]">{props.error}</div>}
      <div className="flex shrink-0 flex-col gap-3 border-b border-[var(--color-border-soft)] px-4 py-3 xl:flex-row xl:items-center xl:justify-between">
        <div className="min-w-0">
          <p className="text-xs font-bold uppercase tracking-[0.22em] text-[var(--color-muted-soft)]">Configuration</p>
          <p className="mt-1 truncate text-xs text-[var(--color-muted)]">{props.selectedCache?.description || 'Use the generated form for common edits, or raw JSON for advanced configuration.'}</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <SummaryChip label="instances" value={props.selectedCache?.instances ?? 0} />
          <SummaryChip label="size" value={formatBytes(props.selectedCache?.size_bytes || 0)} />
          <SummaryChip label="max" value={props.selectedCache?.max_instances || '∞'} />
          <SummaryChip label="limit" value={props.selectedCache?.max_size_bytes ? formatBytes(props.selectedCache.max_size_bytes) : '∞'} />
          <div className="ml-0 inline-flex rounded-2xl bg-[var(--color-surface-subtle)] p-1 ring-1 ring-[var(--color-border-soft)] xl:ml-2">
            <Button size="sm" variant="ghost" active={mode === 'form'} disabled={!canUseForm} onClick={() => setMode('form')}>
              Form
            </Button>
            <Button size="sm" variant="ghost" active={mode === 'json'} onClick={() => setMode('json')}>
              JSON
            </Button>
          </div>
        </div>
      </div>
      <div className="min-h-0 flex-1 overflow-auto p-4">
        {mode === 'form' ? (
          props.selectedCache?.key === 'http_route_table' ? (
            <HTTPRouteTableForm configText={props.configText} onChange={props.onConfigText} />
          ) : (
            <CacheSchemaForm realm={props.realm} cache={props.selectedCache} configText={props.configText} onChange={props.onConfigText} />
          )
        ) : (
          <CacheJsonEditor value={props.configText} onChange={props.onConfigText} />
        )}
      </div>
    </main>
  );
}

function CacheEditorHeader(props: CacheEditorPanelProps) {
  const modeLabel = props.isNew ? 'New instance' : 'Persisted instance';
  const title = props.isNew ? 'Create runtime dependency' : props.draftInstanceID || props.selectedID || 'Runtime dependency';

  return (
    <header className="flex shrink-0 flex-col gap-4 border-b border-[var(--color-border-soft)] p-4 xl:flex-row xl:items-start xl:justify-between">
      <div className="min-w-0 flex-1">
        <p className="text-xs font-bold uppercase tracking-[0.32em] text-[var(--color-blue)]">{modeLabel}</p>
        <div className="mt-2 flex flex-wrap items-center gap-3">
          <h1 className="truncate text-2xl font-bold text-[var(--color-ink)]">{title}</h1>
          {props.selectedCache?.key && <span className="rounded-full bg-[var(--color-blue-subtle)] px-3 py-1 font-mono text-xs font-bold text-[var(--color-blue)] ring-1 ring-[var(--color-blue-border)]">{props.selectedCache.key}</span>}
          {!props.isNew && <span className="rounded-full bg-[var(--color-green-soft)] px-3 py-1 text-xs font-bold uppercase tracking-[0.14em] text-[var(--color-green)] ring-1 ring-[var(--color-green-border)]">persisted</span>}
        </div>
        <p className="mt-1 max-w-2xl text-sm text-[var(--color-muted)]">
          {props.isNew ? 'Choose an instance type, give it a stable id, then configure the constructor below.' : 'Identity is read-only. Edit only constructor configuration or delete this persisted instance.'}
        </p>
        {props.isNew && (
          <div className="mt-4 grid gap-3 md:grid-cols-[260px_1fr]">
            <label className="grid gap-1">
              <span className="text-xs font-bold uppercase tracking-[0.24em] text-[var(--color-muted-soft)]">Instance type</span>
              <select value={props.draftCacheKey} onChange={(event) => updateCacheKey(event.target.value, props)} className="rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-4 py-3 text-base font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]">
                <option value="">Select instance type</option>
                {props.caches.map((cache) => <option key={cache.key} value={cache.key}>{cache.key}</option>)}
              </select>
            </label>
            <label className="grid gap-1">
              <span className="text-xs font-bold uppercase tracking-[0.24em] text-[var(--color-muted-soft)]">Instance id</span>
              <input value={props.draftInstanceID} onChange={(event) => { props.onDraftInstanceID(event.target.value); props.onDirty(); }} placeholder="default, analytics, ldap-main..." className="rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-4 py-3 text-base font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]" />
            </label>
          </div>
        )}
        {!props.isNew && (
          <div className="mt-3 flex flex-wrap gap-2">
            <IdentityPill label="type" value={props.selectedCache?.key || props.draftCacheKey || '—'} />
            <IdentityPill label="id" value={props.draftInstanceID || props.selectedID || '—'} />
          </div>
        )}
      </div>
      <div className="flex shrink-0 items-center justify-end gap-2">
        <span title={props.dirty ? 'Unsaved changes' : 'Saved'} className={['inline-flex h-9 w-9 items-center justify-center rounded-xl border', props.dirty ? 'border-[var(--color-warning-border)] bg-[var(--color-warning-soft)] text-[var(--color-warning)]' : 'border-[var(--color-border-soft)] bg-[var(--color-surface-soft)] text-[var(--color-muted)]'].join(' ')}>{props.dirty ? <ChangesIcon /> : <SavedIcon />}</span>
        <IconButton onClick={props.onDelete} disabled={!props.selectedID || props.saving} label="Delete instance" variant="danger" size="md"><DeleteIcon /></IconButton>
        <IconButton onClick={props.onSave} disabled={props.saving || !props.dirty || !isUserConfigurableCache(props.selectedCache)} label="Save instance" variant="primary" size="md">{props.saving ? <SpinnerIcon /> : <SaveIcon />}</IconButton>
      </div>
    </header>
  );
}

function isUserConfigurableCache(cache?: CacheInfo) {
  return cache?.user_configurable === true;
}

function IdentityPill({ label, value }: { label: string; value: string }) {
  return (
    <span className="inline-flex max-w-full items-center gap-2 rounded-full border border-[var(--color-border-soft)] bg-[var(--color-surface-subtle)] px-3 py-1.5 text-xs">
      <span className="font-bold uppercase tracking-[0.18em] text-[var(--color-muted-soft)]">{label}</span>
      <span className="truncate font-mono font-bold text-[var(--color-ink)]">{value}</span>
    </span>
  );
}

function SummaryChip({ label, value }: { label: string; value: string | number }) {
  return (
    <span className="inline-flex items-center gap-2 rounded-full border border-[var(--color-border-soft)] bg-[var(--color-surface-subtle)] px-3 py-1.5">
      <span className="text-[10px] font-bold uppercase tracking-[0.18em] text-[var(--color-muted-soft)]">{label}</span>
      <span className="text-xs font-bold text-[var(--color-ink)]">{value}</span>
    </span>
  );
}

function updateCacheKey(nextCacheKey: string, props: CacheEditorPanelProps) {
  const shouldSwapTemplate = props.isNew && isGeneratedTemplate(props.configText, props.caches);
  const nextCache = props.caches.find((cache) => cache.key === nextCacheKey);
  props.onDraftCacheKey(nextCacheKey);
  if (shouldSwapTemplate) props.onConfigText(defaultConfigText(nextCache));
  props.onDirty();
}
