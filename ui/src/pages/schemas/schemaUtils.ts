import type { DeveloperSchema } from '../../types/journey'
import type { SchemaBuilderDraft, SchemaDraft, SchemaPropertyDraft, SchemaPropertyType } from './schemaTypes'

export const schemaTypes: SchemaPropertyType[] = ['string', 'integer', 'number', 'boolean', 'array', 'object']

export const schemaStringFormats = ['', 'email', 'uri', 'uuid', 'date', 'date-time', 'hostname', 'ipv4', 'ipv6'] as const

export const schemaStringRules = [
  { value: '', label: 'Any string', pattern: '' },
  { value: 'digits', label: 'Digits', pattern: '^[0-9]+$' },
  { value: 'letters-lower', label: 'Lowercase letters', pattern: '^[a-z]+$' },
  { value: 'letters-upper', label: 'Uppercase letters', pattern: '^[A-Z]+$' },
  { value: 'letters', label: 'Letters', pattern: '^[A-Za-z]+$' },
  { value: 'custom', label: 'Custom pattern', pattern: '' },
] as const

export function newDeveloperSchema(): SchemaDraft {
  return {
    name: '',
    description: '',
    draft: 'draft-07',
    schema: builderToSchema({ additionalProperties: false, properties: [] }),
  }
}

export function schemaToBuilder(schema: Record<string, unknown>): SchemaBuilderDraft {
  return {
    additionalProperties: schema.additionalProperties === true,
    properties: schemaPropertiesToDraft(schema),
  }
}

export function builderToSchema(builder: SchemaBuilderDraft) {
  const properties: Record<string, unknown> = {}
  const required: string[] = []
  for (const field of builder.properties) {
    const name = field.name.trim()
    if (!name) continue
    if (field.required) required.push(name)
    properties[name] = propertyToSchema(field)
  }
  return {
    $schema: 'http://json-schema.org/draft-07/schema#',
    type: 'object',
    additionalProperties: builder.additionalProperties,
    properties,
    ...(required.length ? { required } : {}),
  }
}

export function parseSchemaJSON(value: string) {
  const parsed = JSON.parse(value || '{}')
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('Schema JSON must be an object.')
  }
  return parsed as Record<string, unknown>
}

export function schemaSearchText(schema: DeveloperSchema) {
  return `${schema.name || ''} ${schema.description || ''} ${schema.id || ''}`.toLowerCase()
}

function propertyToSchema(field: SchemaPropertyDraft) {
  const schema: Record<string, unknown> = { type: field.type }
  if (field.type === 'array') schema.items = field.items ? propertyToSchema(field.items) : { type: 'string' }
  if (field.type === 'array') {
    setNumeric(schema, 'minItems', field.minItems)
    setNumeric(schema, 'maxItems', field.maxItems)
    if (field.uniqueItems === true) schema.uniqueItems = true
  }
  if (field.type === 'object') {
    const nestedProperties: Record<string, unknown> = {}
    const required: string[] = []
    for (const child of field.properties || []) {
      const name = child.name.trim()
      if (!name) continue
      if (child.required) required.push(name)
      nestedProperties[name] = propertyToSchema(child)
    }
    schema.additionalProperties = field.additionalProperties === true
    schema.properties = nestedProperties
    setNumeric(schema, 'minProperties', field.minProperties)
    setNumeric(schema, 'maxProperties', field.maxProperties)
    if (required.length) schema.required = required
  }
  if (field.type === 'string') {
    setNumeric(schema, 'minLength', field.minLength)
    setNumeric(schema, 'maxLength', field.maxLength)
    if (field.format?.trim()) schema.format = field.format.trim()
    const pattern = patternForStringRule(field)
    if (pattern) schema.pattern = pattern
    const values = (field.enum || []).map((item) => item.trim()).filter(Boolean)
    if (values.length) schema.enum = values
  }
  if (field.type === 'integer' || field.type === 'number') {
    setNumeric(schema, 'minimum', field.minimum)
    setNumeric(schema, 'maximum', field.maximum)
  }
  return schema
}

function schemaPropertiesToDraft(schema: Record<string, unknown>) {
  const properties = asRecord(schema.properties)
  const required = new Set(Array.isArray(schema.required) ? schema.required.map(String) : [])
  return Object.entries(properties).map(([name, value]) => propertySchemaToDraft(name, asRecord(value), required.has(name)))
}

function propertySchemaToDraft(name: string, property: Record<string, unknown>, required: boolean): SchemaPropertyDraft {
  const type = normalizeType(property.type)
  return {
    id: crypto.randomUUID(),
    name,
    type,
    required,
    minLength: stringNumber(property.minLength),
    maxLength: stringNumber(property.maxLength),
    format: typeof property.format === 'string' ? property.format : '',
    stringRule: stringRuleFromPattern(typeof property.pattern === 'string' ? property.pattern : ''),
    pattern: typeof property.pattern === 'string' ? property.pattern : '',
    minimum: stringNumber(property.minimum),
    maximum: stringNumber(property.maximum),
    enum: type === 'string' && Array.isArray(property.enum) ? property.enum.map(String) : [],
    minItems: stringNumber(property.minItems),
    maxItems: stringNumber(property.maxItems),
    uniqueItems: type === 'array' ? property.uniqueItems === true : undefined,
    minProperties: stringNumber(property.minProperties),
    maxProperties: stringNumber(property.maxProperties),
    additionalProperties: type === 'object' ? property.additionalProperties === true : undefined,
    properties: type === 'object' ? schemaPropertiesToDraft(property) : undefined,
    items: type === 'array' ? propertySchemaToDraft('item', asRecord(property.items), false) : undefined,
  }
}

function setNumeric(target: Record<string, unknown>, key: string, value?: string) {
  if (value === undefined || value.trim() === '') return
  const parsed = Number(value)
  if (Number.isFinite(parsed)) target[key] = parsed
}

function normalizeType(value: unknown): SchemaPropertyType {
  const type = Array.isArray(value) ? value.find((item) => item !== 'null') : value
  return schemaTypes.includes(type as SchemaPropertyType) ? (type as SchemaPropertyType) : 'string'
}

function stringNumber(value: unknown) {
  return typeof value === 'number' ? String(value) : ''
}

function stringRuleFromPattern(pattern: string) {
  const found = schemaStringRules.find((rule) => rule.pattern && rule.pattern === pattern)
  if (found) return found.value
  return pattern ? 'custom' : ''
}

function patternForStringRule(field: SchemaPropertyDraft) {
  const rule = schemaStringRules.find((item) => item.value === field.stringRule)
  if (rule?.pattern) return rule.pattern
  if (field.stringRule === 'custom' && field.pattern?.trim()) return field.pattern.trim()
  if (!field.stringRule && field.pattern?.trim()) return field.pattern.trim()
  return ''
}

function asRecord(value: unknown) {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : {}
}
