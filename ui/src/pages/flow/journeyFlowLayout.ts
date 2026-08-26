import type { Edge, Node } from '@xyflow/react'
import type { JourneyConfiguration, StepOutcome } from '../../types/journey'
import type { JourneyStepNodeData } from './JourneyStepNode'
import {
  assignStepColors,
  collectHighlightedEdges,
  configString,
  edgeLaneOffset,
  extractOutcomes,
  outcomeHandleID,
  referenceKey,
  sortOutcomesByVisualTarget,
  stepPalette,
  type FlowActions,
  type FlowOptions,
} from './layout/flowLayoutTypes'
import { createReferenceNode, detectReferences, edgeReferenceMap, placeReferences } from './layout/referenceLayout'
import { buildThreadOwnership, layoutSteps } from './layout/threadLayout'

export function buildJourneyFlow(journey: JourneyConfiguration | null, actions: FlowActions = {}, options: FlowOptions = {}): { nodes: Node[]; edges: Edge[] } {
  if (!journey) return { nodes: [], edges: [] }

  const endJourneyTypes = options.endJourneyTypes || new Set(['Success', 'Failure'])
  const outcomesByStep = buildOutcomesByStep(journey, endJourneyTypes, options)
  const threadByStep = buildThreadOwnership(journey, outcomesByStep)
  const firstPassPositions = layoutSteps(journey, outcomesByStep, [], threadByStep)
  const references = detectReferences(journey, outcomesByStep, firstPassPositions, threadByStep)
  const positions = new Map(firstPassPositions)
  placeReferences(journey, outcomesByStep, references, positions)
  const referenceByEdge = edgeReferenceMap(references)
  const colorsByStep = assignStepColors(journey, outcomesByStep, positions)
  const highlightedEdges = collectHighlightedEdges(options.highlightedStepID, journey, outcomesByStep)

  const nodes = buildStepNodes(journey, outcomesByStep, positions, referenceByEdge, colorsByStep, endJourneyTypes, actions, options)
  const referenceNodes = references.map((reference) =>
    createReferenceNode(reference, journey.steps[reference.target], positions.get(reference.id) || { x: 0, y: 0 }, colorsByStep.get(reference.target) || stepPalette[0]),
  )
  const edges = buildEdges(outcomesByStep, positions, referenceByEdge, colorsByStep, highlightedEdges, actions, journey)

  return { nodes: [...nodes, ...referenceNodes], edges }
}

function buildOutcomesByStep(journey: JourneyConfiguration, endJourneyTypes: Set<string>, options: FlowOptions) {
  const outcomesByStep = new Map<string, StepOutcome[]>()
  for (const [stepID, step] of Object.entries(journey.steps || {})) {
    const staticOutcomes = options.staticOutcomesByStepID?.get(stepID)
      || options.staticOutcomesByStepType?.get(step.step_type)
      || []
    const strictStatic = options.staticOutcomesByStepID?.has(stepID) || false
    outcomesByStep.set(stepID, endJourneyTypes.has(step.step_type) ? [] : extractOutcomes(step, staticOutcomes, strictStatic))
  }
  return outcomesByStep
}

function buildStepNodes(
  journey: JourneyConfiguration,
  outcomesByStep: Map<string, StepOutcome[]>,
  positions: Map<string, { x: number; y: number }>,
  referenceByEdge: ReturnType<typeof edgeReferenceMap>,
  colorsByStep: ReturnType<typeof assignStepColors>,
  endJourneyTypes: Set<string>,
  actions: FlowActions,
  options: FlowOptions,
): Node<JourneyStepNodeData>[] {
  const subEntries = new Set(journey.sub_entries || [])
  return Object.entries(journey.steps || {}).map(([stepID, step]) => {
    const outcomes = sortOutcomesByVisualTarget(outcomesByStep.get(stepID) || [], stepID, positions, referenceByEdge)
    const endJourney = endJourneyTypes.has(step.step_type)
    const color = colorsByStep.get(stepID) || stepPalette[0]
    const subJourneyId = step.step_type === 'SubJourney' ? configString(step.config, 'journey_id') : ''

    return {
      id: stepID,
      type: 'journeyStep',
      className: 'journey-flow-node',
      position: positions.get(stepID) || { x: 0, y: 0 },
      data: {
        id: stepID,
        name: step.name || step.step_type || 'Unnamed step',
        stepType: step.step_type,
        start: stepID === journey.start_step_id,
        subEntry: subEntries.has(stepID),
        terminal: endJourney || outcomes.length === 0,
        endJourney,
        needsOutcomeConfiguration: !endJourney && outcomes.length === 0 && options.dynamicOutcomeStepTypes?.has(step.step_type),
        noteCount: options.noteCountsByStep?.get(stepID) || 0,
        subJourneyId,
        subJourneyName: subJourneyId,
        color,
        ...actions,
        outcomes: outcomes.map((outcome) => ({ ...outcome, id: outcomeHandleID(outcome.name, outcome.target) })),
      },
    }
  })
}

function buildEdges(
  outcomesByStep: Map<string, StepOutcome[]>,
  positions: Map<string, { x: number; y: number }>,
  referenceByEdge: ReturnType<typeof edgeReferenceMap>,
  colorsByStep: ReturnType<typeof assignStepColors>,
  highlightedEdges: Set<string>,
  actions: FlowActions,
  journey: JourneyConfiguration,
): Edge[] {
  const edges: Edge[] = []
  for (const [stepID, outcomes] of outcomesByStep.entries()) {
    for (const outcome of sortOutcomesByVisualTarget(outcomes, stepID, positions, referenceByEdge)) {
      if (!journey.steps[outcome.target]) continue
      const reference = referenceByEdge.get(referenceKey(stepID, outcome.name, outcome.target))
      const targetID = reference?.id || outcome.target
      const selectedOutcome = actions.connectingOutcome?.source === stepID && actions.connectingOutcome.outcome === outcome.name
      const sourceHasSelectedOutcome = actions.connectingOutcome?.source === stepID
      const highlighted = highlightedEdges.has(referenceKey(stepID, outcome.name, outcome.target))
      const sourceColor = colorsByStep.get(stepID)?.stroke || stepPalette[0].stroke

      edges.push({
        id: `${stepID}:${outcome.name}:${targetID}`,
        source: stepID,
        sourceHandle: outcomeHandleID(outcome.name, outcome.target),
        target: targetID,
        type: 'journeyEdge',
        animated: highlighted,
        style: {
          stroke: sourceColor,
          strokeWidth: selectedOutcome ? 4 : highlighted ? 2.5 : 2,
          opacity: sourceHasSelectedOutcome && !selectedOutcome ? 0.24 : selectedOutcome ? 1 : 0.95,
        },
        data: {
          sourceY: positions.get(stepID)?.y || 0,
          targetY: positions.get(targetID)?.y || 0,
          laneOffset: reference ? 0 : edgeLaneOffset(stepID, outcome, positions.get(stepID), positions.get(targetID)),
          reference: Boolean(reference),
          highlighted,
        },
      })
    }
  }
  return edges
}
