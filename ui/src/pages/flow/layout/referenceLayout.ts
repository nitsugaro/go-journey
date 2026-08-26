import type { Node } from '@xyflow/react'
import type { JourneyConfiguration, JourneyStep, StepOutcome } from '../../../types/journey'
import type { JourneyStepNodeData } from '../JourneyStepNode'
import {
	canvasPadding,
  columnStride,
  longReferenceDistance,
  nodeBaseHeight,
  nodeWidth,
  outcomeHeight,
  referenceCollisionGap,
  referenceHeight,
  referenceKey,
  referenceWidth,
  type Position,
  type ReferenceSpec,
  type StepColor,
} from './flowLayoutTypes'
import { isCrossThread } from './threadLayout'

export function createReferenceNode(
  reference: ReferenceSpec,
  originalStep: JourneyStep,
  position: Position,
  color: StepColor,
): Node<JourneyStepNodeData> {
  return {
    id: reference.id,
    type: 'journeyStep',
    className: 'journey-flow-node',
    position,
    data: {
      id: reference.id,
      originalId: reference.target,
      reference: true,
      name: originalStep.name || originalStep.step_type || 'Referenced step',
      stepType: originalStep.step_type,
      color,
      outcomes: [],
    },
    draggable: false,
  }
}

export function detectReferences(
  journey: JourneyConfiguration,
  outcomesByStep: Map<string, StepOutcome[]>,
  positions: Map<string, Position>,
  threadByStep: Map<string, string>,
) {
  const references: ReferenceSpec[] = []
  const referenceByTarget = new Map<string, ReferenceSpec>()
  const crossingReferences = detectCrossingReferences(journey, outcomesByStep, positions, threadByStep)
  for (const [source, outcomes] of outcomesByStep.entries()) {
    for (const outcome of outcomes) {
      if (!journey.steps[outcome.target]) continue
      const sourcePosition = positions.get(source)
      const targetPosition = positions.get(outcome.target)
      const crossThread = isCrossThread(source, outcome.target, threadByStep)
      const selfCall = source === outcome.target
      const backwardsOrSameColumn = Boolean(sourcePosition && targetPosition && targetPosition.x <= sourcePosition.x)
      const tooLong = Boolean(sourcePosition && targetPosition && Math.abs(targetPosition.x - sourcePosition.x) > longReferenceDistance)
      const groupKey = `${source}\u0000${outcome.target}`
      const crossesAnotherEdge = crossingReferences.has(groupKey)
      if (!selfCall && !crossThread && !backwardsOrSameColumn && !tooLong && !crossesAnotherEdge) continue
      const existing = referenceByTarget.get(groupKey)
      if (existing) {
        existing.outcomes.push(outcome.name)
        continue
      }
      const reference = {
        id: `ref:${source}:${outcome.target}`,
        source,
        outcomes: [outcome.name],
        target: outcome.target,
        crossThread,
        selfCall,
      }
      references.push(reference)
      referenceByTarget.set(groupKey, reference)
    }
  }
  return references
}

export function placeReferences(
  journey: JourneyConfiguration,
  outcomesByStep: Map<string, StepOutcome[]>,
  references: ReferenceSpec[],
  positions: Map<string, Position>,
) {
  for (const reference of references) {
    const source = positions.get(reference.source)
    if (!source) continue
    const sourceHeight = nodeBaseHeight + Math.max(1, outcomesByStep.get(reference.source)?.length || 0) * outcomeHeight
    positions.set(reference.id, {
      x: source.x + columnStride,
      y: source.y + (sourceHeight - referenceHeight) / 2,
    })
  }
  resolveReferenceCollisions(journey, outcomesByStep, references, positions)
  const minY = Math.min(...Array.from(positions.values(), (position) => position.y))
  if (minY < canvasPadding.y) {
    const offset = canvasPadding.y - minY
    for (const [id, position] of positions) positions.set(id, { ...position, y: position.y + offset })
  }
}

/**
 * Dagre minimizes crossings, but a complete 2x2 connection between two source
 * nodes and two target nodes cannot be drawn as monotonic direct edges without
 * one diagonal crossing the other. Keep the edges of the visually earlier
 * source direct and turn only later conflicting source/target pairs into jump
 * references. Outcomes that share a source and target remain one group.
 */
function detectCrossingReferences(
  journey: JourneyConfiguration,
  outcomesByStep: Map<string, StepOutcome[]>,
  positions: Map<string, Position>,
  threadByStep: Map<string, string>,
) {
  const edges = new Map<string, VisualEdge>()
  for (const [source, outcomes] of outcomesByStep.entries()) {
    const sourcePosition = positions.get(source)
    if (!sourcePosition) continue
    for (const outcome of outcomes) {
      const targetPosition = positions.get(outcome.target)
      if (!journey.steps[outcome.target] || !targetPosition || source === outcome.target) continue
      if (isCrossThread(source, outcome.target, threadByStep) || targetPosition.x <= sourcePosition.x) continue
      const key = `${source}\u0000${outcome.target}`
      if (!edges.has(key)) edges.set(key, { key, source, target: outcome.target, sourcePosition, targetPosition })
    }
  }

  const ordered = [...edges.values()].sort((left, right) => {
    if (left.sourcePosition.x !== right.sourcePosition.x) return left.sourcePosition.x - right.sourcePosition.x
    if (left.sourcePosition.y !== right.sourcePosition.y) return left.sourcePosition.y - right.sourcePosition.y
    if (left.targetPosition.y !== right.targetPosition.y) return left.targetPosition.y - right.targetPosition.y
    return left.key.localeCompare(right.key)
  })
  const direct: VisualEdge[] = []
  const references = new Set<string>()
  for (const edge of ordered) {
    let keepDirect = true
    for (let crossingIndex = 0; crossingIndex < direct.length; crossingIndex++) {
      const crossing = direct[crossingIndex]
      if (!edgesCrossInAdjacentColumns(crossing, edge)) continue
      if (isDownwardEdge(crossing) && !isDownwardEdge(edge)) {
        // Keeping a long downward edge from an upper branch makes it pass
        // through the lower branch after references participate in the second
        // Dagre pass. Reference that edge and retain the upward connection.
        references.add(crossing.key)
        direct.splice(crossingIndex, 1)
        crossingIndex--
      } else {
        references.add(edge.key)
        keepDirect = false
        break
      }
    }
    if (keepDirect) direct.push(edge)
  }
  return references
}

type VisualEdge = {
  key: string
  source: string
  target: string
  sourcePosition: Position
  targetPosition: Position
}

function edgesCrossInAdjacentColumns(left: VisualEdge, right: VisualEdge) {
  if (left.source === right.source || left.target === right.target) return false

  // Crossing order is only meaningful when both pairs connect the same two
  // visual ranks. Wider jumps are already represented by the long-edge rule.
  const columnTolerance = nodeWidth / 2
  if (Math.abs(left.sourcePosition.x - right.sourcePosition.x) > columnTolerance) return false
  if (Math.abs(left.targetPosition.x - right.targetPosition.x) > columnTolerance) return false

  const sourceOrder = left.sourcePosition.y - right.sourcePosition.y
  const targetOrder = left.targetPosition.y - right.targetPosition.y
  return sourceOrder !== 0 && targetOrder !== 0 && sourceOrder * targetOrder < 0
}

function isDownwardEdge(edge: VisualEdge) {
  return edge.targetPosition.y > edge.sourcePosition.y
}

export function resolveReferenceCollisions(
	journey: JourneyConfiguration,
	outcomesByStep: Map<string, StepOutcome[]>,
	references: ReferenceSpec[],
	positions: Map<string, Position>,
) {
  const occupied: Rect[] = []
  for (const stepID of Object.keys(journey.steps || {})) {
    const position = positions.get(stepID)
    if (!position) continue
    occupied.push(expandRect({
      x: position.x,
      y: position.y,
      width: nodeWidth,
      height: nodeBaseHeight + Math.max(1, outcomesByStep.get(stepID)?.length || 0) * outcomeHeight,
    }, referenceCollisionGap))
  }

  for (const reference of references) {
    const position = positions.get(reference.id)
    if (!position) continue
    const sourcePosition = positions.get(reference.source)
		const preferredX = sourcePosition ? sourcePosition.x + columnStride : position.x
		let candidate = { x: Math.max(position.x, preferredX), y: position.y, width: referenceWidth, height: referenceHeight }

		let lane = 0
		while (occupied.some((rect) => intersects(candidate, rect)) && lane < 120) {
			lane++
			const direction = lane % 2 === 1 ? -1 : 1
			const distance = Math.ceil(lane / 2) * (referenceHeight + referenceCollisionGap)
			candidate = { ...candidate, y: position.y + direction * distance }
    }

    positions.set(reference.id, { x: candidate.x, y: candidate.y })
    occupied.push(expandRect(candidate, referenceCollisionGap))
  }
}

export function edgeReferenceMap(references: ReferenceSpec[]) {
  const result = new Map<string, ReferenceSpec>()
  for (const reference of references) {
    for (const outcome of reference.outcomes) {
      result.set(referenceKey(reference.source, outcome, reference.target), reference)
    }
  }
  return result
}

type Rect = {
  x: number
  y: number
  width: number
  height: number
}

function expandRect(rect: Rect, gap: number): Rect {
  return {
    x: rect.x - gap,
    y: rect.y - gap,
    width: rect.width + gap * 2,
    height: rect.height + gap * 2,
  }
}

function intersects(left: Rect, right: Rect) {
  return left.x < right.x + right.width && left.x + left.width > right.x && left.y < right.y + right.height && left.y + left.height > right.y
}
