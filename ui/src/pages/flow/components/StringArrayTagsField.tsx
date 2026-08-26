import { TagsInput } from '../../../components/TagsInput';

export function StringArrayTagsField({
  label,
  required,
  description,
  value,
  onChange,
}: {
  label: string;
  required?: boolean;
  description?: string;
  value: unknown[];
  onChange: (value: unknown) => void;
}) {
  return (
    <div className="rounded-3xl bg-[var(--color-surface-subtle)] p-4 ring-1 ring-[var(--color-border-soft)] lg:col-span-2">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.14em] text-[var(--color-muted-soft)]">
            {label}
            {required ? ' *' : ''}
          </p>
          {description && <p className="mt-1 text-xs text-[var(--color-muted-soft)]">{description}</p>}
        </div>
      </div>
      <TagsInput
        value={value}
        onChange={(next) => onChange(next)}
        placeholder="Type attribute and press Enter…"
        className="mt-3"
      />
    </div>
  );
}
