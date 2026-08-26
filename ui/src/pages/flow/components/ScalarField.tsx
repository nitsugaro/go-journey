import { useEffect, useState } from 'react';
import { Button, IconButton } from '../../../components/Button';
import { isFullPlaceholder, isValidScalarText, parseScalarValue, scalarKindForSchemaType } from '../utils/scalarUtils';
import { formatEditableValue } from '../utils/schemaUtils';
import { BooleanToggle } from './BooleanToggle';
import { ExpandedEditorModal } from './CodeEditors';
import { ScalarTextInput } from './ScalarTextInput';

export function ScalarField({
  label,
  required,
  description,
  value,
  schemaType,
  expression,
  onChange,
}: {
  label: string;
  required?: boolean;
  description?: string;
  value: unknown;
  schemaType?: string;
  expression?: boolean;
  onChange: (value: unknown) => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const [draft, setDraft] = useState(() => formatEditableValue(value));
  const [placeholderMode, setPlaceholderMode] = useState(() => isFullPlaceholder(formatEditableValue(value)));
  const text = formatEditableValue(value);
  const scalarKind = scalarKindForSchemaType(schemaType);
  const strictScalar = scalarKind === 'boolean' || scalarKind === 'number';
  const usePlaceholderMode = strictScalar && placeholderMode;
  const canExpand = expression || !strictScalar || usePlaceholderMode;
  const valid = isValidScalarText(draft, usePlaceholderMode ? 'placeholder' : scalarKind);
  const expandAction = canExpand ? (
    <IconButton
      onClick={(event) => {
        event.preventDefault();
        event.stopPropagation();
        setExpanded(true);
      }}
      label={expression ? 'Expand expression editor' : 'Expand value editor'}
      variant="secondary"
      size="xs"
    >
      ⛶
    </IconButton>
  ) : null;
  const update = (next: string) => {
    setDraft(next);
    if (isValidScalarText(next, usePlaceholderMode ? 'placeholder' : scalarKind))
      onChange(parseScalarValue(next, schemaType));
  };

  useEffect(() => {
    setDraft(text);
    if (strictScalar && isFullPlaceholder(text)) setPlaceholderMode(true);
  }, [text]);

  const switchPlaceholderMode = (nextMode: boolean) => {
    setPlaceholderMode(nextMode);
    if (nextMode) {
      const next = isFullPlaceholder(draft) ? draft : '';
      setDraft(next);
      if (next) onChange(next);
      return;
    }
    const nextValue = scalarKind === 'boolean' ? false : 0;
    setDraft(String(nextValue));
    onChange(nextValue);
  };

  return (
    <div className="text-xs font-semibold uppercase tracking-[0.12em] text-[var(--color-muted-soft)]">
      <span className="flex items-center justify-between gap-2">
        <span>
          {label}
          {required ? ' *' : ''}
        </span>
        <span className="flex items-center gap-1.5">
          {strictScalar && (
            <Button
              onClick={(event) => {
                event.preventDefault();
                switchPlaceholderMode(!placeholderMode);
              }}
              variant={placeholderMode ? 'warning' : 'secondary'}
              size="xs"
              title={placeholderMode ? 'Use literal value' : 'Use placeholder value'}
            >
              {'${}'}
            </Button>
          )}
        </span>
      </span>
      {strictScalar && !placeholderMode ? (
        scalarKind === 'boolean' ? (
          <BooleanToggle
            value={draft === 'true'}
            onChange={(next) => {
              setDraft(String(next));
              onChange(next);
            }}
          />
        ) : (
          <input
            type="number"
            value={draft}
            onChange={(event) => update(event.target.value)}
            className="mt-1 h-[42px] w-full rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-3 py-2.5 text-sm normal-case tracking-normal text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]"
          />
        )
      ) : (
        <ScalarTextInput
          value={draft}
          onChange={update}
          multiline={false}
          language={expression ? 'expression' : 'text'}
          invalid={!valid}
          placeholder={usePlaceholderMode ? '${ctx.value}' : undefined}
          action={expandAction}
        />
      )}
      {!valid && (
        <span className="mt-1 block text-xs normal-case tracking-normal text-[var(--color-red)]">
          Use one complete placeholder like {scalarKind === 'boolean' ? '${ctx.enabled}' : '${ctx.amount}'}.
        </span>
      )}
      {description && (
        <span className="mt-1 block text-xs normal-case tracking-normal text-[var(--color-muted-soft)]">
          {description}
        </span>
      )}
      {expanded && (
        <ExpandedEditorModal
          title={label}
          value={draft}
          language={expression ? 'expression' : 'text'}
          onChange={update}
          onClose={() => setExpanded(false)}
        />
      )}
    </div>
  );
}
