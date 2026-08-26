import { Button, IconButton } from '../../../components/Button';
import { PlaceholderValueField } from '../../../components/PlaceholderValueField';
import { TagsInput } from '../../../components/TagsInput';
import type { JourneyConfiguration, StepSchema } from '../../../types/journey';
import type { JSONSchema } from '../flowTypes';
import { asRecord, defaultValueForSchema, mergeSchemaUIMetadata, orderedSchemaProperties, schemaChildOverride, schemaType, unwrapSchema } from '../utils/schemaUtils';
import { SchemaField } from './SchemaField';
import { StringArrayTagsField } from './StringArrayTagsField';

export function ArraySchemaField({
  realm,
  label,
  required,
  schema,
  rootSchema,
  value,
  schemas,
  journey,
  onChange,
  allowPlaceholder = false,
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
  allowPlaceholder?: boolean;
}) {
  const items = unwrapSchema(schema.items || {}, rootSchema || schema);
  if (allowPlaceholder) {
    return (
      <div className="rounded-3xl bg-[var(--color-surface-subtle)] p-4 ring-1 ring-[var(--color-border-soft)] lg:col-span-2">
        <PlaceholderValueField
          label={label}
          required={required}
          description={schema.description}
          value={typeof value === 'string' ? value : Array.isArray(value) ? value : []}
          emptyValue={[]}
          onChange={onChange}
        >
          {(rows) => schemaType(items) === 'string' && !items.properties && !items.additionalProperties ? (
            <TagsInput value={rows} onChange={onChange} />
          ) : (
            <ArraySchemaField
              realm={realm}
              label="Items"
              schema={schema}
              rootSchema={rootSchema}
              value={rows}
              schemas={schemas}
              journey={journey}
              onChange={onChange}
            />
          )}
        </PlaceholderValueField>
      </div>
    );
  }
  const rows = Array.isArray(value) ? value : [];
  if (schemaType(items) === 'string' && !items.properties && !items.additionalProperties) {
    return (
      <StringArrayTagsField
        label={label}
        required={required}
        description={schema.description}
        value={rows}
        onChange={onChange}
      />
    );
  }
  const addRow = () => onChange([...rows, defaultValueForSchema(items) ?? (items.properties ? {} : '')]);
  const updateRow = (index: number, nextValue: unknown) =>
    onChange(rows.map((row, rowIndex) => (rowIndex === index ? nextValue : row)));
  const removeRow = (index: number) => onChange(rows.filter((_, rowIndex) => rowIndex !== index));

  return (
    <div className="rounded-3xl bg-[var(--color-surface-subtle)] p-4 ring-1 ring-[var(--color-border-soft)] lg:col-span-2">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.14em] text-[var(--color-muted-soft)]">
            {label}
            {required ? ' *' : ''}
          </p>
          {schema.description && <p className="mt-1 text-xs text-[var(--color-muted-soft)]">{schema.description}</p>}
        </div>
        <Button onClick={addRow} variant="secondary" size="sm">
          + item
        </Button>
      </div>
      <div className="mt-3 grid gap-3">
        {rows.length === 0 && (
          <p className="rounded-2xl bg-[var(--color-surface)] px-3 py-2 text-sm text-[var(--color-muted-soft)]">
            No items configured.
          </p>
        )}
        {rows.map((row, index) => {
          const rowRecord = asRecord(row);
          const itemProperties = orderedSchemaProperties(items, rootSchema || schema);
          return (
            <div
              key={index}
              className="rounded-3xl bg-[var(--color-surface)] p-3 shadow-sm ring-1 ring-[var(--color-border-soft)]"
            >
              <div className="mb-3 flex items-center justify-between">
                <span className="rounded-full bg-[var(--color-surface-soft)] px-2.5 py-1 text-xs font-semibold text-[var(--color-muted)]">
                  Item {index + 1}
                </span>
                <IconButton onClick={() => removeRow(index)} label="Remove item" variant="ghost" size="xs">
                  ×
                </IconButton>
              </div>
              {itemProperties.length > 0 ? (
                <div className="grid gap-3 lg:grid-cols-2">
                  {itemProperties.map(([key, property]) => (
                    <SchemaField
                      key={key}
                      realm={realm}
                      name={key}
                      schema={mergeSchemaUIMetadata(property, schemaChildOverride(schema, key))}
                      rootSchema={rootSchema || schema}
                      value={rowRecord[key]}
                      config={rowRecord}
                      schemas={schemas}
                      journey={journey}
                      required={items.required?.includes(key)}
                      onChange={(nextValue) => updateRow(index, { ...rowRecord, [key]: nextValue })}
                    />
                  ))}
                </div>
              ) : (
                <SchemaField
                  realm={realm}
                  name="value"
                  schema={items}
                  rootSchema={rootSchema || schema}
                  value={row}
                  schemas={schemas}
                  journey={journey}
                  onChange={(nextValue) => updateRow(index, nextValue)}
                />
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
