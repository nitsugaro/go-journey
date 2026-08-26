import type { JourneyConfiguration, JourneyStep, StepOutcome } from '../../../types/journey'
import type { JourneyStepNodeData } from '../JourneyStepNode'

export const nodeWidth = 320
export const nodeBaseHeight = 116
export const outcomeHeight = 38
export const canvasPadding = { x: 120, y: 120 }
export const longReferenceDistance = 700
export const referenceWidth = 288
export const referenceHeight = 122
export const referenceCollisionGap = 28
export const columnStride = nodeWidth + 150

export type StepColor = {
  stroke: string
  soft: string
  text: string
}

export const stepPalette: StepColor[] = [
  { stroke: 'var(--color-node1-stroke)', soft: 'var(--color-node1-soft)', text: 'var(--color-node1-text)' },
  { stroke: 'var(--color-node2-stroke)', soft: 'var(--color-node2-soft)', text: 'var(--color-node2-text)' },
  { stroke: 'var(--color-node3-stroke)', soft: 'var(--color-node3-soft)', text: 'var(--color-node3-text)' },
  { stroke: 'var(--color-node4-stroke)', soft: 'var(--color-node4-soft)', text: 'var(--color-node4-text)' },
  { stroke: 'var(--color-node5-stroke)', soft: 'var(--color-node5-soft)', text: 'var(--color-node5-text)' },
  { stroke: 'var(--color-node6-stroke)', soft: 'var(--color-node6-soft)', text: 'var(--color-node6-text)' },
  { stroke: 'var(--color-node7-stroke)', soft: 'var(--color-node7-soft)', text: 'var(--color-node7-text)' },
  { stroke: 'var(--color-node8-stroke)', soft: 'var(--color-node8-soft)', text: 'var(--color-node8-text)' },
  { stroke: 'var(--color-node9-stroke)', soft: 'var(--color-node9-soft)', text: 'var(--color-node9-text)' },
  { stroke: 'var(--color-node10-stroke)', soft: 'var(--color-node10-soft)', text: 'var(--color-node10-text)' },
  { stroke: 'var(--color-node11-stroke)', soft: 'var(--color-node11-soft)', text: 'var(--color-node11-text)' },
  { stroke: 'var(--color-node12-stroke)', soft: 'var(--color-node12-soft)', text: 'var(--color-node12-text)' },
]

export type ReferenceSpec = {
  id: string
  source: string
  outcomes: string[]
  target: string
  crossThread: boolean
  selfCall: boolean
}

export type FlowActions = Pick<JourneyStepNodeData, 'onSetStart' | 'onConfigure' | 'onOpenNotes' | 'onConnect' | 'onBreak' | 'onAddStep' | 'onDeleteBranch' | 'onOpenSubJourney' | 'connectingOutcome'>

export type FlowOptions = {
  endJourneyTypes?: Set<string>
  highlightedStepID?: string
  staticOutcomesByStepType?: Map<string, string[]>
  staticOutcomesByStepID?: Map<string, string[]>
  dynamicOutcomeStepTypes?: Set<string>
  noteCountsByStep?: Map<string, number>
}

export type Position = { x: number; y: number }

export function referenceKey(source: string, outcome: string, target: string) {
  return `${source}\u0000${outcome}\u0000${target}`
}

export function outcomeHandleID(outcome: string, target: string) {
  return `outcome-${safeHandlePart(outcome)}-${safeHandlePart(target)}`
}

export function configString(config: Record<string, unknown> | undefined, key: string) {
  const value = config?.[key]
  return typeof value === 'string' ? value.trim() : ''
}

export function extractOutcomes(step: JourneyStep, staticOutcomeNames: string[] = [], strictStatic = false): StepOutcome[] {
  const raw = step.config?.outcome
  const outcomes = new Map<string, string>()
  const canonical = new Map<string, string>()
  for (const rawName of staticOutcomeNames) {
    const name = strictStatic ? rawName.trim().toLowerCase() : rawName
    if (!name) continue
    if (strictStatic) canonical.set(name, name)
    outcomes.set(name, '')
  }
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) {
    return [...outcomes.entries()].map(([name, target]) => ({ name, target }))
  }
  for (const [name, target] of Object.entries(raw as Record<string, unknown>).filter(([, target]) => typeof target === 'string')) {
    const normalized = strictStatic ? name.trim().toLowerCase() : name
    if (!normalized) continue
    if (strictStatic && !canonical.has(normalized)) continue
    outcomes.set(canonical.get(normalized) || normalized, target as string)
  }
  return [...outcomes.entries()].map(([name, target]) => ({ name, target }))
}

export function sortOutcomesByVisualTarget(
  outcomes: StepOutcome[],
  source: string,
  positions: Map<string, Position>,
  referenceByEdge: Map<string, ReferenceSpec>,
) {
  return [...outcomes].sort((left, right) => {
    const leftTarget = referenceByEdge.get(referenceKey(source, left.name, left.target))?.id || left.target
    const rightTarget = referenceByEdge.get(referenceKey(source, right.name, right.target))?.id || right.target
    const leftPosition = positions.get(leftTarget)
    const rightPosition = positions.get(rightTarget)
    if (leftPosition && rightPosition && leftPosition.y !== rightPosition.y) return leftPosition.y - rightPosition.y
    if (leftPosition && rightPosition && leftPosition.x !== rightPosition.x) return leftPosition.x - rightPosition.x
    return left.name.localeCompare(right.name)
  })
}

export function assignStepColors(
  journey: JourneyConfiguration,
  outcomesByStep: Map<string, StepOutcome[]>,
  positions: Map<string, Position>,
) {
  const incomingByStep = new Map<string, string[]>()
  for (const [source, outcomes] of outcomesByStep.entries()) {
    for (const outcome of outcomes) incomingByStep.set(outcome.target, [...(incomingByStep.get(outcome.target) || []), source])
  }

  const sortedStepIDs = Object.keys(journey.steps || {}).sort((left, right) => {
    const leftPosition = positions.get(left)
    const rightPosition = positions.get(right)
    if (leftPosition && rightPosition && leftPosition.x !== rightPosition.x) return leftPosition.x - rightPosition.x
    if (leftPosition && rightPosition && leftPosition.y !== rightPosition.y) return leftPosition.y - rightPosition.y
    return left.localeCompare(right)
  })

  const colorsByStep = new Map<string, StepColor>()
  for (const stepID of sortedStepIDs) {
    const forbidden = new Set<number>()
    for (const outcome of outcomesByStep.get(stepID) || []) addForbiddenColor(forbidden, colorsByStep.get(outcome.target))
    for (const source of incomingByStep.get(stepID) || []) addForbiddenColor(forbidden, colorsByStep.get(source))

    const seed = Math.abs(hashString(stepID))
    let selectedIndex = seed % stepPalette.length
    for (let offset = 0; offset < stepPalette.length; offset++) {
      const candidate = (selectedIndex + offset) % stepPalette.length
      if (!forbidden.has(candidate)) {
        selectedIndex = candidate
        break
      }
    }
    colorsByStep.set(stepID, stepPalette[selectedIndex])
  }
  return colorsByStep
}

export function collectHighlightedEdges(
  highlightedStepID: string | undefined,
  journey: JourneyConfiguration,
  outcomesByStep: Map<string, StepOutcome[]>,
) {
  const highlighted = new Set<string>()
  if (!highlightedStepID || !journey.steps?.[highlightedStepID]) return highlighted
  const queue = [highlightedStepID]
  const visitedSteps = new Set<string>()
  while (queue.length > 0) {
    const stepID = queue.shift()
    if (!stepID || visitedSteps.has(stepID)) continue
    visitedSteps.add(stepID)
    for (const outcome of outcomesByStep.get(stepID) || []) {
      if (!journey.steps[outcome.target]) continue
      highlighted.add(referenceKey(stepID, outcome.name, outcome.target))
      if (!visitedSteps.has(outcome.target)) queue.push(outcome.target)
    }
  }
  return highlighted
}

export function edgeLaneOffset(sourceID: string, outcome: StepOutcome, source?: Position, target?: Position) {
  if (!source || !target) return 0
  const direction = target.y >= source.y ? 1 : -1
  const distance = Math.max(1, Math.abs(target.y - source.y) / 120)
  const hash = Math.abs(hashString(`${sourceID}:${outcome.name}:${outcome.target}`)) % 4
  return direction * (42 + distance * 14 + hash * 10)
}

export function hashString(value: string) {
  let hash = 0
  for (let i = 0; i < value.length; i++) {
    hash = (hash << 5) - hash + value.charCodeAt(i)
    hash |= 0
  }
  return hash
}

function safeHandlePart(value: string) {
  return value.replace(/[^a-zA-Z0-9_-]/g, '_')
}

function addForbiddenColor(forbidden: Set<number>, color?: StepColor) {
  const index = color ? stepPalette.findIndex((candidate) => candidate.stroke === color.stroke) : -1
  if (index >= 0) forbidden.add(index)
}
