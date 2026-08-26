import type { JourneyConfiguration, StepSchema } from '../../../types/journey';
import { PlaceholderValueField } from '../../../components/PlaceholderValueField';
import { normalizeJourneyType } from '../../../config/typeOptions';
import type { JSONSchema, SelectableSource } from '../flowTypes';
import { humanizeKey } from '../utils/journeyStateUtils';
import { inferSchemaFromValue, isEmbeddedStepsField, isExpressionField, isPlaceholderSchema, isScriptableSchema, schemaType, unwrapSchema } from '../utils/schemaUtils';
import { selectableSource, stringProp } from '../utils/selectableUtils';
import { ArraySchemaField } from './ArraySchemaField';
import { EnumScalarField } from './EnumScalarField';
import { JsonObjectField } from './JsonObjectField';
import { KeyValueObjectField } from './KeyValueObjectField';
import { NestedStepsField } from './NestedStepsField';
import { ObjectSchemaField } from './ObjectSchemaField';
import { ScalarField } from './ScalarField';
import { ScriptArgsConfigField } from './ScriptArgsConfigField';
import { SelectableScalarField } from './SelectableScalarField';
import { SubJourneyPropsField } from './SubJourneyPropsField';

export function SchemaField({
  realm,
  name,
  schema,
  rootSchema,
  value,
  config,
  schemas,
  journey,
  required,
  onChange,
  onEditNestedStep,
  onRemoveNestedStep,
}: {
  realm: string;
  name: string;
  schema: JSONSchema;
  rootSchema?: JSONSchema | null;
  value: unknown;
  config?: Record<string, unknown>;
  schemas?: StepSchema[];
  journey?: JourneyConfiguration | null;
  required?: boolean;
  onChange: (value: unknown) => void;
  onEditNestedStep?: (index: number) => void;
  onRemoveNestedStep?: (index: number) => void;
}) {
  const property = unwrapSchema(schema, rootSchema || schema);
  const type = schemaType(property);
  const label = humanizeKey(name);
  const commonLabel = 'text-xs font-semibold uppercase tracking-[0.12em] text-[var(--color-muted-soft)]';
  const commonInput =
    'mt-1 w-full rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-3 py-2.5 text-sm normal-case tracking-normal text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]';
  const selectable = selectableSource(schema) || selectableSource(property);
  const allowsPlaceholder = [...(schema.anyOf || []), ...(schema.oneOf || [])].some(isPlaceholderSchema);

  if (schema['x-type'] === 'json-object' || property['x-type'] === 'json-object') {
    return (
      <JsonObjectField
        label={label}
        required={required}
        description={property.description || schema.description}
        value={value}
        onChange={onChange}
      />
    );
  }

  if (schema['x-type'] === 'script-args' || property['x-type'] === 'script-args') {
    const props = schema['x-props'] || property['x-props'] || {};
    const sourceProperty = stringProp(props, 'sourceProperty') || 'script_id';
    return (
      <ScriptArgsConfigField
        realm={realm}
        label={label}
        required={required}
        description={property.description || schema.description}
        scriptID={typeof config?.[sourceProperty] === 'string' ? config[sourceProperty] : ''}
        value={value}
        onChange={onChange}
      />
    );
  }

  if (schema['x-sub-journey-props'] || property['x-sub-journey-props']) {
    return (
      <SubJourneyPropsField
        realm={realm}
        label={label}
        required={required}
        description={property.description || schema.description}
        journeyID={typeof config?.journey_id === 'string' ? config.journey_id : ''}
        value={value}
        onChange={onChange}
      />
    );
  }

  if (selectable) {
    return (
      <SelectableScalarField
        realm={realm}
        label={label}
        required={required}
        description={property.description || schema.description}
        value={value}
        source={selectableSourceForJourney(name, selectable, journey?.journey_type)}
        onChange={onChange}
      />
    );
  }

  if (isEmbeddedStepsField(name, property, rootSchema || schema)) {
    return (
      <NestedStepsField
        label={label}
        required={required}
        description={property.description}
        value={value}
        schemas={schemas || []}
        onChange={onChange}
        onEditStep={onEditNestedStep}
        onRemoveStep={onRemoveNestedStep}
      />
    );
  }

  if (property.enum?.length) {
    return (
      <EnumScalarField
        label={label}
        required={required}
        description={property.description}
        value={value}
        options={property.enum.map((item) => String(item))}
        onChange={onChange}
      />
    );
  }

  if (type === 'boolean') {
    return (
      <ScalarField
        label={label}
        required={required}
        description={property.description}
        value={value}
        schemaType="boolean"
        onChange={onChange}
      />
    );
  }

  if (type === 'integer' || type === 'number') {
    return (
      <ScalarField
        label={label}
        required={required}
        description={property.description}
        value={value}
        schemaType={type}
        onChange={onChange}
      />
    );
  }

  if (!type && !property.enum?.length && !property.properties && !property.items && !property.additionalProperties) {
    if (Array.isArray(value)) {
      return (
        <ArraySchemaField
          realm={realm}
          label={label}
          required={required}
          schema={{ type: 'array', items: inferSchemaFromValue(value[0]) }}
          rootSchema={rootSchema}
          value={value}
          schemas={schemas}
          journey={journey}
          onChange={onChange}
        />
      );
    }
    if (value && typeof value === 'object') {
      return (
        <KeyValueObjectField
          label={label}
          required={required}
          description={property.description}
          value={value}
          onChange={onChange}
        />
      );
    }
  }

  if (type === 'array' || property.items) {
    return (
      <ArraySchemaField
        realm={realm}
        label={label}
        required={required}
        schema={property}
        rootSchema={rootSchema}
        value={value}
        schemas={schemas}
        journey={journey}
        onChange={onChange}
        allowPlaceholder={allowsPlaceholder}
      />
    );
  }

  if (type === 'object' || property.properties || property.additionalProperties) {
    if (allowsPlaceholder) {
      return (
        <div className="rounded-3xl bg-[var(--color-surface-subtle)] p-4 ring-1 ring-[var(--color-border-soft)] lg:col-span-2">
          <PlaceholderValueField
            label={label}
            required={required}
            description={property.description}
            value={typeof value === 'string' ? value : value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {}}
            emptyValue={{}}
            onChange={onChange}
          >
            {(objectValue) => property.properties && Object.keys(property.properties).length > 0 ? (
              <ObjectSchemaField
                realm={realm}
                label="Fields"
                schema={property}
                rootSchema={rootSchema}
                value={objectValue}
                schemas={schemas}
                journey={journey}
                onChange={onChange}
              />
            ) : (
              <KeyValueObjectField
                label="Fields"
                value={objectValue}
                onChange={onChange}
              />
            )}
          </PlaceholderValueField>
        </div>
      );
    }
    if (property.properties && Object.keys(property.properties).length > 0) {
      return (
        <ObjectSchemaField
          realm={realm}
          label={label}
          required={required}
          schema={property}
          rootSchema={rootSchema}
          value={value}
          schemas={schemas}
          journey={journey}
          onChange={onChange}
        />
      );
    }
    return (
      <KeyValueObjectField
        label={label}
        required={required}
        description={property.description}
        value={value}
        onChange={onChange}
      />
    );
  }

  if (property.properties || property.items || property.additionalProperties) {
    return (
      <label className={`${commonLabel} lg:col-span-2`}>
        {label}
        {required ? ' *' : ''}
        <textarea
          value={JSON.stringify(value ?? (type === 'array' ? [] : {}), null, 2)}
          onChange={(event) => {
            try {
              onChange(JSON.parse(event.target.value || (type === 'array' ? '[]' : '{}')));
            } catch {
              onChange(event.target.value);
            }
          }}
          className={`${commonInput} min-h-28 font-mono text-xs`}
        />
        {property.description && (
          <span className="mt-1 block text-xs normal-case tracking-normal text-[var(--color-muted-soft)]">
            {property.description}
          </span>
        )}
      </label>
    );
  }

  return (
    <ScalarField
      label={label}
      required={required}
      description={property.description}
      value={value}
      schemaType="string"
      expression={isScriptableSchema(schema) || isScriptableSchema(property) || isExpressionField(name, property)}
      onChange={onChange}
    />
  );
}

function selectableSourceForJourney(name: string, source: SelectableSource, journeyType?: string): SelectableSource {
  if (name !== 'script_id' || source.resource !== 'scripts') return source;
  const scriptType = normalizeJourneyType(journeyType);
  return { ...source, query: { ...(source.query || {}), type: scriptType } };
}
