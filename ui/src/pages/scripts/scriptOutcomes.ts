import { normalizeScriptType } from '../../config/typeOptions'

export function supportsScriptOutcomes(type?: string) {
	return ['auth', 'resource', 'workflow'].includes(normalizeScriptType(type))
}

export function normalizeScriptOutcomes(values: unknown): string[] {
  if (!Array.isArray(values)) return []
  const result: string[] = []
  const seen = new Set<string>()
  for (const raw of values) {
    if (typeof raw !== 'string') continue
    const value = raw.trim().toLowerCase()
    if (!value || seen.has(value)) continue
    seen.add(value)
    result.push(value)
  }
  return result
}

export function scriptOutcomes(script?: { type?: string; additional_props?: { outcomes?: string[] } }) {
  return supportsScriptOutcomes(script?.type)
    ? normalizeScriptOutcomes(script?.additional_props?.outcomes)
    : []
}

export function syncScriptOutcomeTargets(raw: unknown, declared: string[]) {
  const current = raw && typeof raw === 'object' && !Array.isArray(raw)
    ? raw as Record<string, unknown>
    : {}
  const next: Record<string, string> = {}
  for (const outcome of declared) {
    const existing = Object.entries(current).find(([name]) => name.trim().toLowerCase() === outcome)?.[1]
    next[outcome] = typeof existing === 'string' ? existing : ''
  }
  return next
}
