import { useEffect, useState } from 'react';
import { getJourney } from '../../../api/journeyApi';
import type { JourneyPropDefinition } from '../flowTypes';
import { asRecord } from '../utils/schemaUtils';
import { normalizeJourneyPropDefinitions, schemaForJourneyProp } from '../utils/journeyStateUtils';
import { SchemaField } from './SchemaField';

export function SubJourneyPropsField({
  realm,
  label,
  required,
  description,
  journeyID,
  value,
  onChange,
}: {
  realm: string;
  label: string;
  required?: boolean;
  description?: string;
  journeyID: string;
  value: unknown;
  onChange: (value: unknown) => void;
}) {
  const [props, setProps] = useState<JourneyPropDefinition[]>([]);
  const [childName, setChildName] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const current = asRecord(value);

  useEffect(() => {
    let cancelled = false;
    setProps([]);
    setChildName('');
    setError('');
    if (!journeyID) return;

    setLoading(true);
    getJourney(realm, journeyID)
      .then((journey) => {
        if (cancelled) return;
        setChildName(journey.name || journeyID);
        setProps(normalizeJourneyPropDefinitions(journey.additional_properties?.props));
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
  }, [realm, journeyID]);

  const setProp = (propID: string, nextValue: unknown) => {
    onChange({ ...current, [propID]: nextValue });
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
            {description || 'Typed values pushed to the sub-journey internal props context.'}
          </p>
        </div>
        {childName && (
          <span className="max-w-44 truncate rounded-full bg-[var(--color-blue-soft)] px-3 py-1 text-xs font-semibold text-[var(--color-blue)]">
            {childName}
          </span>
        )}
      </div>

      <div className="mt-3 grid gap-3">
        {!journeyID && (
          <p className="rounded-2xl bg-[var(--color-surface)] px-3 py-2 text-sm text-[var(--color-muted-soft)]">
            Select a sub-journey first to configure props.
          </p>
        )}
        {loading && (
          <p className="rounded-2xl bg-[var(--color-surface)] px-3 py-2 text-sm text-[var(--color-muted-soft)]">
            Loading sub-journey props…
          </p>
        )}
        {error && (
          <p className="rounded-2xl bg-[var(--color-red-soft)] px-3 py-2 text-sm text-[var(--color-red)]">{error}</p>
        )}
        {!loading && !error && journeyID && props.length === 0 && (
          <p className="rounded-2xl bg-[var(--color-surface)] px-3 py-2 text-sm text-[var(--color-muted-soft)]">
            This sub-journey does not declare props.
          </p>
        )}
        {props.map((prop) => (
          <SchemaField
            key={prop.id}
            realm={realm}
            name={prop.name || prop.id}
            schema={schemaForJourneyProp(prop)}
            value={current[prop.id]}
            onChange={(nextValue) => setProp(prop.id, nextValue)}
          />
        ))}
      </div>
    </div>
  );
}
