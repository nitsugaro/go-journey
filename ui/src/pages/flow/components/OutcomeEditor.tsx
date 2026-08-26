import { Button, IconButton } from '../../../components/Button';
import type { JSONSchema } from '../flowTypes';
import { staticOutcomeNamesFromSchema } from '../utils/schemaUtils';
import { uniqueStrings } from '../utils/journeyStateUtils';

export function OutcomeEditor({
  schema,
  outcomes,
  onChange,
}: {
  schema: JSONSchema;
  outcomes: Record<string, string>;
  onChange: (outcomes: Record<string, string>) => void;
}) {
  const dynamic = schema['x-dynamic-outcome'] === true || Object.keys(schema.properties || {}).length === 0;
  const keys = dynamic
    ? Object.keys(outcomes)
    : uniqueStrings([...staticOutcomeNamesFromSchema(schema), ...Object.keys(outcomes)]);

  const updateKey = (oldKey: string, newKey: string) => {
    const next: Record<string, string> = {};
    for (const [key, value] of Object.entries(outcomes)) {
      next[key === oldKey ? newKey : key] = value;
    }
    onChange(next);
  };

  return (
    <div className="mt-5 rounded-3xl bg-[var(--color-surface-subtle)] p-4 ring-1 ring-[var(--color-border-soft)]">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.16em] text-[var(--color-muted-soft)]">Outcomes</p>
          <p className="text-sm text-[var(--color-muted)]">
            {dynamic ? 'Dynamic outcome keys are allowed for this step.' : 'Schema-defined outcome keys are suggested.'}
          </p>
        </div>
        {dynamic && (
          <Button
            onClick={() => onChange({ ...outcomes, [`outcome_${Object.keys(outcomes).length + 1}`]: '' })}
            variant="secondary"
            size="sm"
          >
            + outcome
          </Button>
        )}
      </div>

      <div className="mt-3 grid gap-2">
        {keys.length === 0 && (
          <p className="rounded-2xl bg-[var(--color-surface)] px-3 py-2 text-sm text-[var(--color-muted-soft)]">
            No outcomes configured.
          </p>
        )}
        {keys.map((key, index) => (
          <div
            key={index}
            className="grid gap-2 rounded-2xl bg-[var(--color-surface)] p-2 ring-1 ring-[var(--color-border-soft)] sm:grid-cols-[1fr_auto]"
          >
            <input
              value={key}
              disabled={!dynamic}
              onChange={(event) => updateKey(key, event.target.value)}
              className="rounded-xl bg-[var(--color-surface-subtle)] px-3 py-2 text-sm font-semibold text-[var(--color-ink)] outline-none disabled:text-[var(--color-muted-soft)]"
            />
            <IconButton
              onClick={() => {
                const next = { ...outcomes };
                delete next[key];
                onChange(next);
              }}
              label="Remove outcome"
              variant="ghost"
              size="sm"
            >
              ×
            </IconButton>
          </div>
        ))}
      </div>
    </div>
  );
}

