import type { CacheInfo } from '../../types/journey';

type JSONSchemaLike = {
  type?: string | string[];
  default?: unknown;
  properties?: Record<string, JSONSchemaLike>;
  required?: string[];
  items?: JSONSchemaLike;
  anyOf?: JSONSchemaLike[];
  oneOf?: JSONSchemaLike[];
  allOf?: JSONSchemaLike[];
};

export function stringifyConfig(value: unknown) {
  if (value === undefined || value === null) return '{}';
  return JSON.stringify(value, null, 2);
}

export function defaultConfigText(cache?: CacheInfo) {
  return JSON.stringify(configDraftFromSchema(cache?.schema), null, 2);
}

export function isGeneratedTemplate(value: string, caches: CacheInfo[] = []) {
  if (isEmptyJSON(value)) return true;
  return caches.some((cache) => jsonTextEquals(value, defaultConfigText(cache)));
}

export function formatBytes(value: number) {
  if (!Number.isFinite(value) || value <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  let amount = value;
  let unit = 0;
  while (amount >= 1024 && unit < units.length - 1) {
    amount /= 1024;
    unit++;
  }
  return `${amount.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
}

export function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

function isEmptyJSON(value: string) {
  const cleaned = value.trim();
  return cleaned === '' || cleaned === '{}';
}

function jsonTextEquals(left: string, right: string) {
  try {
    return JSON.stringify(JSON.parse(left || '{}')) === JSON.stringify(JSON.parse(right || '{}'));
  } catch {
    return false;
  }
}

function configDraftFromSchema(schema?: Record<string, unknown> | null): unknown {
  const draft = draftValue(schema as JSONSchemaLike | undefined);
  return isPlainObject(draft) ? draft : {};
}

function draftValue(schema?: JSONSchemaLike): unknown {
  if (!schema) return {};
  if (schema.default !== undefined) return schema.default;

  const merged = mergeCompositions(schema);
  if (merged !== schema) return draftValue(merged);

  if (schema.anyOf?.length || schema.oneOf?.length) {
    return draftValue(firstObjectSchema(schema.anyOf || schema.oneOf || []) || schema.anyOf?.[0] || schema.oneOf?.[0]);
  }

  const type = primaryType(schema.type);
  if (type === 'object' || schema.properties) {
    const result: Record<string, unknown> = {};
    for (const [key, child] of Object.entries(schema.properties || {})) {
      result[key] = draftValue(child);
    }
    return result;
  }
  if (type === 'array') return [];
  if (type === 'boolean') return false;
  if (type === 'integer' || type === 'number') return 0;
  if (type === 'string') return '';
  return {};
}

function mergeCompositions(schema: JSONSchemaLike): JSONSchemaLike {
  if (!schema.allOf?.length) return schema;
  return schema.allOf.reduce<JSONSchemaLike>((merged, item) => ({
    ...merged,
    ...item,
    properties: { ...(merged.properties || {}), ...(item.properties || {}) },
    required: [...(merged.required || []), ...(item.required || [])],
  }), { ...schema, allOf: undefined });
}

function firstObjectSchema(options: JSONSchemaLike[]) {
  return options.find((option) => primaryType(option.type) === 'object' || option.properties);
}

function primaryType(type?: string | string[]) {
  if (Array.isArray(type)) return type.find((item) => item !== 'null') || type[0];
  return type;
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}
