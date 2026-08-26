import { Button, IconButton } from '../../../components/Button';
import type { JourneyPropDefinition } from '../flowTypes';
import { normalizeJourneyPropType } from '../utils/journeyStateUtils';

export function JourneyPropsDefinitionEditor({
  props,
  onChange,
}: {
  props: JourneyPropDefinition[];
  onChange: (props: JourneyPropDefinition[]) => void;
}) {
  const addProp = () => {
    const nextIndex = props.length + 1;
    onChange([...props, { id: `prop_${nextIndex}`, name: `Prop ${nextIndex}`, type: 'string' }]);
  };
  const updateProp = (index: number, patch: Partial<JourneyPropDefinition>) => {
    onChange(props.map((prop, propIndex) => (propIndex === index ? { ...prop, ...patch } : prop)));
  };
  const removeProp = (index: number) => onChange(props.filter((_, propIndex) => propIndex !== index));

  return (
    <div className="mt-5 rounded-3xl bg-[var(--color-surface-subtle)] p-4 ring-1 ring-[var(--color-border-soft)]">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.16em] text-[var(--color-muted-soft)]">Props</p>
          <p className="mt-1 text-xs text-[var(--color-muted-soft)]">
            Optional typed values a parent journey can pass when this journey is used as a sub-journey.
          </p>
        </div>
        <Button onClick={addProp} variant="secondary" size="sm">
          + prop
        </Button>
      </div>
      <div className="mt-3 grid gap-2">
        {props.length === 0 && (
          <p className="rounded-2xl bg-[var(--color-surface)] px-3 py-2 text-sm text-[var(--color-muted-soft)]">
            No props declared.
          </p>
        )}
        {props.map((prop, index) => (
          <div
            key={index}
            className="grid gap-2 rounded-2xl bg-[var(--color-surface)] p-2 ring-1 ring-[var(--color-border-soft)] md:grid-cols-[1fr_1fr_140px_auto]"
          >
            <input
              value={prop.id}
              onChange={(event) => updateProp(index, { id: event.target.value.replace(/\s+/g, '_') })}
              className="rounded-xl bg-[var(--color-surface-subtle)] px-3 py-2 text-sm font-semibold text-[var(--color-ink)] outline-none focus:ring-2 focus:ring-[var(--color-blue)]"
              placeholder="id"
            />
            <input
              value={prop.name}
              onChange={(event) => updateProp(index, { name: event.target.value })}
              className="rounded-xl bg-[var(--color-surface-subtle)] px-3 py-2 text-sm font-semibold text-[var(--color-ink)] outline-none focus:ring-2 focus:ring-[var(--color-blue)]"
              placeholder="Display name"
            />
            <select
              value={normalizeJourneyPropType(prop.type)}
              onChange={(event) => updateProp(index, { type: normalizeJourneyPropType(event.target.value) })}
              className="rounded-xl bg-[var(--color-surface-subtle)] px-3 py-2 text-sm font-semibold text-[var(--color-ink)] outline-none focus:ring-2 focus:ring-[var(--color-blue)]"
            >
              <option value="string">string</option>
              <option value="int">int</option>
              <option value="float">float</option>
              <option value="bool">bool</option>
              <option value="list">list</option>
              <option value="object">object</option>
            </select>
            <IconButton onClick={() => removeProp(index)} label="Remove prop" variant="ghost" size="sm">
              ×
            </IconButton>
          </div>
        ))}
      </div>
    </div>
  );
}
