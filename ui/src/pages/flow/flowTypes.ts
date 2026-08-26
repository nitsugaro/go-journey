import type { JourneyConfiguration } from '../../types/journey';

export type ConnectDraft = {
  source: string;
  outcome: string;
};

export type JourneyHistory = {
  past: JourneyConfiguration[];
  future: JourneyConfiguration[];
};

export type NewJourneyForm = {
  name: string;
  description: string;
  journey_type: string;
  active: boolean;
  confidential: boolean;
  encrypted_client_inputs: boolean;
  debug: boolean;
  default_exp: number;
  props: JourneyPropDefinition[];
};

export type JourneyFilters = {
  journeyType: string;
  active: JourneyBooleanFilter;
  debug: JourneyBooleanFilter;
  confidential: JourneyBooleanFilter;
  encryptedInputs: JourneyBooleanFilter;
};

export type JourneyBooleanFilter = 'all' | 'yes' | 'no';

export type JourneyPropDefinition = {
  id: string;
  name: string;
  type: JourneyPropType;
};

export type JourneyPropType = 'string' | 'int' | 'float' | 'bool' | 'object' | 'list';

export type JourneyNote = {
  note: string;
  by?: string;
  timestamp: number;
};

export type StepEditorState = {
  mode: 'add' | 'edit';
  sourceStepID?: string;
  stepID: string;
  stepName: string;
  stepType: string;
  connectOutcome?: string;
  configText: string;
};

export type NestedStepEditorState = {
  fieldKey: string;
  index: number;
};

export type JSONSchema = {
  $ref?: string;
  type?: string | string[];
  title?: string;
  description?: string;
  default?: unknown;
  enum?: unknown[];
  anyOf?: JSONSchema[];
  oneOf?: JSONSchema[];
  allOf?: JSONSchema[];
  properties?: Record<string, JSONSchema>;
  required?: string[];
  items?: JSONSchema;
  additionalProperties?: boolean | JSONSchema;
  definitions?: Record<string, JSONSchema>;
  $defs?: Record<string, JSONSchema>;
  format?: string;
  minimum?: number;
  maximum?: number;
  ['x-order']?: string[];
  ['x-dynamic-outcome']?: boolean;
  ['x-flow-type']?: string[];
  ['x-type']?: string;
  ['x-props']?: Record<string, unknown>;
  ['x-sub-journey-props']?: boolean;
};

export type SelectableSource = {
  resource?: string;
  endpoint?: string;
  query?: Record<string, unknown>;
  nameProperty: string;
  valueProperty: string;
};

export type SelectableOption = { label: string; value: string };
