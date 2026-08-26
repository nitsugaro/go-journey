import { useEffect } from 'react';
import { formatEditableValue } from '../utils/schemaUtils';

export function EnumScalarField({
  label,
  required,
  description,
  value,
  options,
  onChange,
}: {
  label: string;
  required?: boolean;
  description?: string;
  value: unknown;
  options: string[];
  onChange: (value: unknown) => void;
}) {
  const text = formatEditableValue(value);
  const currentValue = options.includes(text) ? text : options[0] || '';

  useEffect(() => {
    if (options.length > 0 && text !== currentValue) {
      onChange(currentValue);
    }
  }, [currentValue, options.length, text]);

  return (
    <label className="text-xs font-semibold uppercase tracking-[0.12em] text-[var(--color-muted-soft)]">
      {label}
      {required ? ' *' : ''}
      <select
        value={currentValue}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 w-full rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-3 py-2.5 text-sm font-semibold normal-case tracking-normal text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]"
      >
        {options.map((option) => (
          <option key={option} value={option}>
            {option}
          </option>
        ))}
      </select>
      {description && (
        <span className="mt-1 block text-xs normal-case tracking-normal text-[var(--color-muted-soft)]">
          {description}
        </span>
      )}
    </label>
  );
}
