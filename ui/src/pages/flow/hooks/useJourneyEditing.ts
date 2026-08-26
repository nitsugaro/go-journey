import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import type { JourneyConfiguration } from '../../../types/journey';
import type { ConnectDraft, StepEditorState } from '../flowTypes';
import { isUUID, newStepID, sanitizeStepConfig } from '../utils/journeyStateUtils';
import { cloneJourney, ensureOutcome, ensureStepConfig, normalizeJourneyNotes, normalizeSubEntries, pruneStepBranch, stepsEqual } from '../utils/journeyRuntimeUtils';

type UseJourneyEditingOptions = {
  realm: string;
  journey: JourneyConfiguration | null;
  journeyStack: string[];
  updateJourney: (updater: (journey: JourneyConfiguration) => JourneyConfiguration) => void;
  setError: (message: string) => void;
};

export function useJourneyEditing({ realm, journey, journeyStack, updateJourney, setError }: UseJourneyEditingOptions) {
  const navigate = useNavigate();
  const [connectDraft, setConnectDraft] = useState<ConnectDraft | null>(null);
  const [selectedStepID, setSelectedStepID] = useState('');
  const [notesStepID, setNotesStepID] = useState('');
  const [stepEditor, setStepEditor] = useState<StepEditorState | null>(null);

  const setStartStep = (stepID: string) => {
    setConnectDraft(null);
    setSelectedStepID(stepID);
    updateJourney((draft) => normalizeSubEntries({ ...draft, start_step_id: stepID }));
  };

  const openAddStep = (sourceStepID: string, sourceOutcome: string) => {
    setSelectedStepID(sourceStepID);
    setStepEditor({ mode: 'add', sourceStepID, stepID: '', stepName: '', stepType: '', connectOutcome: sourceOutcome, configText: '{}' });
  };

  const openConfigureStep = (stepID: string) => {
    const step = journey?.steps?.[stepID];
    if (!step) return;
    setSelectedStepID(stepID);
    setNotesStepID('');
    setStepEditor({ mode: 'edit', stepID, stepName: step.name || step.step_type || 'Step', stepType: step.step_type, configText: JSON.stringify(step.config || {}, null, 2) });
  };

  const openStepNotes = (stepID: string) => {
    if (!journey?.steps?.[stepID]) return;
    setSelectedStepID(stepID);
    setStepEditor(null);
    setNotesStepID(stepID);
  };

  const addStepNote = (stepID: string, note: string) => {
    const cleanNote = note.trim();
    if (!cleanNote) return;
    updateJourney((draft) => {
      const additional = { ...(draft.additional_properties || {}) };
      const notes = normalizeJourneyNotes(additional.notes);
      notes[stepID] = [...(notes[stepID] || []), { note: cleanNote, timestamp: Date.now() }];
      additional.notes = notes;
      draft.additional_properties = additional;
      return draft;
    });
  };

  const removeStepNote = (stepID: string, index: number) => {
    updateJourney((draft) => {
      const additional = { ...(draft.additional_properties || {}) };
      const notes = normalizeJourneyNotes(additional.notes);
      const nextNotes = (notes[stepID] || []).filter((_, noteIndex) => noteIndex !== index);
      if (nextNotes.length > 0) notes[stepID] = nextNotes;
      else delete notes[stepID];
      if (Object.keys(notes).length > 0) additional.notes = notes;
      else delete additional.notes;
      draft.additional_properties = Object.keys(additional).length > 0 ? additional : null;
      return draft;
    });
  };

  const openSubJourney = (subJourneyId: string) => {
    if (!subJourneyId) return;
    const nextStack = [...journeyStack, subJourneyId];
    navigate(`/${encodeURIComponent(realm)}/flow/${nextStack.map((item) => encodeURIComponent(item)).join('/')}`);
  };

  const connectToTarget = (targetID: string) => {
    if (!connectDraft?.source || !connectDraft.outcome || !targetID) return;
    if (!isUUID(targetID)) return setError('Outcome targets must be UUID step IDs. Create a new step from the UI or select a UUID-backed step.');
    setError('');
    updateJourney((draft) => {
      const step = draft.steps[connectDraft.source];
      if (!step) return draft;
      ensureOutcome(ensureStepConfig(step))[connectDraft.outcome] = targetID;
      return normalizeSubEntries(draft);
    });
    setConnectDraft(null);
  };

  const breakOutcome = (stepID: string, outcomeKey: string) => {
    updateJourney((draft) => {
      const outcome = draft.steps[stepID]?.config?.outcome;
      if (!outcome || typeof outcome !== 'object' || Array.isArray(outcome)) return draft;
      const target = (outcome as Record<string, unknown>)[outcomeKey];
      delete (outcome as Record<string, unknown>)[outcomeKey];
      return typeof target === 'string' && draft.steps[target] ? normalizeSubEntries(draft) : draft;
    });
  };

  const deleteStepBranch = (stepID: string) => {
    if (!journey?.steps?.[stepID]) return;
    if (stepID === journey.start_step_id) return setError('Start step cannot be deleted. Set another start step first.');
    setError('');
    setConnectDraft(null);
    if (stepEditor?.stepID === stepID) setStepEditor(null);
    if (notesStepID === stepID) setNotesStepID('');
    updateJourney((draft) => pruneStepBranch(draft, stepID));
  };

  const saveStepEditor = (editorOverride?: StepEditorState | null) => {
    const editorToSave = editorOverride === undefined ? stepEditor : editorOverride;
    if (!editorToSave) return;
    const config = parseStepConfig(editorToSave, setError);
    if (!config) return;
    const stepID = editorToSave.stepID.trim() || newStepID(journey?.steps || {});
    const nextStep = { name: editorToSave.stepName.trim() || editorToSave.stepType || 'Step', step_type: editorToSave.stepType, config };
    if (editorToSave.mode === 'edit' && journey?.steps?.[stepID] && stepsEqual(journey.steps[stepID], nextStep)) return setStepEditor(null);

    updateJourney((draft) => {
      draft.steps ||= {};
      draft.steps[stepID] = cloneJourney(nextStep as never) as never;
      if (editorToSave.mode === 'add' && editorToSave.sourceStepID && editorToSave.connectOutcome?.trim()) {
        const source = draft.steps[editorToSave.sourceStepID];
        if (source) ensureOutcome(ensureStepConfig(source))[editorToSave.connectOutcome.trim()] = stepID;
      }
      return normalizeSubEntries(draft);
    });
    setStepEditor(null);
  };

  return { connectDraft, selectedStepID, notesStepID, stepEditor, setConnectDraft, setSelectedStepID, setNotesStepID, setStepEditor, setStartStep, startConnect: (source: string, outcome: string) => setConnectDraft({ source, outcome }), openAddStep, openConfigureStep, openStepNotes, addStepNote, removeStepNote, openSubJourney, connectToTarget, breakOutcome, deleteStepBranch, saveStepEditor };
}

function parseStepConfig(editor: StepEditorState, setError: (message: string) => void) {
  if (!editor.stepType) {
    if (editor.mode === 'add') return null;
    setError('Step type is required.');
    return null;
  }
  try {
    const parsed = JSON.parse(editor.configText || '{}');
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error('Step config must be a JSON object.');
    return sanitizeStepConfig(parsed as Record<string, unknown>);
  } catch (err) {
    setError(err instanceof Error ? err.message : 'Invalid step config JSON.');
    return null;
  }
}
