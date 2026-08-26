import { IconButton } from '../../../components/Button';
import type { StepSchema } from '../../../types/journey';
import { normalizeEmbeddedStep, stepSchemaDescription } from '../utils/schemaUtils';
import { ArrowDownMiniIcon, ArrowUpMiniIcon, CloseMiniIcon, EditMiniIcon, PlusMiniIcon } from '../../../components/Icons';

export function NestedStepsField({
  label,
  required,
  description,
  value,
  schemas,
  onChange,
  onEditStep,
  onRemoveStep,
}: {
  label: string;
  required?: boolean;
  description?: string;
  value: unknown;
  schemas: StepSchema[];
  onChange: (value: unknown) => void;
  onEditStep?: (index: number) => void;
  onRemoveStep?: (index: number) => void;
}) {
  const rows = Array.isArray(value) ? value.map(normalizeEmbeddedStep) : [];
  const addStep = () => {
    const next = [...rows, { name: '', step_type: '', config: {} }];
    onChange(next);
    window.setTimeout(() => onEditStep?.(next.length - 1), 0);
  };
  const moveStep = (from: number, to: number) => {
    if (to < 0 || to >= rows.length || from === to) return;
    const next = [...rows];
    const [item] = next.splice(from, 1);
    next.splice(to, 0, item);
    onChange(next);
  };

  return (
    <div className="rounded-3xl bg-[var(--color-surface-subtle)] p-4 ring-1 ring-[var(--color-border-soft)] lg:col-span-2">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.14em] text-[var(--color-muted-soft)]">
            {label}
            {required ? ' *' : ''}
          </p>
          <p className="mt-1 text-xs text-[var(--color-muted-soft)]">
            {description || 'Embedded steps executed inside this step.'}
          </p>
        </div>
        <IconButton onClick={addStep} label="Add embedded step" variant="secondary" size="sm">
          <PlusMiniIcon />
        </IconButton>
      </div>

      <div className="mt-3 grid gap-2">
        {rows.length === 0 && (
          <p className="rounded-2xl bg-[var(--color-surface)] px-3 py-2 text-sm text-[var(--color-muted-soft)]">
            No embedded steps configured.
          </p>
        )}
        {rows.map((row, index) => {
          const schema = schemas.find((item) => item.step_type === row.step_type);
          return (
            <div
              key={`${index}-${row.step_type}-${row.name}`}
              className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-2 rounded-2xl bg-[var(--color-surface)] p-3 ring-1 ring-[var(--color-border-soft)]"
            >
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-semibold text-[var(--color-ink)]">
                  {row.name || row.step_type || `Step ${index + 1}`}
                </p>
                <p className="mt-0.5 truncate text-xs font-semibold uppercase tracking-[0.14em] text-[var(--color-muted-soft)]">
                  {row.step_type || 'Choose step type'}
                  {schema ? (
                    <span className="normal-case tracking-normal"> · {stepSchemaDescription(schema)}</span>
                  ) : null}
                </p>
              </div>
              <div className="flex shrink-0 items-center gap-1 rounded-xl bg-[var(--color-surface-soft)] p-1">
                <IconButton
                  onClick={() => moveStep(index, index - 1)}
                  label="Move step up"
                  variant="ghost"
                  size="xs"
                  disabled={index === 0}
                >
                  <ArrowUpMiniIcon />
                </IconButton>
                <IconButton
                  onClick={() => moveStep(index, index + 1)}
                  label="Move step down"
                  variant="ghost"
                  size="xs"
                  disabled={index === rows.length - 1}
                >
                  <ArrowDownMiniIcon />
                </IconButton>
                <IconButton
                  onClick={() => onEditStep?.(index)}
                  label="Configure embedded step"
                  variant="secondary"
                  size="xs"
                >
                  <EditMiniIcon />
                </IconButton>
                <IconButton
                  onClick={() => onRemoveStep?.(index)}
                  label="Remove embedded step"
                  variant="ghost"
                  size="xs"
                >
                  <CloseMiniIcon />
                </IconButton>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
