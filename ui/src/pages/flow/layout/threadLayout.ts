import dagre from '@dagrejs/dagre'
import type { JourneyConfiguration, StepOutcome } from '../../../types/journey'
import {
	canvasPadding,
	nodeBaseHeight,
	nodeWidth,
	outcomeHeight,
	referenceHeight,
	referenceKey,
	referenceWidth,
	type Position,
	type ReferenceSpec,
} from './flowLayoutTypes'
import { edgeReferenceMap, resolveReferenceCollisions } from './referenceLayout'

export function layoutSteps(
  journey: JourneyConfiguration,
  outcomesByStep: Map<string, StepOutcome[]>,
  references: ReferenceSpec[],
  threadByStep: Map<string, string>,
) {
  const positions = new Map<string, Position>()
  const threads = groupStepsByThread(journey, threadByStep)
  const referenceByEdge = edgeReferenceMap(references)
  let nextBandY = canvasPadding.y

	for (const thread of orderedThreads(journey, threads)) {
		const nodeIDs = threads.get(thread) || []
		if (nodeIDs.length === 0) continue
		const threadReferences = references.filter((reference) => threadByStep.get(reference.source) === thread)
		const referenceIDs = threadReferences.map((reference) => reference.id)
		const threadPositions = layoutThread(journey, outcomesByStep, nodeIDs, threadReferences, threadByStep, referenceByEdge)
		const bounds = boundsFor([...nodeIDs, ...referenceIDs], threadPositions, outcomesByStep, threadReferences)
		if (!bounds) continue

		const layoutNodeIDs = [...nodeIDs, ...referenceIDs]
		const minX = Math.min(...layoutNodeIDs.map((stepID) => threadPositions.get(stepID)?.x ?? 0))
		for (const stepID of layoutNodeIDs) {
			const position = threadPositions.get(stepID)
			if (!position) continue
      positions.set(stepID, {
        x: position.x - minX + canvasPadding.x,
        y: position.y - bounds.top + nextBandY,
      })
    }
		nextBandY += bounds.bottom - bounds.top + 220
	}

	resolveReferenceCollisions(journey, outcomesByStep, references, positions)
  normalizePositionsInPlace(positions)
  return positions
}

export function buildThreadOwnership(journey: JourneyConfiguration, outcomesByStep: Map<string, StepOutcome[]>) {
  const roots = uniqueNodeIDs([journey.start_step_id, ...(journey.sub_entries || [])].filter(Boolean))
  const rootSet = new Set(roots)
  const visits = new Map<string, { thread: string; distance: number; priority: number; root: string }>()
  let priorityCounter = roots.length
  const queue = roots.map((root, priority) => ({
    root,
    stepID: root,
    thread: root === journey.start_step_id ? 'main' : `sub:${root}`,
    distance: 0,
    priority,
  }))

  while (queue.length > 0) {
    const current = queue.shift()
    if (!current) continue
    const existing = visits.get(current.stepID)
		if (existing && (existing.priority < current.priority || (existing.priority === current.priority && existing.distance <= current.distance))) continue

    visits.set(current.stepID, current)
    for (const outcome of outcomesByStep.get(current.stepID) || []) {
      if (!journey.steps[outcome.target]) continue
      if (rootSet.has(outcome.target) && outcome.target !== current.root) continue
      queue.push({ ...current, stepID: outcome.target, distance: current.distance + 1 })
    }
  }

  for (const stepID of outcomesByStep.keys()) {
    if (visits.has(stepID)) continue
    const thread = `detached:${stepID}`
    const priority = priorityCounter++
    const detachedQueue = [{ root: stepID, stepID, thread, distance: 0, priority }]
    while (detachedQueue.length > 0) {
      const current = detachedQueue.shift()
      if (!current || visits.has(current.stepID)) continue
      visits.set(current.stepID, current)
      for (const outcome of outcomesByStep.get(current.stepID) || []) {
        if (!journey.steps[outcome.target]) continue
        if (rootSet.has(outcome.target) || visits.has(outcome.target)) continue
        detachedQueue.push({ ...current, stepID: outcome.target, distance: current.distance + 1 })
      }
    }
  }

  return new Map([...visits.entries()].map(([stepID, visit]) => [stepID, visit.thread]))
}

export function isCrossThread(source: string, target: string, threadByStep: Map<string, string>) {
  return threadByStep.get(source) !== threadByStep.get(target)
}

export function boundsFor(
  nodeIDs: string[],
  positions: Map<string, Position>,
  outcomesByStep: Map<string, StepOutcome[]>,
  references: ReferenceSpec[],
) {
  const referenceIDs = new Set(references.map((reference) => reference.id))
  let top = Number.POSITIVE_INFINITY
  let bottom = Number.NEGATIVE_INFINITY
  for (const nodeID of nodeIDs) {
    const position = positions.get(nodeID)
    if (!position) continue
    const height = referenceIDs.has(nodeID) ? referenceHeight : nodeBaseHeight + Math.max(1, outcomesByStep.get(nodeID)?.length || 0) * outcomeHeight
    top = Math.min(top, position.y)
    bottom = Math.max(bottom, position.y + height)
  }
  if (!Number.isFinite(top) || !Number.isFinite(bottom)) return null
  return { top, bottom }
}

function layoutThread(
  journey: JourneyConfiguration,
	outcomesByStep: Map<string, StepOutcome[]>,
	nodeIDs: string[],
	references: ReferenceSpec[],
	threadByStep: Map<string, string>,
	referenceByEdge: Map<string, ReferenceSpec>,
) {
	const graph = new dagre.graphlib.Graph()
	const nodeSet = new Set(nodeIDs)
	const referenceSet = new Set(references.map((reference) => reference.id))
	graph.setDefaultEdgeLabel(() => ({}))
  graph.setGraph({ rankdir: 'LR', ranker: 'network-simplex', align: 'UL', nodesep: 110, ranksep: 150, edgesep: 80, marginx: 0, marginy: 0 })

  for (const stepID of nodeIDs) {
    const step = journey.steps[stepID]
    const outcomes = outcomesByStep.get(stepID) || []
    graph.setNode(stepID, {
      width: nodeWidth,
      height: nodeBaseHeight + Math.max(1, outcomes.length) * outcomeHeight,
      rank: stepID === journey.start_step_id ? 'min' : undefined,
      label: step.name || step.step_type || 'Step',
		})
	}
	for (const reference of references) {
		graph.setNode(reference.id, {
			width: referenceWidth,
			height: referenceHeight,
			label: reference.id,
		})
	}

	const virtualEdges = new Set<string>()
	for (const source of nodeIDs) {
		for (const outcome of outcomesByStep.get(source) || []) {
			const reference = referenceByEdge.get(referenceKey(source, outcome.name, outcome.target))
			if (reference && referenceSet.has(reference.id)) {
				const edgeKey = `${source}\u0000${reference.id}`
				if (!virtualEdges.has(edgeKey)) {
					graph.setEdge(source, reference.id, { label: reference.outcomes.join(', '), weight: 1, minlen: 1 })
					virtualEdges.add(edgeKey)
				}
				continue
			}
			if (!nodeSet.has(outcome.target) || source === outcome.target) continue
			if (isCrossThread(source, outcome.target, threadByStep)) continue
			graph.setEdge(source, outcome.target, { label: outcome.name, weight: 1, minlen: 1 })
    }
  }

  dagre.layout(graph)
  const positions = new Map<string, Position>()
	for (const stepID of [...nodeIDs, ...referenceSet]) {
		const node = graph.node(stepID)
    if (node) positions.set(stepID, { x: node.x - node.width / 2, y: node.y - node.height / 2 })
  }
  return positions
}

function groupStepsByThread(journey: JourneyConfiguration, threadByStep: Map<string, string>) {
  const threads = new Map<string, string[]>()
  for (const stepID of Object.keys(journey.steps || {})) {
    const thread = threadByStep.get(stepID) || `detached:${stepID}`
    threads.set(thread, [...(threads.get(thread) || []), stepID])
  }
  return threads
}

function orderedThreads(journey: JourneyConfiguration, threads: Map<string, string[]>) {
  const ordered: string[] = []
  if (threads.has('main')) ordered.push('main')
  for (const subEntry of journey.sub_entries || []) {
    const thread = `sub:${subEntry}`
    if (threads.has(thread)) ordered.push(thread)
  }
  for (const thread of [...threads.keys()].sort()) {
    if (!ordered.includes(thread)) ordered.push(thread)
  }
  return ordered
}

function normalizePositionsInPlace(positions: Map<string, Position>) {
  const minX = Math.min(...Array.from(positions.values()).map((position) => position.x), 0)
  const minY = Math.min(...Array.from(positions.values()).map((position) => position.y), 0)
  for (const [stepID, position] of positions.entries()) {
    positions.set(stepID, { x: position.x - minX + canvasPadding.x, y: position.y - minY + canvasPadding.y })
  }
}

function uniqueNodeIDs(values: string[]) {
  return Array.from(new Set(values))
}
