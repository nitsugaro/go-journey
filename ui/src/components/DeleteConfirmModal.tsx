import { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import { Button, IconButton } from './Button';
import { CloseMiniIcon, DeleteIcon } from './Icons';

type DeleteConfirmModalProps = {
  open: boolean;
  itemLabel: string;
  itemName: string;
  title?: string;
  description?: string;
  confirming?: boolean;
  onCancel: () => void;
  onConfirm: () => void;
};

export function DeleteConfirmModal({
  open,
  itemLabel,
  itemName,
  title = 'Confirm delete',
  description,
  confirming = false,
  onCancel,
  onConfirm,
}: DeleteConfirmModalProps) {
  const [typedName, setTypedName] = useState('');
  const [closing, setClosing] = useState(false);
  const canDelete = typedName === itemName && !confirming;

  useEffect(() => {
    if (!open) return;
    setTypedName('');
    setClosing(false);
  }, [open, itemName]);

  if (!open) return null;

  function requestClose() {
    if (closing || confirming) return;
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
        className="motion-modal-surface w-[min(560px,94vw)] overflow-hidden rounded-[2rem] border border-[var(--color-red-border)] bg-[var(--color-surface)] shadow-2xl"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <header className="flex items-start justify-between gap-4 border-b border-[var(--color-border-soft)] px-6 py-5">
          <div>
            <p className="text-xs font-bold uppercase tracking-[0.28em] text-[var(--color-red)]">{itemLabel}</p>
            <h2 className="mt-2 text-2xl font-bold text-[var(--color-ink)]">{title}</h2>
            <p className="mt-2 text-sm text-[var(--color-muted)]">
              {description || `Type the ${itemLabel.toLowerCase()} name to confirm permanent deletion.`}
            </p>
          </div>
          <IconButton onClick={requestClose} label="Cancel delete" variant="secondary" size="md">
            <CloseMiniIcon />
          </IconButton>
        </header>

        <div className="grid gap-4 px-6 py-5">
          <div className="rounded-2xl border border-[var(--color-border-soft)] bg-[var(--color-surface-subtle)] p-4">
            <p className="text-xs font-bold uppercase tracking-[0.24em] text-[var(--color-muted-soft)]">Delete target</p>
            <p className="mt-2 select-all break-words font-mono text-sm font-bold text-[var(--color-ink)]">{itemName}</p>
          </div>
          <label className="grid gap-2">
            <span className="text-xs font-bold uppercase tracking-[0.24em] text-[var(--color-muted-soft)]">Confirm name</span>
            <input
              autoFocus
              value={typedName}
              onChange={(event) => setTypedName(event.target.value)}
              placeholder={itemName}
              className="rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-4 py-3 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-red)]"
            />
          </label>
        </div>

        <footer className="flex justify-end gap-3 border-t border-[var(--color-border-soft)] px-6 py-5">
          <Button onClick={requestClose} variant="secondary" disabled={confirming}>
            Cancel
          </Button>
          <Button onClick={onConfirm} variant="danger" disabled={!canDelete}>
            <span className="inline-flex items-center gap-2">
              <DeleteIcon /> {confirming ? 'Deleting…' : 'Delete'}
            </span>
          </Button>
        </footer>
      </section>
    </div>,
    document.body,
  );
}
