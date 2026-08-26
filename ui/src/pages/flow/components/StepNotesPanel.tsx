import { useMemo, useState } from 'react';
import { Button, IconButton } from '../../../components/Button';
import type { JourneyNote } from '../flowTypes';
import { formatNoteTimestamp } from '../utils/journeyRuntimeUtils';
import { CloseMiniIcon, NoteTinyIcon } from '../../../components/Icons';

export function StepNotesPanel({
  stepName,
  notes,
  onAdd,
  onRemove,
  onClose,
}: {
  stepName: string;
  notes: JourneyNote[];
  onAdd: (note: string) => void;
  onRemove: (index: number) => void;
  onClose: () => void;
}) {
  const [draft, setDraft] = useState('');
  const sortedNotes = useMemo(
    () =>
      notes
        .map((note, originalIndex) => ({ note, originalIndex }))
        .sort((left, right) => left.note.timestamp - right.note.timestamp),
    [notes]
  );

  const submit = () => {
    const clean = draft.trim();
    if (!clean) return;
    onAdd(clean);
    setDraft('');
  };

  return (
    <div className="flex h-full flex-col overflow-hidden rounded-[2rem] border border-[var(--color-border)] bg-[var(--color-surface)] shadow-2xl shadow-[var(--color-muted-faint)] backdrop-blur-xl">
      <div className="shrink-0 border-b border-[var(--color-border-soft)] px-5 py-4">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="text-xs font-semibold uppercase tracking-[0.2em] text-[var(--color-blue)]">Step notes</p>
            <h2 className="mt-1 truncate text-xl font-semibold tracking-tight">{stepName}</h2>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <span className="inline-flex items-center gap-2 rounded-full bg-[var(--color-surface-soft)] px-3 py-1 text-xs font-semibold text-[var(--color-muted)] ring-1 ring-[var(--color-border-soft)]">
              <NoteTinyIcon />
              {sortedNotes.length}
            </span>
            <IconButton onClick={onClose} label="Close notes" variant="secondary" size="sm">
              <CloseMiniIcon />
            </IconButton>
          </div>
        </div>
      </div>

      <div className="flex min-h-0 flex-1 flex-col bg-[var(--color-surface-subtle)]">
        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-5">
          {sortedNotes.length > 0 ? (
            <div className="space-y-3">
              {sortedNotes.map(({ note, originalIndex }, noteIndex) => (
                <article
                  key={`${note.timestamp}-${originalIndex}`}
                  className="group rounded-3xl border border-[var(--color-border-soft)] bg-[var(--color-surface)] p-4 shadow-sm"
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <p className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-muted-soft)]">
                        Note {noteIndex + 1}
                      </p>
                      <time className="mt-1 block text-xs font-medium text-[var(--color-muted)]">
                        {formatNoteTimestamp(note.timestamp)}
                      </time>
                    </div>
                    <IconButton
                      onClick={() => onRemove(originalIndex)}
                      label="Remove note"
                      variant="ghost"
                      size="xs"
                      className="opacity-70 transition group-hover:opacity-100"
                    >
                      <CloseMiniIcon />
                    </IconButton>
                  </div>
                  <p className="mt-3 max-w-full whitespace-pre-wrap break-words text-sm leading-6 text-[var(--color-ink)] [overflow-wrap:anywhere]">
                    {note.note}
                  </p>
                </article>
              ))}
            </div>
          ) : (
            <div className="flex h-full min-h-56 items-center justify-center rounded-3xl border border-dashed border-[var(--color-border)] bg-[var(--color-surface)] px-6 text-center">
              <div>
                <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-2xl bg-[var(--color-surface-soft)] text-[var(--color-muted)] ring-1 ring-[var(--color-border-soft)]">
                  <NoteTinyIcon />
                </div>
                <p className="mt-3 text-sm font-semibold text-[var(--color-ink)]">No notes for this step</p>
                <p className="mt-1 text-xs leading-5 text-[var(--color-muted)]">
                  Add a short implementation note, warning, or reminder below.
                </p>
              </div>
            </div>
          )}
        </div>

        <div className="shrink-0 border-t border-[var(--color-border-soft)] bg-[var(--color-surface)] p-4">
          <label className="block text-xs font-semibold uppercase tracking-[0.14em] text-[var(--color-muted-soft)]">
            Add note
            <textarea
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              className="mt-2 max-h-36 min-h-24 w-full resize-none rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-3 py-3 text-sm normal-case tracking-normal text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] transition focus:ring-2 focus:ring-[var(--color-blue)]"
              placeholder="Write a note for this step..."
            />
          </label>
          <div className="mt-3 flex justify-end">
            <Button onClick={submit} disabled={!draft.trim()} variant="primary" size="sm">
              Add note
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
