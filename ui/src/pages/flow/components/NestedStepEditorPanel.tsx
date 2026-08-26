import { useState } from 'react';
import { IconButton } from '../../../components/Button';
import type { JourneyScript, JourneyStep, StepSchema } from '../../../types/journey';
import type { NestedStepEditorState } from '../flowTypes';
import {
  asRecord,
  asSchema,
  defaultConfigForSchema,
  normalizeEmbeddedStep,
  orderedSchemaProperties,
  stepSchemaDescription,
  stepSchemaSupportsJourneyType,
} from '../utils/schemaUtils';
import { BackMiniIcon } from '../../../components/Icons';
import { SchemaField } from './SchemaField';
import { scriptOutcomes, syncScriptOutcomeTargets } from '../../scripts/scriptOutcomes';
import { formatStepTypeLabel } from '../utils/displayUtils';

export function NestedStepEditorPanel({
  realm,
  step,
  index,
  schemas,
  scripts,
  journeyType,
  parentTitle,
  onBack,
  onChange,
}: {
  realm: string;
  step: JourneyStep;
  index: number;
  schemas: StepSchema[];
  scripts: JourneyScript[];
  journeyType: string;
  parentTitle: string;
  onBack: () => void;
  onChange: (step: JourneyStep) => void;
}) {
  const [search, setSearch] = useState('');
  const [nestedEditor, setNestedEditor] = useState<NestedStepEditorState | null>(null);
  const schema = schemas.find((item) => item.step_type === step.step_type);
  const schemaObject = asSchema(schema?.schema);
  const config = asRecord(step.config);
  const normalizedSearch = search.trim().toLowerCase();
  const availableSchemas = [...schemas]
    .filter((item) => stepSchemaSupportsJourneyType(item, journeyType))
    .filter(
      (item) =>
        !normalizedSearch || `${item.step_type} ${stepSchemaDescription(item)}`.toLowerCase().includes(normalizedSearch)
    )
    .sort((left, right) => left.step_type.localeCompare(right.step_type));

  const setStepType = (stepType: string) => {
    const selectedSchema = schemas.find((item) => item.step_type === stepType);
    onChange({
      name: step.name || stepType,
      step_type: stepType,
      config: defaultConfigForSchema(selectedSchema),
    });
  };

  const updateConfig = (key: string, value: unknown) => {
    onChange({ ...step, config: { ...config, [key]: value } });
  };

  const updateScriptID = (value: unknown) => {
    const scriptID = typeof value === 'string' ? value : '';
    const selected = scripts.find((script) => script.id === scriptID);
    onChange({
      ...step,
      config: {
        ...config,
        script_id: scriptID,
        outcome: syncScriptOutcomeTargets(config.outcome, scriptOutcomes(selected)),
      },
    });
  };

  const updateNestedStep = (key: string, index: number, childStep: JourneyStep) => {
    const rows = Array.isArray(config[key]) ? [...(config[key] as unknown[])] : [];
    rows[index] = childStep;
    onChange({ ...step, config: { ...config, [key]: rows } });
  };

  const removeNestedStep = (key: string, index: number) => {
    const rows = Array.isArray(config[key]) ? [...(config[key] as unknown[])] : [];
    rows.splice(index, 1);
    onChange({ ...step, config: { ...config, [key]: rows } });
    if (nestedEditor?.fieldKey === key && nestedEditor.index === index) setNestedEditor(null);
  };

  if (nestedEditor) {
    const rows = Array.isArray(config[nestedEditor.fieldKey]) ? (config[nestedEditor.fieldKey] as unknown[]) : [];
    return (
      <NestedStepEditorPanel
        realm={realm}
        step={normalizeEmbeddedStep(rows[nestedEditor.index])}
        index={nestedEditor.index}
        schemas={schemas}
        scripts={scripts}
        journeyType={journeyType}
        parentTitle={step.name || step.step_type || parentTitle}
        onBack={() => setNestedEditor(null)}
        onChange={(childStep) => updateNestedStep(nestedEditor.fieldKey, nestedEditor.index, childStep)}
      />
    );
  }

  return (
    <div
      className="motion-panel flex h-full flex-col overflow-hidden rounded-3xl border border-[var(--color-blue-border)] bg-[var(--color-surface-overlay)] shadow-2xl shadow-[var(--color-muted-faint)] backdrop-blur"
      data-step-editor-panel
    >
      <div className="flex items-start gap-3 border-b border-[var(--color-border-soft)] bg-[var(--color-surface-muted-transparent)] p-5">
        <IconButton onClick={onBack} label="Return to parent step" variant="secondary" size="sm">
          <BackMiniIcon />
        </IconButton>
        <div className="min-w-0 flex-1">
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-[var(--color-blue)]">Embedded step</p>
          <h2 className="mt-1 text-xl font-semibold tracking-tight">
            {step.name || step.step_type || `Step ${index + 1}`}
          </h2>
          <p className="mt-1 text-sm text-[var(--color-muted)]">
            Inside <span className="font-semibold text-[var(--color-ink)]">{parentTitle}</span>. Configure this child,
            then return to parent.
          </p>
        </div>
      </div>

      {!step.step_type ? (
        <div className="min-h-0 flex-1 overflow-y-auto p-4">
          <input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search embedded step type..."
            className="mb-3 w-full rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-4 py-3 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]"
          />
          <div className="grid gap-3">
            {availableSchemas.map((item) => (
              <button
                type="button"
                key={item.step_type}
                onClick={() => setStepType(item.step_type)}
                className="rounded-2xl border border-[var(--color-border-soft)] bg-[var(--color-surface-muted-transparent)] p-4 text-left transition hover:border-[var(--color-blue-border)] hover:shadow-sm"
              >
                <span className="text-xs font-semibold uppercase tracking-[0.16em] text-[var(--color-blue)]">
                  {formatStepTypeLabel(item.step_type)}
                </span>
                <p className="mt-1 text-sm leading-5 text-[var(--color-muted)]">{stepSchemaDescription(item)}</p>
              </button>
            ))}
          </div>
        </div>
      ) : (
        <div className="min-h-0 flex-1 overflow-y-auto">
          <aside className="border-b border-[var(--color-border-soft)] bg-[var(--color-surface-muted-transparent)] p-5">
            <div className="space-y-3">
              <label className="block text-xs font-semibold uppercase tracking-[0.12em] text-[var(--color-muted-soft)]">
                Label
                <input
                  value={step.name || ''}
                  onChange={(event) => onChange({ ...step, name: event.target.value })}
                  className="mt-1 w-full rounded-2xl border border-transparent bg-[var(--color-surface)] px-3 py-2.5 text-sm font-semibold normal-case tracking-normal text-[var(--color-ink)] shadow-sm outline-none ring-1 ring-[var(--color-border-subtle)] focus:ring-2 focus:ring-[var(--color-blue)]"
                />
              </label>
              <div className="block text-xs font-semibold uppercase tracking-[0.12em] text-[var(--color-muted-soft)]">
                Type
                <div className="mt-1 rounded-2xl bg-[var(--color-surface)] px-3 py-2.5 text-sm font-semibold normal-case tracking-normal text-[var(--color-blue)] shadow-sm ring-1 ring-[var(--color-border-subtle)]">
                  {step.step_type}
                </div>
              </div>
            </div>
          </aside>
          <div className="grid gap-4 p-5 lg:grid-cols-2">
            {orderedSchemaProperties(schemaObject, schemaObject)
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
                  required={schemaObject?.required?.includes(key)}
                  onChange={(value) =>
                    step.step_type === 'Script' && key === 'script_id'
                      ? updateScriptID(value)
                      : updateConfig(key, value)
                  }
                  onEditNestedStep={(index) => setNestedEditor({ fieldKey: key, index })}
                  onRemoveNestedStep={(index) => removeNestedStep(key, index)}
                />
              ))}
          </div>
        </div>
      )}
    </div>
  );
}
