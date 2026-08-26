import { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import { FileCode2, LockKeyhole, Maximize2, PanelLeftClose, PanelLeftOpen } from 'lucide-react';
import { getScript, listScriptBindings, saveScript } from '../../api/journeyApi';
import { IconButton } from '../../components/Button';
import { TypeOptionPicker } from '../../components/TypeOptionPicker';
import { TagsInput } from '../../components/TagsInput';
import { normalizeScriptType, scriptTypeOptionsFromBindingSets, type TypeOption } from '../../config/typeOptions';
import type { JourneyScript, ScriptBindingDescriptor, ScriptBindingSet } from '../../types/journey';
import { ScriptArgsEditor } from './ScriptArgsEditor';
import { ScriptCodeEditor } from './ScriptCodeEditor';
import { normalizeScriptOutcomes, supportsScriptOutcomes } from './scriptOutcomes';
import { ChangesIcon, DeleteIcon, SaveIcon, SavedIcon, SpinnerIcon } from '../../components/Icons';

type ScriptEditorPanelProps = {
  realm: string;
  draft: JourneyScript;
  code: string;
  selectedID?: string;
  dirty: boolean;
  saving: boolean;
  error?: string;
  onDraftChange: (next: Partial<JourneyScript>) => void;
  onCodeChange: (next: string) => void;
  onSave: () => void;
  onDelete?: () => void;
  scriptTypeOptions?: TypeOption[];
  bindingSets?: ScriptBindingSet[];
};

export function ScriptEditorPanel(props: ScriptEditorPanelProps) {
  const [localBindingSets, setLocalBindingSets] = useState<ScriptBindingSet[]>([]);
  const [schemaOpen, setSchemaOpen] = useState(false);
  const [codeModalOpen, setCodeModalOpen] = useState(false);
  const bindingSets = props.bindingSets || localBindingSets;
  const typeOptions = props.scriptTypeOptions || scriptTypeOptionsFromBindingSets(bindingSets);
  const existingScript = Boolean(props.draft.id || props.selectedID);
  const bindings = [...bindingsForScriptType(bindingSets, normalizeScriptType(props.draft.type))];

  (props.draft.additional_props?.outcomes || []).forEach((outcome) =>
    bindings.push({
      name: `setOutcome("${outcome}")`,
      type: 'function',
      example: `setOutcome("${outcome}")`,
    })
  );

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const response = await listScriptBindings(props.realm);
        if (!cancelled) setLocalBindingSets(response.result || []);
      } catch {
        if (!cancelled) setLocalBindingSets([]);
      }
    };
    if (props.bindingSets) return undefined;
    void load();
    return () => {
      cancelled = true;
    };
  }, [props.realm, props.bindingSets]);

  return (
    <main className="flex min-w-0 flex-1 flex-col overflow-hidden rounded-3xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-sm">
      <ScriptEditorHeader
        {...props}
        existingScript={existingScript}
        scriptTypeOptions={typeOptions}
        schemaOpen={schemaOpen}
        onToggleSchema={() => setSchemaOpen((current) => !current)}
        onOpenCodeModal={() => setCodeModalOpen(true)}
      />
      {props.error && (
        <div className="m-5 rounded-2xl border border-[var(--color-red)] bg-[var(--color-red-soft)] px-4 py-3 text-sm text-[var(--color-red)]">
          {props.error}
        </div>
      )}
      <div
        className={[
          'grid min-h-0 flex-1 gap-5 overflow-hidden p-5',
          schemaOpen ? 'xl:grid-cols-[minmax(360px,34vw)_minmax(0,1fr)]' : 'grid-cols-1',
        ].join(' ')}
      >
        {schemaOpen && (
          <aside className="min-h-0 overflow-auto rounded-3xl border border-[var(--color-border)] bg-[var(--color-surface-muted-transparent)] p-4">
            <ScriptArgsEditor args={props.draft.args || []} onChange={(args) => props.onDraftChange({ args })} />
            {supportsScriptOutcomes(props.draft.type) && (
              <section className="mt-5 border-t border-[var(--color-border-soft)] pt-5">
                <p className="text-xs font-bold uppercase tracking-[0.24em] text-[var(--color-muted-soft)]">Outcomes</p>
                <p className="mt-1 text-xs leading-5 text-[var(--color-muted)]">
                  Available outcomes for Script steps. Values are stored in lowercase.
                </p>
                <TagsInput
                  value={normalizeScriptOutcomes(props.draft.additional_props?.outcomes)}
                  onChange={(values) =>
                    props.onDraftChange({
                      additional_props: {
                        ...(props.draft.additional_props || {}),
                        outcomes: normalizeScriptOutcomes(values),
                      },
                    })
                  }
                  placeholder="Type outcome and press Enter…"
                  className="mt-3"
                />
              </section>
            )}
          </aside>
        )}
        <div className="h-full min-h-[540px] min-w-0 overflow-hidden">
          <ScriptCodeEditor code={props.code} bindings={bindings} onChange={props.onCodeChange} />
        </div>
      </div>
      {codeModalOpen && (
        <CodeEditorModal
          title={props.draft.name || 'Script'}
          code={props.code}
          bindings={bindings}
          onChange={props.onCodeChange}
          onClose={() => setCodeModalOpen(false)}
        />
      )}
    </main>
  );
}

function bindingsForScriptType(sets: ScriptBindingSet[], scriptType?: string): ScriptBindingDescriptor[] {
  return sets.find((set) => normalizeScriptType(set.type) === normalizeScriptType(scriptType))?.bindings || [];
}

function ScriptEditorHeader({
  draft,
  selectedID = '',
  dirty,
  saving,
  onDraftChange,
  onSave,
  onDelete,
  existingScript,
  scriptTypeOptions = [],
  schemaOpen,
  onToggleSchema,
  onOpenCodeModal,
}: ScriptEditorPanelProps & {
  existingScript: boolean;
  scriptTypeOptions: TypeOption[];
  schemaOpen: boolean;
  onToggleSchema: () => void;
  onOpenCodeModal: () => void;
}) {
  const normalizedType = normalizeScriptType(draft.type);
  const selectedType = scriptTypeOptions.find((option) => normalizeScriptType(option.value) === normalizedType);
  const declaresOutcomes = supportsScriptOutcomes(normalizedType);
  const modeLabel = existingScript ? 'Editing script' : 'New script';
  const title = existingScript ? draft.name || selectedID || 'Script' : 'Create script';

  return (
    <header className="flex shrink-0 flex-col gap-5 border-b border-[var(--color-border-soft)] p-5 xl:flex-row xl:items-start xl:justify-between">
      <div className="min-w-0 flex-1">
        <p className="text-xs font-bold uppercase tracking-[0.32em] text-[var(--color-blue)]">{modeLabel}</p>
        <div className="mt-2 flex flex-wrap items-center gap-3">
          <h1 className="truncate text-2xl font-bold text-[var(--color-ink)]">{title}</h1>
          {existingScript && (
            <span className="rounded-full bg-[var(--color-surface-soft)] px-3 py-1 font-mono text-xs font-bold text-[var(--color-muted)] ring-1 ring-[var(--color-border-soft)]">
              {normalizedType}
            </span>
          )}
        </div>
        <p className="mt-2 max-w-3xl text-sm text-[var(--color-muted)]">
          {existingScript
            ? declaresOutcomes
              ? 'Script type is fixed after creation. Edit the name, args, declared outcomes and JavaScript below.'
              : 'Script type is fixed after creation. Edit the name, args schema and JavaScript below.'
            : declaresOutcomes
            ? 'Choose the runtime family first, then define the script code, args and declared outcomes.'
            : 'Choose the runtime family first, then define the script code and optional args schema.'}
        </p>
        {existingScript && selectedID && (
          <p className="mt-2 font-mono text-xs text-[var(--color-muted-soft)]">{selectedID}</p>
        )}
        <div className="mt-5 grid gap-4">
          <label className="grid gap-1">
            <span className="text-xs font-bold uppercase tracking-[0.24em] text-[var(--color-muted-soft)]">Name</span>
            <input
              value={draft.name}
              onChange={(event) => onDraftChange({ name: event.target.value })}
              placeholder="Script name"
              className="rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-4 py-3 text-base font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]"
            />
          </label>
          {existingScript ? (
            <ReadonlyScriptTypeSummary option={selectedType} value={normalizedType} />
          ) : (
            <TypeOptionPicker
              label="Type"
              value={normalizedType}
              options={scriptTypeOptions}
              searchable
              pageSize={6}
              onChange={(type) => {
                const additionalProps = { ...(draft.additional_props || {}) };
                if (supportsScriptOutcomes(type)) {
                  additionalProps.outcomes = normalizeScriptOutcomes(additionalProps.outcomes);
                  if (additionalProps.outcomes.length === 0) additionalProps.outcomes = ['true'];
                } else {
                  delete additionalProps.outcomes;
                }
                onDraftChange({
                  type,
                  additional_props: Object.keys(additionalProps).length > 0 ? additionalProps : undefined,
                });
              }}
            />
          )}
        </div>
      </div>
      <div className="flex shrink-0 items-center justify-end gap-2">
        <IconButton
          onClick={onToggleSchema}
          label={schemaOpen ? 'Hide script schema' : declaresOutcomes ? 'Show args and outcomes' : 'Show args schema'}
          variant="secondary"
          size="md"
        >
          {schemaOpen ? <PanelLeftClose size={16} /> : <PanelLeftOpen size={16} />}
        </IconButton>
        <IconButton onClick={onOpenCodeModal} label="Expand script editor" variant="secondary" size="md">
          <Maximize2 size={16} />
        </IconButton>
        <span
          title={dirty ? 'Unsaved changes' : 'Saved'}
          aria-label={dirty ? 'Unsaved changes' : 'Saved'}
          className={[
            'inline-flex h-9 w-9 items-center justify-center rounded-xl border',
            dirty
              ? 'border-[var(--color-warning-border)] bg-[var(--color-warning-soft)] text-[var(--color-warning)]'
              : 'border-[var(--color-border-soft)] bg-[var(--color-surface-soft)] text-[var(--color-muted)]',
          ].join(' ')}
        >
          {dirty ? <ChangesIcon /> : <SavedIcon />}
        </span>
        {onDelete && (
          <IconButton
            onClick={onDelete}
            disabled={!selectedID || saving}
            label="Delete script"
            variant="danger"
            size="md"
          >
            <DeleteIcon />
          </IconButton>
        )}
        <IconButton onClick={onSave} disabled={saving || !dirty} label="Save script" variant="primary" size="md">
          {saving ? <SpinnerIcon /> : <SaveIcon />}
        </IconButton>
      </div>
    </header>
  );
}

function ReadonlyScriptTypeSummary({ option, value }: { option?: TypeOption; value: string }) {
  const Icon = option?.icon || FileCode2;
  return (
    <section className="grid gap-2">
      <p className="text-xs font-bold uppercase tracking-[0.24em] text-[var(--color-muted-soft)]">Type</p>
      <div className="rounded-3xl border border-[var(--color-border-soft)] bg-[var(--color-surface-subtle)] p-4">
        <div className="flex items-start gap-3">
          <span className="grid h-10 w-10 shrink-0 place-items-center rounded-2xl border border-[var(--color-border-soft)] bg-[var(--color-surface-soft)] text-[var(--color-blue)]">
            <Icon size={20} strokeWidth={2.2} aria-hidden="true" />
          </span>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h3 className="text-sm font-bold text-[var(--color-ink)]">{option?.title || value}</h3>
              <span className="inline-flex items-center gap-1 rounded-full bg-[var(--color-surface-soft)] px-2 py-0.5 text-[10px] font-bold uppercase tracking-[0.16em] text-[var(--color-muted-soft)]">
                <LockKeyhole size={11} aria-hidden="true" />
                fixed
              </span>
            </div>
            <p className="mt-1.5 text-xs leading-5 text-[var(--color-muted)]">
              {option?.description || 'Script type cannot be changed after creation.'}
            </p>
          </div>
        </div>
        <p className="mt-3 truncate font-mono text-[11px] font-semibold text-[var(--color-blue)]">{value}</p>
      </div>
    </section>
  );
}

function CodeEditorModal({
  title,
  code,
  bindings,
  onChange,
  onClose,
}: {
  title: string;
  code: string;
  bindings: ScriptBindingDescriptor[];
  onChange: (code: string) => void;
  onClose: () => void;
}) {
  const [closing, setClosing] = useState(false);

  function requestClose() {
    if (closing) return;
    setClosing(true);
    window.setTimeout(onClose, 140);
  }

  return createPortal(
    <div
      className="motion-modal-backdrop fixed inset-0 z-[1000] flex items-center justify-center bg-[var(--color-overlay)] p-6"
      data-closing={closing ? 'true' : undefined}
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) requestClose();
      }}
    >
      <div
        className="motion-modal-surface flex h-[min(900px,94vh)] w-[min(1500px,96vw)] flex-col overflow-hidden rounded-[2rem] bg-[var(--color-surface)] shadow-2xl"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="flex shrink-0 items-center justify-between border-b border-[var(--color-border-soft)] px-6 py-5">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.24em] text-[var(--color-blue)]">Script editor</p>
            <h2 className="mt-1 text-lg font-semibold text-[var(--color-ink)]">{title}</h2>
          </div>
        </div>
        <div className="min-h-0 flex-1 bg-[var(--color-surface-muted-transparent)] p-6">
          <ScriptCodeEditor code={code} bindings={bindings} onChange={onChange} />
        </div>
      </div>
    </div>,
    document.body
  );
}

export function ScriptEditorModal({
  realm,
  scriptId,
  onClose,
  onSaved,
}: {
  realm: string;
  scriptId: string;
  onClose: () => void;
  onSaved?: (script: JourneyScript) => void;
}) {
  const [draft, setDraft] = useState<JourneyScript>(() => newScript());
  const [code, setCode] = useState('');
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [closing, setClosing] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      setError('');
      try {
        const script = await getScript(realm, scriptId);
        if (cancelled) return;
        setDraft(script);
        setCode(decodeBase64(script.code_base64));
        setDirty(false);
      } catch (err) {
        if (!cancelled) setError(errorMessage(err));
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
  }, [realm, scriptId]);

  function updateDraft(next: Partial<JourneyScript>) {
    setDraft((current) => ({ ...current, ...next }));
    setDirty(true);
  }

  async function persistScript() {
    const name = draft.name.trim();
    if (!name || !code.trim()) {
      setError(!name ? 'Script name is required.' : 'Script code is required.');
      return;
    }
    setSaving(true);
    setError('');
    try {
      const saved = await saveScript(realm, {
        ...draft,
        id: draft.id || scriptId,
        name,
        type: normalizeScriptType(draft.type),
        code_base64: encodeBase64(code),
      });
      setDraft(saved);
      setCode(decodeBase64(saved.code_base64));
      setDirty(false);
      onSaved?.(saved);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  function requestClose() {
    if (closing) return;
    setClosing(true);
    window.setTimeout(onClose, 140);
  }

  return createPortal(
    <div
      className="motion-modal-backdrop fixed inset-0 z-[1000] flex items-center justify-center bg-[var(--color-overlay)] p-6"
      data-closing={closing ? 'true' : undefined}
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) requestClose();
      }}
    >
      <div
        className="motion-modal-surface flex h-[min(780px,92vh)] w-[min(1120px,94vw)] flex-col overflow-hidden rounded-[2rem] bg-[var(--color-surface)] shadow-2xl"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="flex shrink-0 items-center justify-between border-b border-[var(--color-border-soft)] px-6 py-5">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-[var(--color-blue)]">Script editor</p>
            <h2 className="text-lg font-semibold text-[var(--color-ink)]">{draft.name || 'Script'}</h2>
          </div>
        </div>
        <div className="flex min-h-0 flex-1 overflow-auto bg-[var(--color-surface-muted-transparent)] p-6">
          <ScriptEditorPanel
            realm={realm}
            draft={draft}
            code={code}
            selectedID={scriptId}
            dirty={dirty}
            saving={saving}
            error={error}
            onDraftChange={updateDraft}
            onCodeChange={(next) => {
              setCode(next);
              setDirty(true);
            }}
            onSave={persistScript}
          />
        </div>
      </div>
    </div>,
    document.body
  );
}

export function newScript(): JourneyScript {
  return {
    name: '',
    type: 'auth',
    code_base64: encodeBase64(defaultCode('auth')),
    args: [],
    additional_props: { outcomes: ['true'] },
  };
}

export function defaultCode(type: string) {
  if (type === 'library') return `function helper(value) {\n  return value\n}\n\nmodule.exports = { helper }\n`;
  return `try {\n  // ctx.Set("example", true)\n  logger.Info("script completed")\n  setOutcome("true")\n} catch (err) {\n  logger.Error("script failed", err)\n}\n`;
}

export function decodeBase64(value: string) {
  if (!value) return '';
  try {
    const binary = atob(value);
    return new TextDecoder().decode(Uint8Array.from(binary, (char) => char.charCodeAt(0)));
  } catch {
    return '';
  }
}

export function encodeBase64(value: string) {
  const bytes = new TextEncoder().encode(value);
  let binary = '';
  bytes.forEach((byte) => {
    binary += String.fromCharCode(byte);
  });
  return btoa(binary);
}

export function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : 'Unexpected error';
}
