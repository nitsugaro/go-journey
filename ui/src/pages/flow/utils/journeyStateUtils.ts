import type { JourneyConfiguration, JourneyStep } from '../../../types/journey';
import type { JourneyPropDefinition, JourneyPropType, NewJourneyForm, JSONSchema } from '../flowTypes';
import { asRecord, formatEditableValue, normalizeJourneyType } from './schemaUtils';

export function defaultNewJourneyForm(): NewJourneyForm {
  return {
    name: '',
    description: '',
    journey_type: 'auth',
    active: true,
    confidential: false,
    encrypted_client_inputs: false,
    debug: false,
    default_exp: 1,
    props: [],
  };
}

export function normalizeJourneyPropDefinitions(value: unknown): JourneyPropDefinition[] {
  if (!Array.isArray(value)) return [];
  return value
    .map((item) => {
      const record = asRecord(item);
      const id = formatEditableValue(record.id).trim();
      if (!id) return null;
      const type = normalizeJourneyPropType(formatEditableValue(record.type));
      return {
        id,
        name: formatEditableValue(record.name).trim() || id,
        type,
      };
    })
    .filter((item): item is JourneyPropDefinition => Boolean(item));
}

export function normalizeJourneyPropType(value: string): JourneyPropType {
  const normalized = value
    .trim()
    .toLowerCase()
    .replace(/[\s-]+/g, '_');
  if (normalized === 'string') return 'string';
  if (normalized === 'int') return 'int';
  if (normalized === 'integer') return 'int';
  if (normalized === 'float') return 'float';
  if (normalized === 'number' || normalized === 'double' || normalized === 'decimal') return 'float';
  if (normalized === 'bool') return 'bool';
  if (normalized === 'boolean') return 'bool';
  if (normalized === 'array' || normalized === 'list_of' || normalized.startsWith('list')) return 'list';
  if (normalized === 'object' || normalized === 'map') return 'object';
  return 'string';
}

export function schemaForJourneyProp(prop: JourneyPropDefinition): JSONSchema {
  const base = { title: prop.name || prop.id, description: `Prop ${prop.id}` };
  switch (prop.type) {
    case 'int':
      return { ...base, type: 'integer' };
    case 'float':
      return { ...base, type: 'number' };
    case 'bool':
      return { ...base, type: 'boolean' };
    case 'object':
      return { ...base, type: 'object', additionalProperties: true };
    case 'list':
      return { ...base, type: 'array', items: { type: 'string' } };
    default:
      return { ...base, type: 'string' };
  }
}


export function cleanJourneyPropDefinitions(props: JourneyPropDefinition[]) {
  return props
    .map((prop) => ({
      id: prop.id.trim(),
      name: prop.name.trim() || prop.id.trim(),
      type: normalizeJourneyPropType(prop.type),
    }))
    .filter((prop, index, items) => prop.id && items.findIndex((item) => item.id === prop.id) === index);
}

export function stepDisplayName(journey: JourneyConfiguration | null, stepID: string) {
  const step = journey?.steps?.[stepID];
  return step?.name || step?.step_type || 'Unnamed step';
}


export function newStepID(steps: Record<string, JourneyStep>) {
  let id = randomUUID();
  while (steps[id]) id = randomUUID();
  return id;
}

export function randomUUID() {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (char) => {
    const value = Math.floor(Math.random() * 16);
    const nibble = char === 'x' ? value : (value & 0x3) | 0x8;
    return nibble.toString(16);
  });
}

export function isUUID(value: string) {
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value);
}

export function sanitizeStepConfig(config: Record<string, unknown>) {
  const next = { ...config };
  const outcome = next.outcome;
  if (outcome && typeof outcome === 'object' && !Array.isArray(outcome)) {
    const cleanOutcome = Object.fromEntries(
      Object.entries(outcome as Record<string, unknown>).filter(
        ([key, target]) => key.trim() !== '' && typeof target === 'string'
      )
    );
    if (Object.keys(cleanOutcome).length > 0) {
      next.outcome = cleanOutcome;
    } else {
      delete next.outcome;
    }
  }
  return next;
}

export function uniqueStrings(values: string[]) {
  return Array.from(new Set(values.filter(Boolean)));
}

export function humanizeKey(value: string) {
  return value.replace(/_/g, ' ');
}

export function createJourneyPayload(realm: string, form: NewJourneyForm): JourneyConfiguration {
  const journeyType = normalizeJourneyType(form.journey_type);
  const startStepID = randomUUID();
  const successStepID = randomUUID();
  const props = cleanJourneyPropDefinitions(form.props);
  let steps: Record<string, JourneyStep>;
  if (journeyType === 'auth') {
    steps = {
      [startStepID]: {
        name: 'Collect user name',
        step_type: 'Form',
        config: {
          context: 'ctx',
          inputs: [
            {
              id: 'userName',
              external_id: 'profile.username',
              label: 'User name',
              prompt: 'Choose your user name',
              type: 'string',
              required: true,
              min: 3,
              max: 30,
              user_name: true,
            },
          ],
          outcome: {
            true: successStepID,
          },
        },
      },
      [successStepID]: {
        name: 'Success',
        step_type: 'Success',
        config: {
          data: {
            created_from: 'journey-ui',
          },
        },
      },
    };
  } else if (journeyType === 'workflow') {
    steps = {
      [startStepID]: {
        name: 'Prepare workflow',
        step_type: 'SetCtxProperty',
        config: {
          type: 'ctx',
          key: 'started',
          expression: 'true',
          outcome: {
            true: successStepID,
            false: successStepID,
          },
        },
      },
      [successStepID]: {
        name: 'End',
        step_type: 'End',
        config: {
          result: {
            created_from: 'journey-ui',
          },
        },
      },
    };
  } else {
    const finishStepID = randomUUID();
    steps = {
      [startStepID]: {
        name: 'Start',
        step_type: 'SetCtxProperty',
        config: {
          type: 'ctx',
          key: 'started',
          expression: 'true',
          outcome: {
            true: successStepID,
            false: successStepID,
          },
        },
      },
      [successStepID]: {
        name: 'HTTP response',
        step_type: 'HTTPResponse',
        config: {
          status_code: 200,
          content_type: 'JSON',
          body: {
            created_from: 'journey-ui',
          },
          outcome: {
            true: finishStepID,
          },
        },
      },
      [finishStepID]: {
        name: 'Finish response',
        step_type: 'HTTPFinishResponse',
        config: {},
      },
    };
  }
  return {
    name: form.name.trim(),
    description: form.description.trim(),
    realm,
    journey_type: journeyType,
    active: form.active,
    confidential: form.confidential,
    encrypted_client_inputs: form.encrypted_client_inputs,
    debug: form.debug,
    default_exp: form.default_exp,
    start_step_id: startStepID,
    sub_entries: [],
    steps,
    additional_properties: props.length > 0 ? { props } : null,
  };
}
