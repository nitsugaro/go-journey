import type { JourneyConfiguration } from '../../../types/journey';
import { cloneJourney } from './journeyRuntimeUtils';

const journeyIdentityKeys = ['id', 'rev', 'created_at', 'modified_at'] as const;

export function prepareJourneyCreation(
  source: JourneyConfiguration,
  realm: string,
  name: string,
  description: string,
): JourneyConfiguration {
  const journey = cloneJourney(source) as JourneyConfiguration & Record<string, unknown>;
  for (const key of journeyIdentityKeys) delete journey[key];
  journey.realm = realm;
  journey.name = name.trim();
  journey.description = description.trim();
  return journey;
}

export function parseJourneyImport(value: string, realm: string): JourneyConfiguration {
  let parsed: unknown;
  try {
    parsed = JSON.parse(value);
  } catch {
    throw new Error('The selected file is not valid JSON.');
  }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('The JSON must contain one journey configuration object.');
  }

  const journey = parsed as JourneyConfiguration;
  if (typeof journey.name !== 'string' || !journey.name.trim()) {
    throw new Error('The imported journey must have a name.');
  }
  if (typeof journey.start_step_id !== 'string' || !journey.start_step_id.trim()) {
    throw new Error('The imported journey must have a start_step_id.');
  }
  if (!journey.steps || typeof journey.steps !== 'object' || Array.isArray(journey.steps)) {
    throw new Error('The imported journey must have a steps object.');
  }
  if (!journey.steps[journey.start_step_id]) {
    throw new Error(`The imported start step "${journey.start_step_id}" does not exist in steps.`);
  }
  return prepareJourneyCreation(journey, realm, journey.name, journey.description || '');
}

export function exportJourneyJSON(journey: JourneyConfiguration) {
  const portable = cloneJourney(journey) as JourneyConfiguration & Record<string, unknown>;
  for (const key of journeyIdentityKeys) delete portable[key];
  return `${JSON.stringify(portable, null, 2)}\n`;
}

export function journeyExportFilename(journey: JourneyConfiguration) {
  const base = (journey.name || 'journey')
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9._-]+/g, '-')
    .replace(/^-+|-+$/g, '');
  return `${base || 'journey'}.journey.json`;
}

export function suggestJourneyCopyName(name: string, existingNames: string[]) {
  const base = `${name.trim() || 'Untitled journey'} copy`;
  const used = new Set(existingNames.map((item) => item.trim().toLowerCase()));
  if (!used.has(base.toLowerCase())) return base;
  let suffix = 2;
  while (used.has(`${base} ${suffix}`.toLowerCase())) suffix += 1;
  return `${base} ${suffix}`;
}

