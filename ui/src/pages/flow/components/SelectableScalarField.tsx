import { useEffect, useState } from 'react';
import { Button, IconButton } from '../../../components/Button';
import type { SelectableOption, SelectableSource } from '../flowTypes';
import { formatEditableValue } from '../utils/schemaUtils';
import { fetchSelectableOptions } from '../utils/selectableUtils';
import { ScriptEditorModal } from '../../scripts/ScriptEditorPanel';

export function SelectableScalarField({
  realm,
  label,
  required,
  description,
  value,
  source,
  onChange,
}: {
  realm: string;
  label: string;
  required?: boolean;
  description?: string;
  value: unknown;
  source: SelectableSource;
  onChange: (value: unknown) => void;
}) {
  const [options, setOptions] = useState<SelectableOption[]>([]);
  const [resultCount, setResultCount] = useState(0);
  const [remoteSearch, setRemoteSearch] = useState('');
  const [limit, setLimit] = useState(20);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [editingScriptID, setEditingScriptID] = useState('');
  const text = formatEditableValue(value);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError('');
    fetchSelectableOptions(realm, source, { name: remoteSearch, limit })
      .then((response) => {
        if (cancelled) return;
        setOptions(response.options);
        setResultCount(response.resultCount);
      })
      .catch((err: Error) => {
        if (!cancelled) setError(err.message);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [
    realm,
    source.resource,
    source.endpoint,
    JSON.stringify(source.query),
    source.nameProperty,
    source.valueProperty,
    remoteSearch,
    limit,
  ]);

  useEffect(() => {
    setLimit(20);
  }, [remoteSearch, source.resource, source.endpoint, JSON.stringify(source.query)]);

  const hasCurrent = text && options.some((option) => option.value === text);
  const canLoadMore = options.length < resultCount;
  const canOpenScript = source.resource === 'scripts' && Boolean(text);

  return (
    <div className="text-xs font-semibold uppercase tracking-[0.12em] text-[var(--color-muted-soft)]">
      <div className="flex items-center justify-between gap-2">
        <span>
          {label}
          {required ? ' *' : ''}
        </span>
        {canOpenScript && (
          <IconButton
            onClick={(event) => {
              event.preventDefault();
              setEditingScriptID(text);
            }}
            label="Edit script without leaving canvas"
            variant="secondary"
            size="xs"
          >
            ✎
          </IconButton>
        )}
      </div>
      <div className="mt-1 rounded-2xl bg-[var(--color-surface-subtle)] p-2 ring-1 ring-[var(--color-border-soft)] focus-within:bg-[var(--color-surface)] focus-within:ring-2 focus-within:ring-[var(--color-blue)]">
        <input
          value={remoteSearch}
          onChange={(event) => setRemoteSearch(event.target.value)}
          placeholder="Search option by name..."
          className="mb-2 h-9 w-full rounded-xl border border-transparent bg-[var(--color-surface)] px-3 text-sm font-semibold normal-case tracking-normal text-[var(--color-ink)] outline-none placeholder:text-[var(--color-muted-soft)]"
        />
        <select
          value={text}
          onChange={(event) => onChange(event.target.value)}
          className="w-full rounded-xl border border-transparent bg-transparent px-2 py-2 text-sm font-semibold normal-case tracking-normal text-[var(--color-ink)] outline-none"
        >
          <option value="">{loading ? 'Loading options…' : 'Select option'}</option>
          {text && !hasCurrent && <option value={text}>{text}</option>}
          {options.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
        <div className="mt-2 flex items-center justify-between gap-2 border-t border-[var(--color-border-soft)] pt-2">
          <span className="text-[11px] font-semibold normal-case tracking-normal text-[var(--color-muted-soft)]">
            {loading ? 'Loading…' : `${options.length} / ${resultCount || options.length}`}
          </span>
          {canLoadMore && (
            <Button
              onClick={(event) => {
                event.preventDefault();
                setLimit((current) => current + 20);
              }}
              variant="secondary"
              size="xs"
            >
              more
            </Button>
          )}
        </div>
      </div>
      {description && (
        <span className="mt-1 block text-xs normal-case tracking-normal text-[var(--color-muted-soft)]">
          {description}
        </span>
      )}
      {error && <span className="mt-1 block text-xs normal-case tracking-normal text-[var(--color-red)]">{error}</span>}
      {editingScriptID && (
        <ScriptEditorModal
          realm={realm}
          scriptId={editingScriptID}
          onClose={() => setEditingScriptID('')}
          onSaved={(script) => {
            if (!script.id) return;
            setOptions((current) =>
              current.map((option) =>
                option.value === script.id ? { ...option, label: script.name || script.id || option.label } : option
              )
            );
          }}
        />
      )}
    </div>
  );
}
