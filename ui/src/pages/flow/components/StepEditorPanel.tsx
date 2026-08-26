import { useState } from 'react';
import { Button } from '../../../components/Button';
import type { JourneyConfiguration, JourneyScript, JourneyStep, StepSchema } from '../../../types/journey';
import type { NestedStepEditorState, StepEditorState } from '../flowTypes';
import {
  asSchema,
  defaultConfigForSchema,
  isDynamicOutcomeSchema,
  normalizeEmbeddedStep,
  normalizeJourneyType,
  objectRecord,
  orderedSchemaProperties,
  parseConfigText,
  stepSchemaDescription,
  stepSchemaSupportsJourneyType,
} from '../utils/schemaUtils';
import { stepDisplayName } from '../utils/journeyStateUtils';
import { newStepID } from '../utils/journeyStateUtils';
import { HighlightedTextEditor } from './CodeEditors';
import { NestedStepEditorPanel } from './NestedStepEditorPanel';
import { OutcomeEditor } from './OutcomeEditor';
import { SchemaField } from './SchemaField';
import { scriptOutcomes, syncScriptOutcomeTargets } from '../../scripts/scriptOutcomes';
import { formatStepTypeLabel } from '../utils/displayUtils';

export function StepEditorPanel({
  realm,
  editor,
  schemas,
  scripts,
  journey,
  onChange,
  onCancel,
  onSave,
}: {
  realm: string;
  editor: StepEditorState;
  schemas: StepSchema[];
  scripts: JourneyScript[];
  journey: JourneyConfiguration | null;
  onChange: (editor: StepEditorState) => void;
  onCancel: () => void;
  onSave: (editor?: StepEditorState) => void;
}) {
  const [stepSearch, setStepSearch] = useState('');
  const [nestedEditor, setNestedEditor] = useState<NestedStepEditorState | null>(null);
  const schema = schemas.find((item) => item.step_type === editor.stepType);
  const schemaObject = asSchema(schema?.schema);
  const config = parseConfigText(editor.configText);
  const orderedProperties = orderedSchemaProperties(schemaObject, schemaObject);
  const normalizedSearch = stepSearch.trim().toLowerCase();
  const availableSchemas = [...schemas]
    .filter((item) => stepSchemaSupportsJourneyType(item, normalizeJourneyType(journey?.journey_type)))
    .filter((item) => {
      if (!normalizedSearch) return true;
      return `${item.step_type} ${stepSchemaDescription(item)}`.toLowerCase().includes(normalizedSearch);
    })
    .sort((left, right) => left.step_type.localeCompare(right.step_type));

  const updateConfig = (key: string, value: unknown) => {
    onChange({
      ...editor,
      configText: JSON.stringify({ ...config, [key]: value }, null, 2),
    });
  };

  const updateOutcome = (nextOutcome: Record<string, string>) => {
    updateConfig('outcome', nextOutcome);
  };

  const updateScriptID = (value: unknown) => {
    const scriptID = typeof value === 'string' ? value : '';
    const selected = scripts.find((script) => script.id === scriptID);
    const outcomes = syncScriptOutcomeTargets(config.outcome, scriptOutcomes(selected));
    onChange({
      ...editor,
      configText: JSON.stringify({ ...config, script_id: scriptID, outcome: outcomes }, null, 2),
    });
  };

  const updateNestedStep = (key: string, index: number, step: JourneyStep) => {
    const rows = Array.isArray(config[key]) ? [...(config[key] as unknown[])] : [];
    rows[index] = step;
    onChange({
      ...editor,
      configText: JSON.stringify({ ...config, [key]: rows }, null, 2),
    });
  };

  const removeNestedStep = (key: string, index: number) => {
    const rows = Array.isArray(config[key]) ? [...(config[key] as unknown[])] : [];
    rows.splice(index, 1);
    onChange({
      ...editor,
      configText: JSON.stringify({ ...config, [key]: rows }, null, 2),
    });
    if (nestedEditor?.fieldKey === key && nestedEditor.index === index) setNestedEditor(null);
  };

  const selectStepType = (stepType: string) => {
    const selectedSchema = schemas.find((item) => item.step_type === stepType);
    onChange({
      ...editor,
      stepType,
      stepName: editor.mode === 'add' ? stepType : editor.stepName,
      stepID: editor.mode === 'add' ? newStepID(journey?.steps || {}) : editor.stepID,
      connectOutcome: editor.mode === 'add' ? editor.connectOutcome || 'true' : editor.connectOutcome,
      configText: JSON.stringify(defaultConfigForSchema(selectedSchema), null, 2),
    });
  };

  if (editor.mode === 'add' && !editor.stepType) {
    return (
      <div className="motion-panel flex h-full flex-col overflow-hidden rounded-3xl border border-[var(--color-blue-border)] bg-[var(--color-surface-overlay)] shadow-2xl shadow-[var(--color-muted-faint)] backdrop-blur">
        <div className="border-b border-[var(--color-border-soft)] p-5">
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.2em] text-[var(--color-blue)]">Add step</p>
              <h2 className="mt-1 text-xl font-semibold tracking-tight">Choose a step type</h2>
              {editor.sourceStepID && (
                <p className="mt-1 text-sm text-[var(--color-muted)]">
                  New step from{' '}
                  <span className="font-semibold text-[var(--color-ink)]">
                    {stepDisplayName(journey, editor.sourceStepID)}
                  </span>
                  {editor.connectOutcome ? (
                    <span>
                      {' '}
                      / <span className="font-semibold text-[var(--color-ink)]">{editor.connectOutcome}</span>
                    </span>
                  ) : null}
                  .
                </p>
              )}
            </div>
            <Button onClick={onCancel} variant="secondary" size="sm">
              Close
            </Button>
          </div>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto p-4">
          <input
            value={stepSearch}
            onChange={(event) => setStepSearch(event.target.value)}
            placeholder="Search step type..."
            className="mb-3 w-full rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-4 py-3 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]"
          />
          <div className="grid gap-3">
            {availableSchemas.map((item) => (
              <button
                type="button"
                key={item.step_type}
                onClick={() => selectStepType(item.step_type)}
                className="rounded-2xl border border-[var(--color-border-soft)] bg-[var(--color-surface-muted-transparent)] p-4 text-left transition hover:border-[var(--color-blue-border)] hover:bg-[var(--color-surface-muted-transparent)] hover:shadow-sm"
              >
                <span className="text-xs font-semibold uppercase tracking-[0.16em] text-[var(--color-blue)]">
                  {formatStepTypeLabel(item.step_type)}
                </span>
                <p className="mt-1 text-sm leading-5 text-[var(--color-muted)]">{stepSchemaDescription(item)}</p>
              </button>
            ))}
          </div>
        </div>
      </div>
    );
  }

  if (nestedEditor) {
    const rows = Array.isArray(config[nestedEditor.fieldKey]) ? (config[nestedEditor.fieldKey] as unknown[]) : [];
    const nestedStep = normalizeEmbeddedStep(rows[nestedEditor.index]);
    return (
      <NestedStepEditorPanel
        realm={realm}
        step={nestedStep}
        index={nestedEditor.index}
        schemas={schemas}
        scripts={scripts}
        journeyType={normalizeJourneyType(journey?.journey_type)}
        parentTitle={editor.stepName || editor.stepType || 'Parent step'}
        onBack={() => setNestedEditor(null)}
        onChange={(step) => updateNestedStep(nestedEditor.fieldKey, nestedEditor.index, step)}
      />
    );
  }

  return (
    <div
      className="motion-panel flex h-full flex-col overflow-hidden rounded-3xl border border-[var(--color-blue-border)] bg-[var(--color-surface-overlay)] shadow-2xl shadow-[var(--color-muted-faint)] backdrop-blur"
      data-step-editor-panel
    >
      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className="grid gap-0">
          <aside className="border-b border-[var(--color-border-soft)] bg-[var(--color-surface-muted-transparent)] p-5">
            <p className="text-xs font-semibold uppercase tracking-[0.2em] text-[var(--color-blue)]">
              {editor.mode === 'add' ? 'Add step' : 'Configure step'}
            </p>
            <h2 className="mt-1 text-xl font-semibold tracking-tight">
              {editor.mode === 'add' ? 'New step' : editor.stepName || editor.stepType || 'Step'}
            </h2>
            {editor.sourceStepID && (
              <p className="mt-2 text-sm text-[var(--color-muted)]">
                Connect from{' '}
                <span className="font-semibold text-[var(--color-ink)]">
                  {stepDisplayName(journey, editor.sourceStepID)}
                </span>
                {editor.connectOutcome ? (
                  <span>
                    {' '}
                    / <span className="font-semibold text-[var(--color-ink)]">{editor.connectOutcome}</span>
                  </span>
                ) : null}
              </p>
            )}

            <div className="mt-5 space-y-3">
              <label className="block text-xs font-semibold uppercase tracking-[0.12em] text-[var(--color-muted-soft)]">
                Label
                <input
                  value={editor.stepName}
                  onChange={(event) => onChange({ ...editor, stepName: event.target.value })}
                  className="mt-1 w-full rounded-2xl border border-transparent bg-[var(--color-surface)] px-3 py-2.5 text-sm font-semibold normal-case tracking-normal text-[var(--color-ink)] shadow-sm outline-none ring-1 ring-[var(--color-border-subtle)] focus:ring-2 focus:ring-[var(--color-blue)]"
                />
              </label>
              <div className="block text-xs font-semibold uppercase tracking-[0.12em] text-[var(--color-muted-soft)]">
                Type
                <div className="mt-1 rounded-2xl bg-[var(--color-surface)] px-3 py-2.5 text-sm font-semibold normal-case tracking-normal text-[var(--color-blue)] shadow-sm ring-1 ring-[var(--color-border-subtle)]">
                  {editor.stepType}
                </div>
              </div>
              {editor.mode === 'add' && editor.sourceStepID && (
                <div className="block text-xs font-semibold uppercase tracking-[0.12em] text-[var(--color-muted-soft)]">
                  Source outcome
                  <div className="mt-1 rounded-2xl bg-[var(--color-surface)] px-3 py-2.5 text-sm font-semibold normal-case tracking-normal text-[var(--color-blue)] shadow-sm ring-1 ring-[var(--color-border-subtle)]">
                    {editor.connectOutcome || 'Selected outcome'}
                  </div>
                </div>
              )}
            </div>
          </aside>

          <div className="p-5">
            <div className="grid gap-4 lg:grid-cols-2">
              {orderedProperties
                .filter(([key]) => key !== 'outcome')
                .map(([key, property]) => (
                  <SchemaField
                    key={key}
                    realm={realm}
                    name={key}
                    schema={property}
                    rootSchema={schemaObject}
                    value={config[key]}
                    config={config}
                    schemas={schemas}
                    journey={journey}
                    required={schemaObject?.required?.includes(key)}
                    onChange={(value) =>
                      editor.stepType === 'Script' && key === 'script_id'
                        ? updateScriptID(value)
                        : updateConfig(key, value)
                    }
                    onEditNestedStep={(index) => setNestedEditor({ fieldKey: key, index })}
                    onRemoveNestedStep={(index) => removeNestedStep(key, index)}
                  />
                ))}
            </div>

            {editor.stepType === 'Script' && (
              <DeclaredOutcomesSummary
                outcomes={scriptOutcomes(scripts.find((script) => script.id === config.script_id))}
              />
            )}

            {editor.stepType !== 'Script' &&
              schemaObject?.properties?.outcome &&
              isDynamicOutcomeSchema(schemaObject.properties.outcome) && (
                <OutcomeEditor
                  schema={schemaObject.properties.outcome}
                  outcomes={objectRecord(config.outcome)}
                  onChange={updateOutcome}
                />
              )}

            <details className="mt-5 rounded-2xl bg-[var(--color-surface-subtle)] p-4">
              <summary className="cursor-pointer text-sm font-semibold text-[var(--color-muted)]">
                Raw config JSON
              </summary>
              <HighlightedTextEditor
                value={editor.configText}
                onChange={(value) => onChange({ ...editor, configText: value })}
                language="json"
                multiline
                className="mt-3 min-h-48"
              />
            </details>

            <div className="mt-5 flex justify-end gap-2">
              <Button onClick={onCancel} variant="ghost">
                Cancel
              </Button>
              {editor.mode === 'add' && (
                <Button onClick={() => onSave(editor)} variant="primary">
                  Add step
                </Button>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function DeclaredOutcomesSummary({ outcomes }: { outcomes: string[] }) {
  return (
    <section className="mt-5 rounded-2xl bg-[var(--color-surface-subtle)] p-4 ring-1 ring-[var(--color-border-soft)]">
      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-[var(--color-muted-soft)]">
        Script outcomes
      </p>
      {outcomes.length > 0 ? (
        <div className="mt-2 flex flex-wrap gap-2">
          {outcomes.map((outcome) => (
            <span
              key={outcome}
              className="rounded-full bg-[var(--color-blue-soft)] px-2.5 py-1 font-mono text-xs font-semibold text-[var(--color-blue)]"
            >
              {outcome}
            </span>
          ))}
        </div>
      ) : (
        <p className="mt-2 text-sm text-[var(--color-muted)]">Select a script with declared outcomes.</p>
      )}
      <p className="mt-2 text-xs text-[var(--color-muted)]">
        These values come from the selected script and cannot be edited in the step.
      </p>
    </section>
  );
}
