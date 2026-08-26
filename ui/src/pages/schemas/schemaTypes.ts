import type { DeveloperSchema } from '../../types/journey'

export type SchemaPropertyType = 'string' | 'integer' | 'number' | 'boolean' | 'array' | 'object'

export type SchemaPropertyDraft = {
  id: string
  name: string
  type: SchemaPropertyType
  required: boolean
  minLength?: string
  maxLength?: string
  format?: string
  stringRule?: string
  pattern?: string
  minimum?: string
  maximum?: string
  enum?: string[]
  minItems?: string
  maxItems?: string
  uniqueItems?: boolean
  minProperties?: string
  maxProperties?: string
  additionalProperties?: boolean
  properties?: SchemaPropertyDraft[]
  items?: SchemaPropertyDraft
}

export type SchemaBuilderDraft = {
  additionalProperties: boolean
  properties: SchemaPropertyDraft[]
}

export type SchemaEditorMode = 'builder' | 'json'

export type SchemaDraft = DeveloperSchema & {
  schema: Record<string, unknown>
}
