import { Button, IconButton } from '../../../components/Button';
import { CopyIcon, DeleteIcon, ExportIcon } from '../../../components/Icons';
import type { JourneyConfiguration } from '../../../types/journey';

type JourneyCanvasHeaderProps = {
  realm: string;
  journeyId: string;
  journey: JourneyConfiguration | null;
  dirty: boolean;
  saving: boolean;
  journeySettingsOpen: boolean;
  onBack: () => void;
  onToggleSettings: () => void;
  onDuplicate: () => void;
  onExport: () => void;
  onSave: () => void;
  onDelete: () => void;
};

export function JourneyCanvasHeader({
  realm,
  journeyId,
  journey,
  dirty,
  saving,
  journeySettingsOpen,
  onBack,
  onToggleSettings,
  onDuplicate,
  onExport,
  onSave,
  onDelete,
}: JourneyCanvasHeaderProps) {
  return (
    <div className="shrink-0 flex flex-col gap-4 rounded-3xl border border-[var(--color-border)] bg-[var(--color-surface)] p-4 shadow-sm lg:flex-row lg:items-end lg:justify-between">
      <div>
        <p className="text-sm font-semibold uppercase tracking-[0.2em] text-[var(--color-blue)]">Flow</p>
        <h1 className="mt-2 text-3xl font-semibold tracking-tight">Journey canvas</h1>
        <p className="mt-1 text-sm text-[var(--color-muted)]">
          Steps are placed left to right by execution order. Branches and sub-entries fall below the main path.
        </p>
      </div>
      <div className="flex flex-wrap items-end justify-end gap-3">
        <Button onClick={onBack} variant="secondary" size="lg" className="self-end">
          ← Journeys
        </Button>
        <InfoPill label="Realm" value={realm} blue />
        <InfoPill label="Journey" value={journey?.name || journeyId} />
        <Button disabled={!journey} onClick={onToggleSettings} variant="secondary" active={journeySettingsOpen} size="lg" className="self-end">
          Settings
        </Button>
        <IconButton disabled={!journey || saving} onClick={onDuplicate} label="Duplicate journey" variant="secondary" size="lg" className="self-end">
          <CopyIcon />
        </IconButton>
        <IconButton disabled={!journey} onClick={onExport} label="Export journey JSON" variant="secondary" size="lg" className="self-end">
          <ExportIcon />
        </IconButton>
        <IconButton disabled={!journey || saving} onClick={onDelete} label="Delete journey" variant="danger" size="lg" className="self-end">
          <DeleteIcon />
        </IconButton>
        <Button disabled={!journey || !dirty || saving} onClick={onSave} variant={dirty ? 'primary' : 'status'} size="lg" className="self-end">
          {saving ? 'Saving…' : dirty ? 'Save changes' : 'Saved'}
        </Button>
      </div>
    </div>
  );
}

function InfoPill({ label, value, blue }: { label: string; value: string; blue?: boolean }) {
  return (
    <div className="text-left text-xs font-semibold uppercase tracking-[0.14em] text-[var(--color-muted)]">
      {label}
      <div
        className={[
          'mt-1 max-w-72 truncate rounded-xl px-4 py-3 text-sm font-semibold normal-case tracking-normal',
          blue
            ? 'bg-[var(--color-blue-soft)] text-[var(--color-blue)]'
            : 'border border-[var(--color-border)] bg-[var(--color-surface)] text-[var(--color-ink)]',
        ].join(' ')}
      >
        {value}
      </div>
    </div>
  );
}
