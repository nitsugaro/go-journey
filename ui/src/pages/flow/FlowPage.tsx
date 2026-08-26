import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ReactFlowProvider } from '@xyflow/react';
import { useNavigate, useParams } from 'react-router-dom';
import { deleteJourney, getJourney, listJourneys, listScripts, listStepSchemas, saveJourney } from '../../api/journeyApi';
import { DeleteConfirmModal } from '../../components/DeleteConfirmModal';
import type { JourneyConfiguration, JourneyScript, StepSchema } from '../../types/journey';
import { buildJourneyFlow } from './journeyFlowLayout';
import { defaultEndJourneyTypes, historyLimit } from './flowConstants';
import type { JourneyFilters, JourneyHistory, NewJourneyForm } from './flowTypes';
import { JourneyCanvas } from './components/JourneyCanvas';
import { JourneyCanvasHeader } from './components/JourneyCanvasHeader';
import { JourneyCreateCopyModal } from './components/JourneyCreateCopyModal';
import { JourneyListView, ErrorMessage } from './components/JourneyListView';
import { JourneySettingsModal } from './components/JourneySettingsModal';
import { isEditableKeyboardTarget } from '../../utils/dom';
import { createJourneyPayload, defaultNewJourneyForm } from './utils/journeyStateUtils';
import { cloneJourney, extractEndJourneyTypes, journeyNoteCounts, normalizeJourneyNotes } from './utils/journeyRuntimeUtils';
import { normalizeJourneyType, stepSchemaDynamicOutcomeTypes, stepSchemaOutcomesByType } from './utils/schemaUtils';
import { useJourneyEditing } from './hooks/useJourneyEditing';
import { scriptOutcomes } from '../scripts/scriptOutcomes';
import {
  exportJourneyJSON,
  journeyExportFilename,
  parseJourneyImport,
  prepareJourneyCreation,
  suggestJourneyCopyName,
} from './utils/journeyTransferUtils';

type JourneyCreationDraft = {
  mode: 'duplicate' | 'import';
  source: JourneyConfiguration;
  name: string;
  description: string;
};

export function FlowPage() {
  const route = useFlowRoute();
  const navigate = useNavigate();
  const [journeys, setJourneys] = useState<JourneyConfiguration[]>([]);
  const [journey, setJourney] = useState<JourneyConfiguration | null>(null);
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [creating, setCreating] = useState(false);
  const [creatingCopy, setCreatingCopy] = useState(false);
  const [creationDraft, setCreationDraft] = useState<JourneyCreationDraft | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [journeySettingsOpen, setJourneySettingsOpen] = useState(false);
  const [listSettingsTarget, setListSettingsTarget] = useState<JourneyConfiguration | null>(null);
  const [listSettingsSaving, setListSettingsSaving] = useState(false);
  const [listSettingsError, setListSettingsError] = useState('');
  const [newJourney, setNewJourney] = useState<NewJourneyForm>(() => defaultNewJourneyForm());
  const [journeySearch, setJourneySearch] = useState('');
  const [journeyFilters, setJourneyFilters] = useState<JourneyFilters>(() => defaultJourneyFilters());
  const [deleteTargetJourney, setDeleteTargetJourney] = useState<JourneyConfiguration | null>(null);
  const [stepSchemas, setStepSchemas] = useState<StepSchema[]>([]);
  const [scripts, setScripts] = useState<JourneyScript[]>([]);
  const [endJourneyTypes, setEndJourneyTypes] = useState(defaultEndJourneyTypes);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const journeyRef = useRef<JourneyConfiguration | null>(null);
  const historyRef = useRef<JourneyHistory>({ past: [], future: [] });

  const resetHistory = () => { historyRef.current = { past: [], future: [] }; };
  const updateJourney = useJourneyUpdater(setJourney, setDirty, journeyRef, historyRef);
  const editor = useJourneyEditing({ realm: route.realm, journey, journeyStack: route.journeyStack, updateJourney, setError });

  useEffect(() => { journeyRef.current = journey; }, [journey]);
  useStepSchemas(setStepSchemas, setEndJourneyTypes);
  useFlowScripts(route.realm, setScripts);
  useJourneyList(route.realm, setJourneys, setLoading, setError);
  useSelectedJourney(route.realm, route.journeyId, setJourney, setDirty, setLoading, setError, journeyRef, resetHistory, editor);
  useJourneyHistoryKeys(journeyRef, historyRef, setJourney, setDirty, editor.setConnectDraft);

  const staticOutcomesByStepType = useMemo(() => stepSchemaOutcomesByType(stepSchemas), [stepSchemas]);
  const dynamicOutcomeStepTypes = useMemo(() => stepSchemaDynamicOutcomeTypes(stepSchemas), [stepSchemas]);
  const scriptOutcomesByID = useMemo(() => new Map(
    scripts.flatMap((script) => script.id && Array.isArray(script.additional_props?.outcomes)
      ? [[script.id, scriptOutcomes(script)] as const]
      : []),
  ), [scripts]);
  const staticOutcomesByStepID = useMemo(() => {
    const outcomes = new Map<string, string[]>();
    for (const [stepID, step] of Object.entries(journey?.steps || {})) {
      if (step.step_type !== 'Script') continue;
      const scriptID = typeof step.config?.script_id === 'string' ? step.config.script_id : '';
      const declaredOutcomes = scriptOutcomesByID.get(scriptID);
      if (declaredOutcomes) outcomes.set(stepID, declaredOutcomes);
    }
    return outcomes;
  }, [journey, scriptOutcomesByID]);
  const noteCountsByStep = useMemo(() => journeyNoteCounts(journey?.additional_properties?.notes), [journey?.additional_properties?.notes]);
  const flow = useMemo(() => buildJourneyFlow(journey, flowActions(editor), { endJourneyTypes, highlightedStepID: editor.selectedStepID, staticOutcomesByStepType, staticOutcomesByStepID, dynamicOutcomeStepTypes, noteCountsByStep }), [journey, editor.connectDraft, endJourneyTypes, editor.selectedStepID, staticOutcomesByStepType, staticOutcomesByStepID, dynamicOutcomeStepTypes, noteCountsByStep]);
  const filteredJourneys = useMemo(() => filterJourneys(journeys, journeySearch, journeyFilters), [journeyFilters, journeySearch, journeys]);
  const parentJourneyName = useMemo(() => route.parentJourneyId ? journeys.find((item) => item.id === route.parentJourneyId)?.name || route.parentJourneyId : '', [journeys, route.parentJourneyId]);

  function saveCurrentJourney() {
    if (!journey) return;
    setSaving(true);
    setError('');
    saveJourney(route.realm, journey).then((saved) => {
      setJourney(saved);
      journeyRef.current = saved;
      setDirty(false);
      resetHistory();
      setJourneys((items) => [saved, ...items.filter((item) => item.id !== saved.id)]);
    }).catch((err: Error) => setError(err.message)).finally(() => setSaving(false));
  }

  function createJourney() {
    if (!newJourney.name.trim()) return setError('Journey name is required.');
    setCreating(true);
    setError('');
    saveJourney(route.realm, createJourneyPayload(route.realm, newJourney)).then((saved) => {
      setJourneys((items) => [saved, ...items.filter((item) => item.id !== saved.id)]);
      setJourney(saved);
      journeyRef.current = saved;
      setDirty(false);
      editor.setConnectDraft(null);
      editor.setSelectedStepID('');
      setCreateOpen(false);
      setNewJourney(defaultNewJourneyForm());
      resetHistory();
      if (saved.id) navigate(`/${encodeURIComponent(route.realm)}/flow/${encodeURIComponent(saved.id)}`);
    }).catch((err: Error) => setError(err.message)).finally(() => setCreating(false));
  }

  function openDuplicateJourney(source: JourneyConfiguration) {
    setError('');
    setCreationDraft({
      mode: 'duplicate',
      source: cloneJourney(source),
      name: suggestJourneyCopyName(source.name, journeys.map((item) => item.name)),
      description: source.description || '',
    });
  }

  async function importJourney(file: File) {
    setError('');
    try {
      const source = parseJourneyImport(await file.text(), route.realm);
      setCreationDraft({
        mode: 'import',
        source,
        name: source.name,
        description: source.description || '',
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not import the selected journey.');
    }
  }

  async function confirmCreateCopy(name: string, description: string) {
    if (!creationDraft || !name.trim()) return;
    setCreatingCopy(true);
    setError('');
    try {
      const payload = prepareJourneyCreation(creationDraft.source, route.realm, name, description);
      const saved = await saveJourney(route.realm, payload);
      setJourneys((items) => [saved, ...items.filter((item) => item.id !== saved.id)]);
      setCreationDraft(null);
      setJourney(saved);
      journeyRef.current = saved;
      setDirty(false);
      resetHistory();
      editor.setConnectDraft(null);
      editor.setSelectedStepID('');
      editor.setNotesStepID('');
      if (saved.id) navigate(`/${encodeURIComponent(route.realm)}/flow/${encodeURIComponent(saved.id)}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not create the journey.');
    } finally {
      setCreatingCopy(false);
    }
  }

  function exportJourney(source: JourneyConfiguration) {
    const blob = new Blob([exportJourneyJSON(source)], { type: 'application/json;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = journeyExportFilename(source);
    link.click();
    window.setTimeout(() => URL.revokeObjectURL(url), 0);
  }

  function openListJourneySettings(source: JourneyConfiguration) {
    setListSettingsError('');
    setListSettingsTarget(source);
  }

  async function saveListJourneySettings(next: JourneyConfiguration) {
    setListSettingsSaving(true);
    setListSettingsError('');
    try {
      const saved = await saveJourney(route.realm, next);
      setJourneys((items) => items.map((item) => item.id === saved.id ? saved : item));
      setListSettingsTarget(null);
    } catch (err) {
      setListSettingsError(err instanceof Error ? err.message : 'Could not save journey settings.');
    } finally {
      setListSettingsSaving(false);
    }
  }

  async function confirmDeleteJourney() {
    const target = deleteTargetJourney;
    if (!target?.id) return;
    setSaving(true);
    setError('');
    try {
      await deleteJourney(route.realm, target.id);
      setJourneys((items) => items.filter((item) => item.id !== target.id));
      setDeleteTargetJourney(null);
      if (route.journeyId === target.id) {
        setJourney(null);
        journeyRef.current = null;
        setDirty(false);
        resetHistory();
        editor.setConnectDraft(null);
        editor.setSelectedStepID('');
        editor.setNotesStepID('');
        navigate(`/${encodeURIComponent(route.realm)}/flow`, { replace: true });
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unexpected error');
    } finally {
      setSaving(false);
    }
  }

  if (!route.journeyId) {
    return (
      <>
        <JourneyListView
          realm={route.realm}
          journeys={journeys}
          filteredJourneys={filteredJourneys}
          journeySearch={journeySearch}
          filters={journeyFilters}
          createOpen={createOpen}
          newJourney={newJourney}
          creating={creating}
          loading={loading}
          error={error}
          onSearchChange={setJourneySearch}
          onFiltersChange={setJourneyFilters}
          onToggleCreate={() => setCreateOpen((open) => !open)}
          onNewJourneyChange={setNewJourney}
          onCancelCreate={() => setCreateOpen(false)}
          onCreateJourney={createJourney}
          onOpenJourney={(item) => item.id && navigate(`/${encodeURIComponent(route.realm)}/flow/${encodeURIComponent(item.id)}`)}
          onEditJourney={openListJourneySettings}
          onDuplicateJourney={openDuplicateJourney}
          onExportJourney={exportJourney}
          onImportJourney={importJourney}
          onDeleteJourney={setDeleteTargetJourney}
        />
        <JourneyCreateCopyModal
          open={Boolean(creationDraft)}
          mode={creationDraft?.mode || 'duplicate'}
          realm={route.realm}
          initialName={creationDraft?.name || ''}
          initialDescription={creationDraft?.description || ''}
          saving={creatingCopy}
          error={error}
          onCancel={() => {
            setCreationDraft(null);
            setError('');
          }}
          onConfirm={confirmCreateCopy}
        />
        <JourneySettingsModal
          open={Boolean(listSettingsTarget)}
          realm={route.realm}
          journey={listSettingsTarget}
          saving={listSettingsSaving}
          error={listSettingsError}
          onClose={() => {
            if (listSettingsSaving) return;
            setListSettingsTarget(null);
            setListSettingsError('');
          }}
          onSave={saveListJourneySettings}
        />
        <DeleteConfirmModal open={Boolean(deleteTargetJourney)} itemLabel="Journey" itemName={deleteTargetJourney?.name?.trim() || deleteTargetJourney?.id || ''} confirming={saving} onCancel={() => setDeleteTargetJourney(null)} onConfirm={confirmDeleteJourney} />
      </>
    );
  }

  return (
    <section className="flex h-full min-h-0 flex-col gap-4 overflow-hidden">
      <JourneyCanvasHeader
        realm={route.realm}
        journeyId={route.journeyId}
        journey={journey}
        dirty={dirty}
        saving={saving}
        journeySettingsOpen={journeySettingsOpen}
        onBack={() => {
          setJourneySettingsOpen(false);
          navigate(`/${encodeURIComponent(route.realm)}/flow`);
        }}
        onToggleSettings={() => setJourneySettingsOpen(true)}
        onDuplicate={() => journey && openDuplicateJourney(journey)}
        onExport={() => journey && exportJourney(journey)}
        onSave={saveCurrentJourney}
        onDelete={() => journey && setDeleteTargetJourney(journey)}
      />
      <JourneySettingsModal
        open={journeySettingsOpen}
        realm={route.realm}
        journey={journey}
        confirmLabel="Apply changes"
        onClose={() => setJourneySettingsOpen(false)}
        onSave={(next) => {
          updateJourney(() => next);
          setJourneySettingsOpen(false);
        }}
      />
      {error && <ErrorMessage message={error} />}
      <ReactFlowProvider>
        <JourneyCanvas realm={route.realm} journey={journey} nodes={flow.nodes} edges={flow.edges} loading={loading} connectDraft={editor.connectDraft} stepEditor={editor.stepEditor} stepSchemas={stepSchemas} scripts={scripts} notesStepID={editor.notesStepID} notes={editor.notesStepID ? normalizeJourneyNotes(journey?.additional_properties?.notes)[editor.notesStepID] || [] : []} onSelectStep={editor.setSelectedStepID} onConnectTarget={editor.connectToTarget} onCancelConnect={() => editor.setConnectDraft(null)} onChangeStepEditor={editor.setStepEditor} onCancelStepEditor={() => editor.setStepEditor(null)} onSaveStepEditor={editor.saveStepEditor} onAutoSaveStepEditor={editor.saveStepEditor} onAddNote={editor.addStepNote} onRemoveNote={editor.removeStepNote} onCloseNotes={() => editor.setNotesStepID('')} parentJourneyName={parentJourneyName} onReturnToParentJourney={() => returnToParentJourney(route, navigate)} />
      </ReactFlowProvider>
      <JourneyCreateCopyModal
        open={Boolean(creationDraft)}
        mode={creationDraft?.mode || 'duplicate'}
        realm={route.realm}
        initialName={creationDraft?.name || ''}
        initialDescription={creationDraft?.description || ''}
        saving={creatingCopy}
        error={error}
        onCancel={() => {
          setCreationDraft(null);
          setError('');
        }}
        onConfirm={confirmCreateCopy}
      />
      <DeleteConfirmModal open={Boolean(deleteTargetJourney)} itemLabel="Journey" itemName={deleteTargetJourney?.name?.trim() || deleteTargetJourney?.id || ''} confirming={saving} onCancel={() => setDeleteTargetJourney(null)} onConfirm={confirmDeleteJourney} />
    </section>
  );
}

function useFlowRoute() {
  const { realm: routeRealm = 'alpha', '*': routeJourneyPath = '' } = useParams();
  const realm = routeRealm.trim() || 'alpha';
  const journeyStack = useMemo(() => routeJourneyPath.split('/').map((part) => part.trim()).filter(Boolean), [routeJourneyPath]);
  return { realm, journeyStack, journeyId: journeyStack.at(-1) || '', parentJourneyId: journeyStack.length > 1 ? journeyStack.at(-2) || '' : '' };
}

function useJourneyUpdater(setJourney: React.Dispatch<React.SetStateAction<JourneyConfiguration | null>>, setDirty: (value: boolean) => void, journeyRef: React.MutableRefObject<JourneyConfiguration | null>, historyRef: React.MutableRefObject<JourneyHistory>) {
  return (updater: (journey: JourneyConfiguration) => JourneyConfiguration) => setJourney((current) => {
    if (!current) return current;
    historyRef.current = { past: [...historyRef.current.past, cloneJourney(current)].slice(-historyLimit), future: [] };
    const next = updater(cloneJourney(current));
    journeyRef.current = next;
    setDirty(true);
    return next;
  });
}

function useStepSchemas(setStepSchemas: (schemas: StepSchema[]) => void, setEndJourneyTypes: (types: Set<string>) => void) {
  useEffect(() => {
    let cancelled = false;
    listStepSchemas().then((response) => { if (!cancelled) { setStepSchemas(response.result); const discovered = extractEndJourneyTypes(response.result); setEndJourneyTypes(discovered.size > 0 ? discovered : defaultEndJourneyTypes); } }).catch(() => { if (!cancelled) setEndJourneyTypes(defaultEndJourneyTypes); });
    return () => { cancelled = true; };
  }, [setEndJourneyTypes, setStepSchemas]);
}

function useFlowScripts(realm: string, setScripts: (scripts: JourneyScript[]) => void) {
  useEffect(() => {
    let cancelled = false;
    listScripts(realm, { limit: 500 })
      .then((response) => { if (!cancelled) setScripts(response.result || []); })
      .catch(() => { if (!cancelled) setScripts([]); });
    return () => { cancelled = true; };
  }, [realm, setScripts]);
}

function useJourneyList(realm: string, setJourneys: (items: JourneyConfiguration[]) => void, setLoading: (value: boolean) => void, setError: (message: string) => void) {
  useEffect(() => {
    let cancelled = false;
    setLoading(true); setError('');
    listJourneys(realm).then((response) => { if (!cancelled) setJourneys(response.result); }).catch((err: Error) => { if (!cancelled) setError(err.message); }).finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [realm, setError, setJourneys, setLoading]);
}

function useSelectedJourney(realm: string, journeyId: string, setJourney: (item: JourneyConfiguration | null) => void, setDirty: (value: boolean) => void, setLoading: (value: boolean) => void, setError: (message: string) => void, journeyRef: React.MutableRefObject<JourneyConfiguration | null>, resetHistory: () => void, editor: ReturnType<typeof useJourneyEditing>) {
  useEffect(() => {
    if (!journeyId) { setJourney(null); return; }
    let cancelled = false;
    setLoading(true); setError('');
    getJourney(realm, journeyId).then((item) => { if (!cancelled) { setJourney(item); journeyRef.current = item; setDirty(false); editor.setConnectDraft(null); editor.setSelectedStepID(''); editor.setNotesStepID(''); resetHistory(); } }).catch((err: Error) => { if (!cancelled) setError(err.message); }).finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [realm, journeyId]);
}

function useJourneyHistoryKeys(journeyRef: React.MutableRefObject<JourneyConfiguration | null>, historyRef: React.MutableRefObject<JourneyHistory>, setJourney: (journey: JourneyConfiguration) => void, setDirty: (value: boolean) => void, setConnectDraft: (draft: null) => void) {
  const restore = useCallback((direction: 'undo' | 'redo') => {
    const current = journeyRef.current;
    const target = direction === 'undo' ? historyRef.current.past.at(-1) : historyRef.current.future[0];
    if (!current || !target) return;
    historyRef.current = direction === 'undo' ? { past: historyRef.current.past.slice(0, -1), future: [cloneJourney(current), ...historyRef.current.future].slice(0, historyLimit) } : { past: [...historyRef.current.past, cloneJourney(current)].slice(-historyLimit), future: historyRef.current.future.slice(1) };
    setConnectDraft(null); setDirty(true); journeyRef.current = cloneJourney(target); setJourney(journeyRef.current);
  }, [historyRef, journeyRef, setConnectDraft, setDirty, setJourney]);
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (!(event.ctrlKey || event.metaKey) || event.altKey || isEditableKeyboardTarget(event.target)) return;
      const key = event.key.toLowerCase();
      if (key === 'z' || key === 'y') { event.preventDefault(); restore(key === 'z' && !event.shiftKey ? 'undo' : 'redo'); }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [restore]);
}

function flowActions(editor: ReturnType<typeof useJourneyEditing>) {
  return { onSetStart: editor.setStartStep, onConfigure: editor.openConfigureStep, onOpenNotes: editor.openStepNotes, onConnect: editor.startConnect, onBreak: editor.breakOutcome, onAddStep: editor.openAddStep, onDeleteBranch: editor.deleteStepBranch, onOpenSubJourney: editor.openSubJourney, connectingOutcome: editor.connectDraft || undefined };
}

function returnToParentJourney(route: ReturnType<typeof useFlowRoute>, navigate: ReturnType<typeof useNavigate>) {
  if (route.journeyStack.length <= 1) return;
  const nextStack = route.journeyStack.slice(0, -1);
  navigate(`/${encodeURIComponent(route.realm)}/flow/${nextStack.map((item) => encodeURIComponent(item)).join('/')}`);
}

function filterJourneys(journeys: JourneyConfiguration[], search: string, filters: JourneyFilters) {
  const query = search.trim().toLowerCase();
  return journeys.filter((item) => {
    if (query && !`${item.name || ''} ${item.description || ''} ${item.id || ''}`.toLowerCase().includes(query)) return false;
    if (filters.journeyType && normalizeJourneyType(item.journey_type) !== filters.journeyType) return false;
    if (!matchesBooleanFilter(item.active, filters.active)) return false;
    if (!matchesBooleanFilter(item.debug, filters.debug)) return false;
    if (!matchesBooleanFilter(item.confidential, filters.confidential)) return false;
    return matchesBooleanFilter(item.encrypted_client_inputs, filters.encryptedInputs);
  });
}

function matchesBooleanFilter(value: boolean | undefined, filter: JourneyFilters['active']) {
  if (filter === 'all') return true;
  return Boolean(value) === (filter === 'yes');
}

function defaultJourneyFilters(): JourneyFilters {
  return { journeyType: '', active: 'all', debug: 'all', confidential: 'all', encryptedInputs: 'all' };
}
