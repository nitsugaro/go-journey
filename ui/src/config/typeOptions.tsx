import type { LucideIcon } from 'lucide-react'
import { Boxes, CalendarClock, FileCode2, Globe2, Library, ShieldCheck, Timer, Workflow } from 'lucide-react'
import type { ScriptBindingSet } from '../types/journey'

export type TypeOption = {
  value: string
  title: string
  description: string
  icon: LucideIcon
  group?: string
}

export const journeyTypeOptions: TypeOption[] = [
  {
    value: 'auth',
    title: 'Auth journey',
    description: 'Interactive authentication flow with client inputs, sessions and journey tokens.',
    icon: ShieldCheck,
    group: 'Identity',
  },
  {
    value: 'resource',
    title: 'Resource journey',
    description: 'Validates or transforms resource requests and returns an HTTP response.',
    icon: Globe2,
    group: 'HTTP',
  },
  {
    value: 'workflow',
    title: 'Workflow journey',
    description: 'Backend workflow for schedules and orchestration. No external request or client interaction.',
    icon: Workflow,
    group: 'Backend',
  },
]

export const scriptTypeOptions: TypeOption[] = [
  {
    value: 'auth',
    title: 'Auth script',
    description: 'Runs inside auth journey steps with context, outcome and journey bindings.',
    icon: FileCode2,
    group: 'Runtime',
  },
  {
    value: 'resource',
    title: 'Resource script',
    description: 'Runs for resource journeys with request-oriented bindings.',
    icon: Boxes,
    group: 'Runtime',
  },
  {
    value: 'workflow',
    title: 'Workflow script',
    description: 'Runs inside workflow journeys with backend-safe context, HTTP, crypto and logger bindings.',
    icon: Workflow,
    group: 'Runtime',
  },
  {
    value: 'schedule',
    title: 'Schedule script',
    description: 'Runs directly from scheduler jobs with args, HTTP, crypto and logger bindings.',
    icon: CalendarClock,
    group: 'Runtime',
  },
  {
    value: 'library',
    title: 'Library script',
    description: 'Reusable JavaScript helpers imported by other scripts.',
    icon: Library,
    group: 'Shared',
  },
  {
    value: 'async',
    title: 'Async script',
    description: 'Script definition reserved for asynchronous usage without default runtime bindings.',
    icon: Timer,
    group: 'Shared',
  },
]

export function scriptTypeOptionsFromBindingSets(sets: ScriptBindingSet[] = []): TypeOption[] {
  const byType = new Map(scriptTypeOptions.map((option) => [option.value, option]))
  for (const set of sets) {
    if (!set.type) continue
    const type = normalizeScriptType(set.type)
    const existing = byType.get(type)
    byType.set(type, {
      value: type,
      title: set.name || existing?.title || set.type,
      description: set.description || existing?.description || 'Custom script type provided by the host application.',
      icon: existing?.icon || FileCode2,
      group: existing?.group || (set.runnable === false ? 'Shared' : 'Custom'),
    })
  }
  return Array.from(byType.values())
}

export function normalizeJourneyType(value?: string) {
  const normalized = String(value || '').trim().toLowerCase()
  if (normalized === 'auth-journey' || normalized === 'journey') return 'auth'
  if (normalized === 'resource-journey' || normalized === 'proxy-journey') return 'resource'
  if (normalized === 'workflow-journey') return 'workflow'
  if (normalized === 'resource' || normalized === 'workflow') return normalized
  return 'auth'
}

export function normalizeScriptType(value?: string) {
  const normalized = String(value || '').trim().toLowerCase()
  if (normalized === 'journey') return 'auth'
  return normalized || 'auth'
}
