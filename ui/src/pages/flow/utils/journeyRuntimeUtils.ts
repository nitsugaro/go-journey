import type { JourneyConfiguration, JourneyStep, StepSchema } from '../../../types/journey';
import type { JourneyNote } from '../flowTypes';

export function cloneJourney(journey: JourneyConfiguration): JourneyConfiguration {
  return JSON.parse(JSON.stringify(journey)) as JourneyConfiguration;
}

export function stepsEqual(left: JourneyStep, right: JourneyStep) {
  return JSON.stringify(normalizeStepForCompare(left)) === JSON.stringify(normalizeStepForCompare(right));
}

export function normalizeStepForCompare(step: JourneyStep) {
  return {
    name: step.name || '',
    step_type: step.step_type || '',
    config: sortObjectForCompare(step.config || {}),
  };
}

export function sortObjectForCompare(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(sortObjectForCompare);
  if (!value || typeof value !== 'object') return value;
  return Object.fromEntries(
    Object.entries(value as Record<string, unknown>)
      .sort(([left], [right]) => left.localeCompare(right))
      .map(([key, item]) => [key, sortObjectForCompare(item)])
  );
}

export function ensureStepConfig(step: JourneyStep) {
  step.config ||= {};
  return step.config;
}

export function ensureOutcome(config: Record<string, unknown>) {
  if (!config.outcome || typeof config.outcome !== 'object' || Array.isArray(config.outcome)) {
    config.outcome = {};
  }
  return config.outcome as Record<string, string>;
}

export function pruneStepBranch(journey: JourneyConfiguration, rootStepID: string) {
  if (!journey.steps?.[rootStepID] || rootStepID === journey.start_step_id) return journey;

  const deleting = new Set<string>([rootStepID]);
  const queue = [...stepOutcomeTargets(journey.steps[rootStepID])];
  while (queue.length > 0) {
    const candidate = queue.shift();
    if (!candidate || deleting.has(candidate) || !journey.steps[candidate] || candidate === journey.start_step_id)
      continue;

    const parents = incomingParents(journey, candidate);
    const hasExternalParent = parents.some((parent) => !deleting.has(parent));
    if (hasExternalParent) continue;

    deleting.add(candidate);
    queue.push(...stepOutcomeTargets(journey.steps[candidate]));
  }

  for (const stepID of deleting) {
    delete journey.steps[stepID];
  }

  const additional = journey.additional_properties ? { ...journey.additional_properties } : null;
  if (additional?.notes) {
    const notes = normalizeJourneyNotes(additional.notes);
    for (const stepID of deleting) {
      delete notes[stepID];
    }
    if (Object.keys(notes).length > 0) additional.notes = notes;
    else delete additional.notes;
    journey.additional_properties = Object.keys(additional).length > 0 ? additional : null;
  }

  for (const step of Object.values(journey.steps || {})) {
    const outcome = step.config?.outcome;
    if (!outcome || typeof outcome !== 'object' || Array.isArray(outcome)) continue;
    for (const [key, target] of Object.entries(outcome as Record<string, unknown>)) {
      if (typeof target === 'string' && deleting.has(target)) {
        delete (outcome as Record<string, unknown>)[key];
      }
    }
  }

  return normalizeSubEntries(journey);
}

export function incomingParents(journey: JourneyConfiguration, targetStepID: string) {
  const parents: string[] = [];
  for (const [sourceID, step] of Object.entries(journey.steps || {})) {
    if (sourceID === targetStepID) continue;
    if (stepOutcomeTargets(step).includes(targetStepID)) {
      parents.push(sourceID);
    }
  }
  return parents;
}

export function normalizeSubEntries(journey: JourneyConfiguration) {
  const reachableFromStart = collectReachableStepIDs(journey, journey.start_step_id);
  const detached = Object.keys(journey.steps || {}).filter((stepID) => !reachableFromStart.has(stepID));
  const detachedSet = new Set(detached);
  const hasDetachedIncoming = new Set<string>();

  for (const source of detached) {
    for (const target of stepOutcomeTargets(journey.steps[source])) {
      if (detachedSet.has(target)) {
        hasDetachedIncoming.add(target);
      }
    }
  }

  journey.sub_entries = detached.filter((stepID) => !hasDetachedIncoming.has(stepID));
  return journey;
}

export function collectReachableStepIDs(journey: JourneyConfiguration, start: string) {
  const visited = new Set<string>();
  const queue = [start];
  while (queue.length > 0) {
    const stepID = queue.shift();
    if (!stepID || visited.has(stepID) || !journey.steps?.[stepID]) continue;
    visited.add(stepID);
    queue.push(...stepOutcomeTargets(journey.steps[stepID]));
  }
  return visited;
}

export function stepOutcomeTargets(step?: JourneyStep) {
  const outcome = step?.config?.outcome;
  if (!outcome || typeof outcome !== 'object' || Array.isArray(outcome)) return [];
  return Object.values(outcome).filter((target): target is string => typeof target === 'string' && target.length > 0);
}

export function normalizeJourneyNotes(value: unknown): Record<string, JourneyNote[]> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {};
  const notes: Record<string, JourneyNote[]> = {};
  for (const [stepID, rawNotes] of Object.entries(value as Record<string, unknown>)) {
    if (!Array.isArray(rawNotes)) continue;
    const stepNotes = rawNotes
      .map((rawNote) => {
        if (!rawNote || typeof rawNote !== 'object' || Array.isArray(rawNote)) return null;
        const record = rawNote as Record<string, unknown>;
        const note = typeof record.note === 'string' ? record.note.trim() : '';
        if (!note) return null;
        const timestamp =
          typeof record.timestamp === 'number' && Number.isFinite(record.timestamp) ? record.timestamp : Date.now();
        const by = typeof record.by === 'string' && record.by.trim() ? record.by.trim() : '';
        return by ? { note, by, timestamp } : { note, timestamp };
      })
      .filter((note): note is JourneyNote => Boolean(note));
    if (stepNotes.length > 0) notes[stepID] = stepNotes;
  }
  return notes;
}

export function journeyNoteCounts(value: unknown) {
  const notes = normalizeJourneyNotes(value);
  return new Map(Object.entries(notes).map(([stepID, stepNotes]) => [stepID, stepNotes.length]));
}

export function formatNoteTimestamp(timestamp: number) {
  return new Date(timestamp).toLocaleString();
}

export function extractEndJourneyTypes(schemas: StepSchema[]) {
  const types = new Set<string>();
  for (const item of schemas) {
    if (schemaHasEndJourneyExtension(item.schema)) {
      types.add(item.step_type);
    }
  }
  return types;
}

export function schemaHasEndJourneyExtension(value: unknown): boolean {
  if (!value || typeof value !== 'object') return false;
  if ((value as Record<string, unknown>)['x-end-journey'] === true) return true;
  return Object.values(value as Record<string, unknown>).some(schemaHasEndJourneyExtension);
}

