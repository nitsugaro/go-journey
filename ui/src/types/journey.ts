export type QueryResponse<T> = {
  result: T[]
  resultCount: number
}

export type ScriptMetadata = {
  id?: string
  rev?: string
  created_at?: string
  modified_at?: string
}

export type JourneyScript = ScriptMetadata & {
  name: string
  type: 'auth' | 'resource' | 'workflow' | 'schedule' | 'library' | 'async' | string
  code_base64: string
  args?: ScriptArgument[]
  additional_props?: {
    outcomes?: string[]
    [key: string]: unknown
  }
}

export type ScriptArgument = {
  id: string
  type: 'string' | 'bool' | 'int' | 'float' | 'list' | 'object' | string
  enum?: string[]
}

export type ScriptBindingDescriptor = {
  name: string
  type: string
  signature?: string
  description?: string
  example?: string
  children?: ScriptBindingDescriptor[]
}

export type ScriptBindingSet = {
  type: string
  name?: string
  description?: string
  runnable?: boolean
  bindings: ScriptBindingDescriptor[]
}

export type JourneyConfiguration = {
  id?: string
  name: string
  description?: string
  encrypted_client_inputs?: boolean
  confidential?: boolean
  realm?: string
  active?: boolean
  debug?: boolean
  journey_type?: 'auth' | 'resource' | 'workflow' | string
  default_exp?: number
  start_step_id: string
  sub_entries?: string[] | null
  steps: Record<string, JourneyStep>
  additional_properties?: Record<string, unknown> | null
}

export type JourneyStep = {
  name?: string
  step_type: string
  config?: Record<string, unknown>
}

export type StepSchema = {
  step_type: string
  schema: unknown
}

export type DeveloperSchema = ScriptMetadata & {
  name: string
  description?: string
  realm?: string
  draft?: string
  schema: Record<string, unknown>
}

export type StepOutcome = {
  name: string
  target: string
}

export type CacheInfo = {
  key: string
  max_instances: number
  max_size_bytes: number
  instances: number
  size_bytes: number
  persistable: boolean
  user_configurable?: boolean
  description?: string
  schema?: Record<string, unknown> | null
}

export type CacheInstanceInfo = {
  cache_key: string
  instance_id: string
  config?: unknown
  persisted: boolean
  runtime: boolean
  size_bytes: number
}

export type InstancesResponse = QueryResponse<CacheInstanceInfo> & {
  caches: CacheInfo[]
}

export type ScheduleConfiguration = ScriptMetadata & {
  name: string
  description?: string
  realm?: string
  active?: boolean
  kind: 'cron' | 'interval' | string
  cron?: string
  interval_seconds?: number
  start_at?: number
  timezone?: string
  max_runs?: number
  trigger_enabled?: boolean
  trigger_wait?: boolean
  timeout_seconds?: number
  target: ScheduleTarget
  run_count?: number
  last_run_at?: number
  next_run_at?: number
  status?: string
  last_error?: string
  running?: boolean
}

export type ScheduleTarget = {
  type: 'script' | 'workflow' | string
  script_id?: string
  script_type?: string
  args?: Record<string, unknown>
  journey_id?: string
  initial_data?: Record<string, unknown>
}

export type ScheduleRunResult = {
  schedule_id?: string
  status: string
  error?: string
  value?: unknown
  has_value?: boolean
  started_at?: number
  finished_at?: number
  run_count?: number
}
