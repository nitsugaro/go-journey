import { selectableAPIBasePath } from '../flowConstants';
import type { JSONSchema, SelectableSource } from '../flowTypes';
import { asRecord } from './schemaUtils';

export function selectableSource(schema: JSONSchema): SelectableSource | null {
  if (schema['x-type'] !== 'selectable') return null;
  const props = schema['x-props'] || {};
  const nameProperty = stringProp(props, 'nameProperty') || stringProp(props, 'labelProperty') || 'name';
  const valueProperty = stringProp(props, 'valueProperty') || 'id';
  const resource = stringProp(props, 'resource');
  const endpoint = stringProp(props, 'endpoint');
  const query = recordProp(props, 'query');
  if (!resource && !endpoint) return null;
  return { resource, endpoint, query, nameProperty, valueProperty };
}

export async function fetchSelectableOptions(
  realm: string,
  source: SelectableSource,
  filters: { name?: string; limit?: number } = {}
) {
  const response = await fetch(selectableURL(realm, source, filters), {
    headers: { Accept: 'application/json' },
  });
  if (!response.ok) {
    throw new Error(`Cannot load options (${response.status})`);
  }
  const payload = (await response.json()) as unknown;
  const result = Array.isArray(payload)
    ? payload
    : Array.isArray((payload as Record<string, unknown>)?.result)
    ? ((payload as Record<string, unknown>).result as unknown[])
    : [];
  const rawCount = !Array.isArray(payload) ? (payload as Record<string, unknown>)?.resultCount : undefined;
  const resultCount = typeof rawCount === 'number' ? rawCount : result.length;

  const options = result
    .map((item) => {
      const record = asRecord(item);
      const value = nestedValue(record, source.valueProperty);
      const label = nestedValue(record, source.nameProperty);
      if (value === undefined || value === null) return null;
      return {
        value: String(value),
        label: label === undefined || label === null || String(label).trim() === '' ? String(value) : String(label),
      };
    })
    .filter((item): item is { label: string; value: string } => Boolean(item));

  return { options, resultCount };
}

export function selectableURL(realm: string, source: SelectableSource, filters: { name?: string; limit?: number } = {}) {
  const url = source.endpoint
    ? source.endpoint
    : source.resource === 'scripts'
    ? `${selectableAPIBasePath}/:realm/scripts`
    : source.resource === 'schemas'
    ? `${selectableAPIBasePath}/:realm/schemas`
    : `${selectableAPIBasePath}/:realm`;

  const [path, existingQuery = ''] = url.replaceAll(':realm', encodeURIComponent(realm)).split('?');
  const params = new URLSearchParams(existingQuery);
  for (const [key, value] of Object.entries(source.query || {})) {
    if (value !== undefined && value !== null && String(value).trim() !== '') {
      params.set(key, String(value));
    }
  }
  if (filters.name?.trim()) params.set('name', filters.name.trim());
  if (filters.limit && filters.limit > 0) params.set('limit', String(filters.limit));
  const query = params.toString();
  return query ? `${path}?${query}` : path;
}

export function nestedValue(record: Record<string, unknown>, path: string) {
  return path.split('.').reduce<unknown>((current, part) => {
    if (!current || typeof current !== 'object' || Array.isArray(current)) return undefined;
    return (current as Record<string, unknown>)[part];
  }, record);
}

export function stringProp(record: Record<string, unknown>, key: string) {
  const value = record[key];
  return typeof value === 'string' ? value.trim() : '';
}

export function recordProp(record: Record<string, unknown>, key: string) {
  const value = record[key];
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : undefined;
}
