import type {
  CacheInstanceInfo,
  DeveloperSchema,
  InstancesResponse,
  JourneyConfiguration,
  JourneyScript,
  QueryResponse,
  ScheduleConfiguration,
  ScheduleRunResult,
  ScriptBindingDescriptor,
  ScriptBindingSet,
  StepSchema,
} from '../types/journey'

const apiBasePath = import.meta.env.VITE_JOURNEY_API_BASE_PATH || '/journey'

export async function listJourneys(realm: string, filters?: { name?: string; type?: string; journeyType?: string; limit?: number }) {
  const params = new URLSearchParams()
  if (filters?.name) params.set('name', filters.name)
  if (filters?.type) params.set('type', filters.type)
  if (filters?.journeyType) params.set('journey_type', filters.journeyType)
  if (filters?.limit) params.set('limit', String(filters.limit))
  return request<QueryResponse<JourneyConfiguration>>(`${apiBasePath}/${encodeURIComponent(realm)}?${params}`)
}

export async function getJourney(realm: string, journeyId: string) {
  return request<JourneyConfiguration>(`${apiBasePath}/${encodeURIComponent(realm)}/${encodeURIComponent(journeyId)}`)
}

export async function saveJourney(realm: string, journey: JourneyConfiguration) {
  return request<JourneyConfiguration>(`${apiBasePath}/${encodeURIComponent(realm)}`, {
    method: 'PUT',
    body: JSON.stringify(journey),
  })
}

export async function deleteJourney(realm: string, journeyId: string) {
  await request<void>(`${apiBasePath}/${encodeURIComponent(realm)}/${encodeURIComponent(journeyId)}`, {
    method: 'DELETE',
  })
}

export async function listScripts(realm: string, filters?: { name?: string; type?: string; limit?: number }) {
  const params = new URLSearchParams()
  if (filters?.name) params.set('name', filters.name)
  if (filters?.type) params.set('type', filters.type)
  if (filters?.limit) params.set('limit', String(filters.limit))
  return request<QueryResponse<JourneyScript>>(`${apiBasePath}/${encodeURIComponent(realm)}/scripts?${params}`)
}

export async function getScript(realm: string, scriptId: string) {
  return request<JourneyScript>(`${apiBasePath}/${encodeURIComponent(realm)}/scripts/${encodeURIComponent(scriptId)}`)
}

export async function saveScript(realm: string, script: JourneyScript) {
  return request<JourneyScript>(`${apiBasePath}/${encodeURIComponent(realm)}/scripts`, {
    method: 'PUT',
    body: JSON.stringify(script),
  })
}

export async function deleteScript(realm: string, scriptId: string) {
  await request<void>(`${apiBasePath}/${encodeURIComponent(realm)}/scripts/${encodeURIComponent(scriptId)}`, {
    method: 'DELETE',
  })
}

export async function listScriptBindings(realm: string, scriptType?: string) {
  const params = new URLSearchParams()
  if (scriptType) params.set('type', scriptType)
  const query = params.toString()
  return request<QueryResponse<ScriptBindingSet>>(`${apiBasePath}/${encodeURIComponent(realm)}/script-bindings${query ? `?${query}` : ''}`)
}

export async function getScriptBindings(realm: string, scriptId: string) {
  return request<QueryResponse<ScriptBindingDescriptor>>(`${apiBasePath}/${encodeURIComponent(realm)}/scripts/${encodeURIComponent(scriptId)}/bindings`)
}

export async function listStepSchemas() {
  return request<QueryResponse<StepSchema>>(`${apiBasePath}/step-schemas`)
}

export async function listDeveloperSchemas(realm: string, filters?: { name?: string; limit?: number }) {
  const params = new URLSearchParams()
  if (filters?.name) params.set('name', filters.name)
  if (filters?.limit) params.set('limit', String(filters.limit))
  const query = params.toString()
  return request<QueryResponse<DeveloperSchema>>(`${apiBasePath}/${encodeURIComponent(realm)}/schemas${query ? `?${query}` : ''}`)
}

export async function getDeveloperSchema(realm: string, schemaId: string) {
  return request<DeveloperSchema>(`${apiBasePath}/${encodeURIComponent(realm)}/schemas/${encodeURIComponent(schemaId)}`)
}

export async function saveDeveloperSchema(realm: string, schema: DeveloperSchema) {
  return request<DeveloperSchema>(`${apiBasePath}/${encodeURIComponent(realm)}/schemas`, {
    method: 'PUT',
    body: JSON.stringify(schema),
  })
}

export async function deleteDeveloperSchema(realm: string, schemaId: string) {
  await request<void>(`${apiBasePath}/${encodeURIComponent(realm)}/schemas/${encodeURIComponent(schemaId)}`, {
    method: 'DELETE',
  })
}

export async function listInstances(realm: string, filters?: { cacheKey?: string; name?: string; limit?: number }) {
  const params = new URLSearchParams()
  if (filters?.cacheKey) params.set('cache_key', filters.cacheKey)
  if (filters?.name) params.set('name', filters.name)
  if (filters?.limit) params.set('limit', String(filters.limit))
  return request<InstancesResponse>(`${apiBasePath}/${encodeURIComponent(realm)}/instances?${params}`)
}

export async function getInstance(realm: string, cacheKey: string, instanceId: string) {
  return request<CacheInstanceInfo>(
    `${apiBasePath}/${encodeURIComponent(realm)}/instances/${encodeURIComponent(cacheKey)}/${encodeURIComponent(instanceId)}`,
  )
}

export async function saveInstance(realm: string, cacheKey: string, instanceId: string, config: unknown) {
  return request<CacheInstanceInfo>(
    `${apiBasePath}/${encodeURIComponent(realm)}/instances/${encodeURIComponent(cacheKey)}/${encodeURIComponent(instanceId)}`,
    {
      method: 'PUT',
      body: JSON.stringify(config),
    },
  )
}

export async function deleteInstance(realm: string, cacheKey: string, instanceId: string) {
  await request<void>(
    `${apiBasePath}/${encodeURIComponent(realm)}/instances/${encodeURIComponent(cacheKey)}/${encodeURIComponent(instanceId)}`,
    { method: 'DELETE' },
  )
}

export async function listSchedules(realm: string, filters?: { name?: string; targetType?: string; limit?: number }) {
  const params = new URLSearchParams()
  if (filters?.name) params.set('name', filters.name)
  if (filters?.targetType) params.set('target_type', filters.targetType)
  if (filters?.limit) params.set('limit', String(filters.limit))
  return request<QueryResponse<ScheduleConfiguration>>(`${apiBasePath}/${encodeURIComponent(realm)}/schedules?${params}`)
}

export async function getSchedule(realm: string, scheduleId: string) {
  return request<ScheduleConfiguration>(`${apiBasePath}/${encodeURIComponent(realm)}/schedules/${encodeURIComponent(scheduleId)}`)
}

export async function saveSchedule(realm: string, schedule: ScheduleConfiguration) {
  return request<ScheduleConfiguration>(`${apiBasePath}/${encodeURIComponent(realm)}/schedules`, {
    method: 'PUT',
    body: JSON.stringify(schedule),
  })
}

export async function deleteSchedule(realm: string, scheduleId: string) {
  await request<void>(`${apiBasePath}/${encodeURIComponent(realm)}/schedules/${encodeURIComponent(scheduleId)}`, { method: 'DELETE' })
}

export async function triggerSchedule(realm: string, scheduleId: string, wait = true) {
  return request<ScheduleRunResult>(`${apiBasePath}/${encodeURIComponent(realm)}/schedules/${encodeURIComponent(scheduleId)}/trigger`, {
    method: 'POST',
    body: JSON.stringify({ wait }),
  })
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: { Accept: 'application/json', 'Content-Type': 'application/json', ...init?.headers },
  })
  if (!response.ok) {
    const text = await response.text()
    throw new Error(text || `Request failed with ${response.status}`)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}
