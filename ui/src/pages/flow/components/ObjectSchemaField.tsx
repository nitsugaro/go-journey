import type { JourneyConfiguration, StepSchema } from '../../../types/journey';
import type { JSONSchema } from '../flowTypes';
import { asRecord, mergeSchemaUIMetadata, orderedSchemaProperties, schemaChildOverride } from '../utils/schemaUtils';
import { SchemaField } from './SchemaField';

export function ObjectSchemaField({
  realm,
  label,
  required,
  schema,
  rootSchema,
  value,
  schemas,
  journey,
  onChange,
}: {
  realm: string;
  label: string;
  required?: boolean;
  schema: JSONSchema;
  rootSchema?: JSONSchema | null;
  value: unknown;
  schemas?: StepSchema[];
  journey?: JourneyConfiguration | null;
  onChange: (value: unknown) => void;
}) {
  const current = asRecord(value);
  const properties = orderedSchemaProperties(schema, rootSchema || schema);
  return (
    <div className="rounded-3xl bg-[var(--color-surface-subtle)] p-4 ring-1 ring-[var(--color-border-soft)] lg:col-span-2">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.14em] text-[var(--color-muted-soft)]">
            {label}
            {required ? ' *' : ''}
          </p>
          {schema.description && <p className="mt-1 text-xs text-[var(--color-muted-soft)]">{schema.description}</p>}
        </div>
      </div>
      <div className="mt-3 grid gap-3 lg:grid-cols-2">
        {properties.map(([key, property]) => (
          <SchemaField
            key={key}
            realm={realm}
            name={key}
            schema={mergeSchemaUIMetadata(property, schemaChildOverride(schema, key))}
            rootSchema={rootSchema || schema}
            value={current[key]}
            config={current}
            schemas={schemas}
            journey={journey}
            required={schema.required?.includes(key)}
            onChange={(nextValue) => onChange({ ...current, [key]: nextValue })}
          />
        ))}
      </div>
    </div>
  );
}
