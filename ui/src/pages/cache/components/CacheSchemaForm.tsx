import { useMemo } from 'react';
import type { CacheInfo } from '../../../types/journey';
import type { JSONSchema } from '../../flow/flowTypes';
import { SchemaField } from '../../flow/components/SchemaField';
import { asRecord, mergeSchemaUIMetadata, orderedSchemaProperties, schemaChildOverride } from '../../flow/utils/schemaUtils';
import { stringifyConfig } from '../cacheUtils';

type CacheSchemaFormProps = {
  realm: string;
  cache?: CacheInfo;
  configText: string;
  onChange: (value: string) => void;
};

export function CacheSchemaForm({ realm, cache, configText, onChange }: CacheSchemaFormProps) {
  const schema = useMemo(() => asSchema(cache?.schema), [cache?.schema]);
  const parsed = useMemo(() => parseConfig(configText), [configText]);

  if (!schema) {
    return (
      <div className="rounded-3xl border border-[var(--color-border-soft)] bg-[var(--color-surface-subtle)] p-5 text-sm text-[var(--color-muted)]">
        This instance type does not expose a configuration schema. Use raw JSON mode.
      </div>
    );
  }

  if (!parsed.valid) {
    return (
      <div className="rounded-3xl border border-[var(--color-red-border)] bg-[var(--color-red-soft)] p-5 text-sm text-[var(--color-red)]">
        Current JSON is invalid. Fix it in raw JSON mode before using the form.
      </div>
    );
  }

  const current = asRecord(parsed.value);
  const properties = orderedSchemaProperties(schema, schema);

  return (
    <div className="grid gap-4 lg:grid-cols-2">
      {properties.length === 0 && (
        <p className="rounded-2xl bg-[var(--color-surface-subtle)] px-4 py-3 text-sm text-[var(--color-muted)] lg:col-span-2">
          No schema fields available for this instance type.
        </p>
      )}
      {properties.map(([key, property]) => (
        <SchemaField
          key={key}
          realm={realm}
          name={key}
          schema={mergeSchemaUIMetadata(property, schemaChildOverride(schema, key))}
          rootSchema={schema}
          value={current[key]}
          config={current}
          required={schema.required?.includes(key)}
          onChange={(nextValue) => onChange(stringifyConfig({ ...current, [key]: nextValue }))}
        />
      ))}
    </div>
  );
}

function parseConfig(value: string): { valid: true; value: unknown } | { valid: false } {
  try {
    return { valid: true, value: JSON.parse(value || '{}') };
  } catch {
    return { valid: false };
  }
}

function asSchema(value: unknown): JSONSchema | null {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as JSONSchema) : null;
}
