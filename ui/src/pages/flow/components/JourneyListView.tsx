import { useRef } from 'react';
import { Button, IconButton } from '../../../components/Button';
import { CopyIcon, DeleteIcon, EditMiniIcon, ExportIcon, ImportIcon } from '../../../components/Icons';
import type { JourneyConfiguration } from '../../../types/journey';
import type { JourneyFilters, NewJourneyForm } from '../flowTypes';
import { normalizeJourneyType } from '../utils/schemaUtils';
import { JourneyBadge } from './JourneyBadge';
import { NewJourneyPanel } from './NewJourneyPanel';

type JourneyListViewProps = {
  realm: string;
  journeys: JourneyConfiguration[];
  filteredJourneys: JourneyConfiguration[];
  journeySearch: string;
  filters: JourneyFilters;
  createOpen: boolean;
  newJourney: NewJourneyForm;
  creating: boolean;
  loading: boolean;
  error: string;
  onSearchChange: (value: string) => void;
  onFiltersChange: (filters: JourneyFilters) => void;
  onToggleCreate: () => void;
  onNewJourneyChange: (updater: (current: NewJourneyForm) => NewJourneyForm) => void;
  onCancelCreate: () => void;
  onCreateJourney: () => void;
  onOpenJourney: (journey: JourneyConfiguration) => void;
  onEditJourney: (journey: JourneyConfiguration) => void;
  onDuplicateJourney: (journey: JourneyConfiguration) => void;
  onExportJourney: (journey: JourneyConfiguration) => void;
  onImportJourney: (file: File) => void;
  onDeleteJourney: (journey: JourneyConfiguration) => void;
};

export function JourneyListView({
  realm,
  journeys,
  filteredJourneys,
  journeySearch,
  filters,
  createOpen,
  newJourney,
  creating,
  loading,
  error,
  onSearchChange,
  onFiltersChange,
  onToggleCreate,
  onNewJourneyChange,
  onCancelCreate,
  onCreateJourney,
  onOpenJourney,
  onEditJourney,
  onDuplicateJourney,
  onExportJourney,
  onImportJourney,
  onDeleteJourney,
}: JourneyListViewProps) {
  const importInputRef = useRef<HTMLInputElement>(null);
  return (
    <section className="flex h-full min-h-0 flex-col gap-4 overflow-hidden">
      <div className="shrink-0 rounded-3xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <p className="text-sm font-semibold uppercase tracking-[0.2em] text-[var(--color-blue)]">Flow</p>
            <h1 className="mt-2 text-3xl font-semibold tracking-tight">Journeys</h1>
            <p className="mt-1 text-sm text-[var(--color-muted)]">
              Search journeys in realm <span className="font-semibold text-[var(--color-blue)]">{realm}</span>, then
              open one to render its canvas.
            </p>
          </div>
          <div className="flex flex-wrap justify-end gap-3">
            <input
              ref={importInputRef}
              type="file"
              accept="application/json,.json"
              className="hidden"
              onChange={(event) => {
                const file = event.target.files?.[0];
                event.target.value = '';
                if (file) onImportJourney(file);
              }}
            />
            <Button onClick={() => importInputRef.current?.click()} variant="secondary" size="lg">
              <ImportIcon /> Import JSON
            </Button>
            <Button onClick={onToggleCreate} variant="primary" size="lg">
              + New journey
            </Button>
          </div>
        </div>
      </div>

      {createOpen && (
        <NewJourneyPanel
          realm={realm}
          form={newJourney}
          creating={creating}
          onChange={onNewJourneyChange}
          onCancel={onCancelCreate}
          onCreate={onCreateJourney}
        />
      )}

      {error && <ErrorMessage message={error} />}

      <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-3xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm">
        <div className="shrink-0 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <label className="min-w-0 flex-1 text-xs font-semibold uppercase tracking-[0.14em] text-[var(--color-muted)]">
            Search
            <input
              value={journeySearch}
              onChange={(event) => onSearchChange(event.target.value)}
              placeholder="Search by name, description or UUID..."
              className="mt-1 w-full rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-4 py-3 text-sm font-semibold normal-case tracking-normal text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]"
            />
          </label>
          <div className="rounded-xl bg-[var(--color-surface-subtle)] px-4 py-2 text-sm font-semibold text-[var(--color-muted)] ring-1 ring-[var(--color-border-soft)]">
            {filteredJourneys.length} / {journeys.length}
          </div>
        </div>

        <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
          <FilterSelect label="Journey type" value={filters.journeyType} onChange={(journeyType) => onFiltersChange({ ...filters, journeyType })}>
            <option value="">All journey types</option>
            <option value="auth">Auth</option>
            <option value="resource">Resource</option>
            <option value="workflow">Workflow</option>
          </FilterSelect>
          <FilterSelect label="Status" value={filters.active} onChange={(active) => onFiltersChange({ ...filters, active: active as JourneyFilters['active'] })}>
            <option value="all">Any status</option>
            <option value="yes">Active</option>
            <option value="no">Inactive</option>
          </FilterSelect>
          <FilterSelect label="Debug" value={filters.debug} onChange={(debug) => onFiltersChange({ ...filters, debug: debug as JourneyFilters['debug'] })}>
            <option value="all">Debug on or off</option>
            <option value="yes">Debug enabled</option>
            <option value="no">Debug disabled</option>
          </FilterSelect>
          <FilterSelect label="Confidential" value={filters.confidential} onChange={(confidential) => onFiltersChange({ ...filters, confidential: confidential as JourneyFilters['confidential'] })}>
            <option value="all">Any visibility</option>
            <option value="yes">Confidential</option>
            <option value="no">Not confidential</option>
          </FilterSelect>
          <FilterSelect label="Encrypted inputs" value={filters.encryptedInputs} onChange={(encryptedInputs) => onFiltersChange({ ...filters, encryptedInputs: encryptedInputs as JourneyFilters['encryptedInputs'] })}>
            <option value="all">Any input mode</option>
            <option value="yes">Encrypted</option>
            <option value="no">Not encrypted</option>
          </FilterSelect>
        </div>

        {(journeySearch.trim() || hasJourneyFilters(filters)) && (
          <div className="mt-3 flex justify-end">
            <Button size="sm" variant="ghost" onClick={() => {
              onSearchChange('');
              onFiltersChange(defaultJourneyFilters());
            }}>
              Clear filters
            </Button>
          </div>
        )}

        <div className="mt-5 min-h-0 flex-1 overflow-y-auto pr-1">
          <div className="grid gap-3 pb-8">
            {loading && <ListMessage message="Loading journeys…" />}
            {!loading && filteredJourneys.length === 0 && <ListMessage message="No journeys found." />}
            {filteredJourneys.map((item) => (
              <JourneyRow
                key={item.id || item.name}
                journey={item}
                onOpen={() => onOpenJourney(item)}
                onEdit={() => onEditJourney(item)}
                onDuplicate={() => onDuplicateJourney(item)}
                onExport={() => onExportJourney(item)}
                onDelete={() => onDeleteJourney(item)}
              />
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}

export function ErrorMessage({ message }: { message: string }) {
  return (
    <div className="rounded-2xl border border-[var(--color-red)] bg-[var(--color-red-soft)] px-4 py-3 text-sm text-[var(--color-red)]">
      {message}
    </div>
  );
}

function ListMessage({ message }: { message: string }) {
  return <p className="rounded-2xl bg-[var(--color-surface-subtle)] px-4 py-3 text-sm text-[var(--color-muted)]">{message}</p>;
}

function JourneyRow({
  journey,
  onOpen,
  onEdit,
  onDuplicate,
  onExport,
  onDelete,
}: {
  journey: JourneyConfiguration;
  onOpen: () => void;
  onEdit: () => void;
  onDuplicate: () => void;
  onExport: () => void;
  onDelete: () => void;
}) {
  return (
    <div
      role="button"
      tabIndex={0}
      onClick={onOpen}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          onOpen();
        }
      }}
      className="group cursor-pointer rounded-3xl border border-[var(--color-border-soft)] bg-[var(--color-surface-muted-transparent)] p-4 text-left transition hover:border-[var(--color-blue-border)] hover:shadow-sm"
    >
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <p className="text-lg font-semibold text-[var(--color-ink)]">{journey.name || 'Untitled journey'}</p>
          {journey.description && <p className="mt-1 text-sm text-[var(--color-muted)]">{journey.description}</p>}
          <p className="mt-2 font-mono text-xs text-[var(--color-muted-soft)]">{journey.id}</p>
        </div>
        <div className="flex shrink-0 flex-wrap items-center justify-end gap-2">
          <JourneyBadge label={normalizeJourneyType(journey.journey_type)} tone="blue" />
          <JourneyBadge label={journey.active ? 'active' : 'inactive'} tone={journey.active ? 'green' : 'slate'} />
          {journey.confidential && <JourneyBadge label="confidential" tone="blue" />}
          {journey.encrypted_client_inputs && <JourneyBadge label="encrypted inputs" tone="blue" />}
          {journey.debug && <JourneyBadge label="debug" tone="amber" />}
          <IconButton
            onClick={(event) => {
              event.stopPropagation();
              onEdit();
            }}
            label="Edit journey settings"
            variant="secondary"
            size="sm"
          >
            <EditMiniIcon />
          </IconButton>
          <IconButton
            onClick={(event) => {
              event.stopPropagation();
              onDuplicate();
            }}
            label="Duplicate journey"
            variant="secondary"
            size="sm"
          >
            <CopyIcon />
          </IconButton>
          <IconButton
            onClick={(event) => {
              event.stopPropagation();
              onExport();
            }}
            label="Export journey JSON"
            variant="secondary"
            size="sm"
          >
            <ExportIcon />
          </IconButton>
          <IconButton
            onClick={(event) => {
              event.stopPropagation();
              onDelete();
            }}
            label="Delete journey"
            variant="danger"
            size="sm"
          >
            <DeleteIcon />
          </IconButton>
        </div>
      </div>
    </div>
  );
}

function FilterSelect({ label, value, onChange, children }: { label: string; value: string; onChange: (value: string) => void; children: React.ReactNode }) {
  return (
    <label className="text-xs font-semibold uppercase tracking-[0.12em] text-[var(--color-muted-soft)]">
      {label}
      <select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 w-full rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-4 py-3 text-sm font-semibold normal-case tracking-normal text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]"
      >
        {children}
      </select>
    </label>
  );
}

function hasJourneyFilters(filters: JourneyFilters) {
  return Boolean(filters.journeyType) || filters.active !== 'all' || filters.debug !== 'all' || filters.confidential !== 'all' || filters.encryptedInputs !== 'all';
}

function defaultJourneyFilters(): JourneyFilters {
  return { journeyType: '', active: 'all', debug: 'all', confidential: 'all', encryptedInputs: 'all' };
}
