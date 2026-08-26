import type { JourneyStep, ScriptArgument, StepSchema } from '../../../types/journey';
import { journeyTypes } from '../flowConstants';
import type { JSONSchema } from '../flowTypes';
import { normalizeJourneyType as normalizeConfiguredJourneyType } from '../../../config/typeOptions';

export function isDynamicOutcomeSchema(schema: JSONSchema) {
  return schema['x-dynamic-outcome'] === true || Object.keys(unwrapSchema(schema).properties || {}).length === 0;
}

export function isExpressionField(name: string, schema: JSONSchema) {
  const haystack = `${name} ${schema.title || ''} ${schema.description || ''}`.toLowerCase();
  return haystack.includes('expression') || haystack.includes('gval') || haystack.includes('go expression');
}

export function isScriptableSchema(schema: JSONSchema) {
  return schema['x-type'] === 'scriptable';
}

export function stepSchemaDescription(schema: StepSchema) {
  const schemaObject = asSchema(schema.schema);
  const direct = schemaObject?.description || schemaObject?.title;
  if (direct) return direct;
  const hiddenDescription = asSchema(schemaObject?.properties?._)?.description;
  if (hiddenDescription) return hiddenDescription;
  return 'Configure and execute this journey step.';
}

export function normalizeJourneyType(journeyType?: string) {
  const normalized = normalizeConfiguredJourneyType(journeyType);
  return journeyTypes.includes(normalized as (typeof journeyTypes)[number])
    ? (normalized as (typeof journeyTypes)[number])
    : 'auth';
}

export function stepSchemaSupportsJourneyType(schema: StepSchema, journeyType: string) {
  const schemaObject = asSchema(schema.schema);
  const flowTypes = schemaObject?.['x-flow-type'];
  const normalizedJourneyType = normalizeJourneyType(journeyType);
  if (!Array.isArray(flowTypes) || flowTypes.length === 0) return normalizedJourneyType === 'auth';
  return flowTypes.some((flowType) => normalizeJourneyType(String(flowType)) === normalizedJourneyType);
}

export function stepSchemaOutcomesByType(schemas: StepSchema[]) {
  const outcomes = new Map<string, string[]>();
  for (const schema of schemas) {
    const outcomeSchema = asSchema(schema.schema)?.properties?.outcome;
    outcomes.set(schema.step_type, outcomeSchema ? staticOutcomeNamesFromSchema(outcomeSchema) : []);
  }
  return outcomes;
}

export function stepSchemaDynamicOutcomeTypes(schemas: StepSchema[]) {
  const types = new Set<string>();
  for (const schema of schemas) {
    const outcomeSchema = asSchema(schema.schema)?.properties?.outcome;
    if (outcomeSchema && isDynamicOutcomeSchema(outcomeSchema)) {
      types.add(schema.step_type);
    }
  }
  return types;
}

export function staticOutcomeNamesFromSchema(schema: JSONSchema) {
  return Object.keys(unwrapSchema(schema).properties || {});
}

export function defaultConfigForSchema(schema?: StepSchema) {
  const schemaObject = asSchema(schema?.schema);
  const config: Record<string, unknown> = {};
  for (const [key, property] of orderedSchemaProperties(schemaObject)) {
    if (key === 'outcome') {
      config.outcome = {};
      continue;
    }
    const defaultValue = defaultValueForSchema(property);
    if (defaultValue !== undefined) config[key] = defaultValue;
  }
  return config;
}

export function defaultValueForSchema(schema: JSONSchema): unknown {
  const property = unwrapSchema(schema);
  if (property.default !== undefined) return property.default;
  if (property.enum?.length) return property.enum[0];
  const type = schemaType(property);
  if (type === 'boolean') return false;
  if (type === 'integer' || type === 'number') return property.minimum ?? 0;
  if (type === 'array') return [];
  if (type === 'object' || property.properties || property.additionalProperties) return {};
  return undefined;
}

export function orderedSchemaProperties(schema: JSONSchema | null, rootSchema?: JSONSchema | null) {
  const resolved = schema ? unwrapSchema(schema, rootSchema || schema) : null;
  const properties = resolved?.properties || {};
  const order = resolved?.['x-order'] || [];
  return Object.entries(properties).sort(([left], [right]) => {
    const leftIndex = order.indexOf(left);
    const rightIndex = order.indexOf(right);
    if (leftIndex >= 0 || rightIndex >= 0) {
      return (
        (leftIndex >= 0 ? leftIndex : Number.MAX_SAFE_INTEGER) -
        (rightIndex >= 0 ? rightIndex : Number.MAX_SAFE_INTEGER)
      );
    }
    if (left === 'outcome') return 1;
    if (right === 'outcome') return -1;
    return left.localeCompare(right);
  });
}

export function asSchema(value: unknown): JSONSchema | null {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as JSONSchema) : null;
}

export function unwrapSchema(schema: JSONSchema, rootSchema: JSONSchema = schema): JSONSchema {
  const resolved = resolveSchemaReference(schema, rootSchema);
  const candidates = resolved.anyOf || resolved.oneOf || resolved.allOf;
  if (!candidates?.length) return resolved;
  const best = [...candidates].sort(
    (left, right) => schemaSpecificity(right, rootSchema) - schemaSpecificity(left, rootSchema)
  )[0];
  return best ? mergeSchemaUIMetadata(unwrapSchema(best, rootSchema), resolved) : resolved;
}

export function schemaChildOverride(schema: JSONSchema, key: string): JSONSchema | null {
  return asSchema((schema as Record<string, unknown>)[key]);
}

export function mergeSchemaUIMetadata(schema: JSONSchema, source?: JSONSchema | null): JSONSchema {
  if (!source) return schema;
  const metadata = Object.fromEntries(Object.entries(source).filter(([key]) => key.startsWith('x-')));
  if (Object.keys(metadata).length === 0) return schema;
  return { ...schema, ...metadata };
}

export function resolveSchemaReference(schema: JSONSchema, rootSchema: JSONSchema): JSONSchema {
  if (!schema.$ref?.startsWith('#/')) return schema;
  const path = schema.$ref
    .slice(2)
    .split('/')
    .map((part) => part.replace(/~1/g, '/').replace(/~0/g, '~'));
  let current: unknown = rootSchema;
  for (const part of path) {
    if (!current || typeof current !== 'object' || Array.isArray(current)) return schema;
    current = (current as Record<string, unknown>)[part];
  }
  return asSchema(current) || schema;
}

export function schemaSpecificity(schema: JSONSchema, rootSchema: JSONSchema) {
  let score = 0;
  const resolved = resolveSchemaReference(schema, rootSchema);
  const type = schemaType(resolved);
  if (schema.$ref) score += 120;
  if (resolved.properties && Object.keys(resolved.properties).length > 0) score += 100;
  if (resolved.items) score += 80;
  if (resolved.additionalProperties && typeof resolved.additionalProperties === 'object') score += 60;
  if (resolved.enum?.length) score += 40;
  if (type === 'object') score += 30;
  if (type === 'array') score += 25;
  if (type === 'boolean' || type === 'integer' || type === 'number') score += 20;
  if (type === 'string') score += 5;
  if (isPlaceholderSchema(resolved)) score -= 100;
  return score;
}

export function isPlaceholderSchema(schema: JSONSchema) {
  const pattern = (schema as Record<string, unknown>).pattern;
  return (
    schemaType(schema) === 'string' &&
    typeof pattern === 'string' &&
    (pattern.includes('\\$\\{') || pattern.includes('${'))
  );
}

export function inferSchemaFromValue(value: unknown): JSONSchema {
  if (Array.isArray(value)) return { type: 'array', items: inferSchemaFromValue(value[0]) };
  if (value && typeof value === 'object') {
    return {
      type: 'object',
      properties: Object.fromEntries(
        Object.keys(value as Record<string, unknown>).map((key) => [
          key,
          inferSchemaFromValue((value as Record<string, unknown>)[key]),
        ])
      ),
    };
  }
  if (typeof value === 'boolean') return { type: 'boolean' };
  if (typeof value === 'number') return Number.isInteger(value) ? { type: 'integer' } : { type: 'number' };
  return { type: 'string' };
}

export function schemaType(schema: JSONSchema) {
  const type = schema.type;
  if (Array.isArray(type)) return type.find((item) => item !== 'null') || type[0];
  return type;
}

export function isEmbeddedStepsField(name: string, schema: JSONSchema, rootSchema: JSONSchema = schema) {
  const property = unwrapSchema(schema, rootSchema);
  const items = property.items ? unwrapSchema(property.items, rootSchema) : null;
  return (
    name === 'steps' &&
    (schemaType(property) === 'array' || property.items) &&
    Boolean(items?.properties?.step_type || items?.properties?.config)
  );
}

export function normalizeEmbeddedStep(value: unknown): JourneyStep {
  const record = asRecord(value);
  return {
    name: typeof record.name === 'string' ? record.name : '',
    step_type: typeof record.step_type === 'string' ? record.step_type : '',
    config: asRecord(record.config),
  };
}

export function parseConfigText(value: string): Record<string, unknown> {
  try {
    const parsed = JSON.parse(value || '{}');
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? (parsed as Record<string, unknown>) : {};
  } catch {
    return {};
  }
}

export function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
}

export function formatEditableValue(value: unknown) {
  if (value === undefined || value === null) return '';
  if (typeof value === 'string') return value;
  return JSON.stringify(value);
}

export function parseEditableValue(value: string): unknown {
  const trimmed = value.trim();
  if (trimmed === '') return '';
  if (trimmed === 'true') return true;
  if (trimmed === 'false') return false;
  if (trimmed === 'null') return null;
  if (!Number.isNaN(Number(trimmed)) && /^-?\d+(\.\d+)?$/.test(trimmed)) return Number(trimmed);
  if ((trimmed.startsWith('{') && trimmed.endsWith('}')) || (trimmed.startsWith('[') && trimmed.endsWith(']'))) {
    try {
      return JSON.parse(trimmed);
    } catch {
      return value;
    }
  }
  return value;
}

export function objectRecord(value: unknown): Record<string, string> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {};
  return Object.fromEntries(
    Object.entries(value as Record<string, unknown>)
      .filter(([, target]) => typeof target === 'string')
      .map(([key, target]) => [key, target as string])
  );
}


export function schemaForScriptArgument(arg: ScriptArgument): JSONSchema {
  const enumValues = (arg.enum || []).map((value) => String(value)).filter(Boolean);
  const base: JSONSchema = {
    title: arg.id,
    description: `Script arg ${arg.id}`,
    ...(enumValues.length > 0 ? { enum: enumValues } : {}),
  };
  switch (arg.type) {
    case 'int':
      return { ...base, type: 'integer' };
    case 'float':
      return { ...base, type: 'number' };
    case 'bool':
      return { ...base, type: 'boolean' };
    case 'object':
      return { ...base, type: 'object', additionalProperties: true };
    case 'list':
      return { ...base, type: 'array', items: { type: 'string' } };
    default:
      return { ...base, type: 'string' };
  }
}
