import { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import { Button, IconButton } from '../../../components/Button';
import { CloseMiniIcon, CopyIcon, ImportIcon } from '../../../components/Icons';

type JourneyCreateCopyModalProps = {
  open: boolean;
  mode: 'duplicate' | 'import';
  realm: string;
  initialName: string;
  initialDescription: string;
  saving: boolean;
  error?: string;
  onCancel: () => void;
  onConfirm: (name: string, description: string) => void;
};

export function JourneyCreateCopyModal({
  open,
  mode,
  realm,
  initialName,
  initialDescription,
  saving,
  error,
  onCancel,
  onConfirm,
}: JourneyCreateCopyModalProps) {
  const [name, setName] = useState(initialName);
  const [description, setDescription] = useState(initialDescription);
  const [closing, setClosing] = useState(false);

  useEffect(() => {
    if (!open) return;
    setName(initialName);
    setDescription(initialDescription);
    setClosing(false);
  }, [initialDescription, initialName, open]);

  if (!open) return null;

  const importing = mode === 'import';
  const title = importing ? 'Import journey' : 'Duplicate journey';
  const canConfirm = Boolean(name.trim()) && !saving;

  function requestClose() {
    if (closing || saving) return;
    setClosing(true);
    window.setTimeout(onCancel, 140);
  }

  return createPortal(
    <div
      className="motion-modal-backdrop fixed inset-0 z-[1200] flex items-center justify-center bg-[var(--color-overlay)] p-5"
      data-closing={closing ? 'true' : undefined}
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) requestClose();
      }}
    >
      <section
        className="motion-modal-surface w-[min(620px,94vw)] overflow-hidden rounded-[2rem] border border-[var(--color-blue-border)] bg-[var(--color-surface)] shadow-2xl"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <header className="flex items-start justify-between gap-4 border-b border-[var(--color-border-soft)] px-6 py-5">
          <div>
            <p className="text-xs font-bold uppercase tracking-[0.28em] text-[var(--color-blue)]">
              {importing ? 'Portable JSON' : 'Journey copy'}
            </p>
            <h2 className="mt-2 text-2xl font-bold text-[var(--color-ink)]">{title}</h2>
            <p className="mt-2 text-sm text-[var(--color-muted)]">
              {importing
                ? 'The imported flow keeps its steps and references, receives a new journey UUID, and uses the current UI realm.'
                : 'The complete flow is copied unchanged. Only the new journey identity, name, and optional description differ.'}
            </p>
          </div>
          <IconButton onClick={requestClose} label={`Cancel ${title.toLowerCase()}`} variant="secondary" size="md">
            <CloseMiniIcon />
          </IconButton>
        </header>

        <div className="grid gap-4 px-6 py-5">
          {error && (
            <div className="rounded-2xl border border-[var(--color-red)] bg-[var(--color-red-soft)] px-4 py-3 text-sm text-[var(--color-red)]">
              {error}
            </div>
          )}
          <div className="rounded-2xl border border-[var(--color-blue-border)] bg-[var(--color-blue-soft)] px-4 py-3">
            <p className="text-xs font-bold uppercase tracking-[0.22em] text-[var(--color-blue)]">Target realm</p>
            <p className="mt-1 font-mono text-sm font-bold text-[var(--color-ink)]">{realm}</p>
          </div>
          <label className="grid gap-2">
            <span className="text-xs font-bold uppercase tracking-[0.22em] text-[var(--color-muted-soft)]">Name</span>
            <input
              autoFocus
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="Journey name"
              className="rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-4 py-3 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]"
            />
          </label>
          <label className="grid gap-2">
            <span className="text-xs font-bold uppercase tracking-[0.22em] text-[var(--color-muted-soft)]">Description</span>
            <textarea
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              placeholder="Optional description"
              rows={4}
              className="resize-none rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-4 py-3 text-sm text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]"
            />
          </label>
        </div>

        <footer className="flex justify-end gap-3 border-t border-[var(--color-border-soft)] px-6 py-5">
          <Button onClick={requestClose} variant="secondary" disabled={saving}>
            Cancel
          </Button>
          <Button onClick={() => onConfirm(name, description)} variant="primary" disabled={!canConfirm}>
            {importing ? <ImportIcon /> : <CopyIcon />}
            {saving ? (importing ? 'Importing…' : 'Duplicating…') : title}
          </Button>
        </footer>
      </section>
    </div>,
    document.body,
  );
}
