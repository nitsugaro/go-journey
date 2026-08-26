import { useEffect, useMemo, useState } from 'react';
import { IconButton } from '../../../components/Button';
import { ExpandedEditorModal, HighlightedTextEditor } from './CodeEditors';

function formatJSON(value: unknown) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return '{}';
  return JSON.stringify(value, null, 2);
}

function parseJSONObject(value: string) {
  const parsed = JSON.parse(value || '{}');
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('Value must be a JSON object.');
  }
  return parsed as Record<string, unknown>;
}

export function JsonObjectField({
  label,
  required,
  description,
  value,
  onChange,
}: {
  label: string;
  required?: boolean;
  description?: string;
  value: unknown;
  onChange: (value: unknown) => void;
}) {
  const sourceText = useMemo(() => formatJSON(value), [value]);
  const [draft, setDraft] = useState(sourceText);
  const [expanded, setExpanded] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => setDraft(sourceText), [sourceText]);

  const update = (next: string) => {
    setDraft(next);
    try {
      onChange(parseJSONObject(next));
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Invalid JSON object.');
    }
  };

  return (
    <div className="text-xs font-semibold uppercase tracking-[0.12em] text-[var(--color-muted-soft)] lg:col-span-2">
      <span className="flex items-center justify-between gap-2">
        <span>
          {label}
          {required ? ' *' : ''}
        </span>
        <IconButton
          onClick={(event) => {
            event.preventDefault();
            event.stopPropagation();
            setExpanded(true);
          }}
          label={`Expand ${label} JSON editor`}
          variant="secondary"
          size="xs"
        >
          ⛶
        </IconButton>
      </span>
      <HighlightedTextEditor
        value={draft}
        onChange={update}
        language="json"
        multiline={false}
        invalid={Boolean(error)}
        className="mt-1"
      />
      {error && <span className="mt-1 block text-xs normal-case tracking-normal text-[var(--color-red)]">{error}</span>}
      {description && (
        <span className="mt-1 block text-xs normal-case tracking-normal text-[var(--color-muted-soft)]">
          {description}
        </span>
      )}
      {expanded && (
        <ExpandedEditorModal
          title={label}
          value={draft}
          language="json"
          onChange={update}
          onClose={() => setExpanded(false)}
        />
      )}
    </div>
  );
}
