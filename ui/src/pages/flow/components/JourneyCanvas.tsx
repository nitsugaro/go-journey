import { useEffect, useRef, useState } from 'react';
import { Background, Controls, ReactFlow, useReactFlow, type Edge, type Node, type Viewport } from '@xyflow/react';
import type { JourneyConfiguration, JourneyScript, StepSchema } from '../../../types/journey';
import { Button } from '../../../components/Button';
import type { ConnectDraft, JourneyNote, StepEditorState } from '../flowTypes';
import { JourneyEdge } from '../JourneyEdge';
import { JourneyStepNode } from '../JourneyStepNode';
import { stepDisplayName } from '../utils/journeyStateUtils';
import { StepEditorPanel } from './StepEditorPanel';
import { StepNotesPanel } from './StepNotesPanel';

const nodeTypes = { journeyStep: JourneyStepNode };
const edgeTypes = { journeyEdge: JourneyEdge };

export function JourneyCanvas({
  realm,
  journey,
  nodes,
  edges,
  loading,
  parentJourneyName,
  connectDraft,
  stepEditor,
  stepSchemas,
  scripts,
  notesStepID,
  notes,
  onConnectTarget,
  onCancelConnect,
  onSelectStep,
  onChangeStepEditor,
  onCancelStepEditor,
  onSaveStepEditor,
  onAutoSaveStepEditor,
  onAddNote,
  onRemoveNote,
  onCloseNotes,
  onReturnToParentJourney,
}: {
  realm: string;
  journey: JourneyConfiguration | null;
  nodes: Node[];
  edges: Edge[];
  loading: boolean;
  parentJourneyName?: string;
  connectDraft: ConnectDraft | null;
  stepEditor: StepEditorState | null;
  stepSchemas: StepSchema[];
  scripts: JourneyScript[];
  notesStepID: string;
  notes: JourneyNote[];
  onConnectTarget: (targetID: string) => void;
  onCancelConnect: () => void;
  onSelectStep: (stepID: string) => void;
  onChangeStepEditor: (editor: StepEditorState) => void;
  onCancelStepEditor: () => void;
  onSaveStepEditor: (editor?: StepEditorState) => void;
  onAutoSaveStepEditor: (editor: StepEditorState) => void;
  onAddNote: (stepID: string, note: string) => void;
  onRemoveNote: (stepID: string, index: number) => void;
  onCloseNotes: () => void;
  onReturnToParentJourney?: () => void;
}) {
  const flow = useReactFlow();
  const [returnViewport, setReturnViewport] = useState<Viewport | null>(null);
  const [returnFading, setReturnFading] = useState(false);
  const dismissTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const fadeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const programmaticMove = useRef(false);

  const clearReturnTimers = () => {
    if (dismissTimer.current) clearTimeout(dismissTimer.current);
    if (fadeTimer.current) clearTimeout(fadeTimer.current);
    dismissTimer.current = null;
    fadeTimer.current = null;
  };

  useEffect(() => {
    clearReturnTimers();
    setReturnViewport(null);
    setReturnFading(false);
    return clearReturnTimers;
  }, [journey?.id]);

  const jumpToOriginal = (originalID: string) => {
    const node = flow.getNode(originalID);
    if (!node) return;
    clearReturnTimers();
    setReturnFading(false);
    setReturnViewport(flow.getViewport());
    programmaticMove.current = true;
    void flow.setCenter(node.position.x + 160, node.position.y + 80, { zoom: 0.9, duration: 450 }).finally(() => {
      programmaticMove.current = false;
    });
  };

  const scheduleReturnDismissal = () => {
    if (!returnViewport || programmaticMove.current || dismissTimer.current) return;
    dismissTimer.current = setTimeout(() => {
      setReturnFading(true);
      fadeTimer.current = setTimeout(() => {
        setReturnViewport(null);
        setReturnFading(false);
      }, 300);
    }, 5000);
  };

  const returnToPreviousViewport = () => {
    if (!returnViewport) return;
    const viewport = returnViewport;
    clearReturnTimers();
    setReturnViewport(null);
    setReturnFading(false);
    programmaticMove.current = true;
    void flow.setViewport(viewport, { duration: 450 }).finally(() => {
      programmaticMove.current = false;
    });
  };

  return (
    <div className="relative min-h-0 flex-1 overflow-hidden rounded-3xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-sm">
      {connectDraft && journey && (
        <div className="absolute left-5 top-5 z-10 flex items-center gap-3 rounded-2xl border border-[var(--color-blue)] bg-[var(--color-surface-overlay)] px-4 py-2 text-sm shadow-sm">
          <span className="font-semibold text-[var(--color-blue)]">↗ reconnect</span>
          <span className="text-[var(--color-muted)]">
            {stepDisplayName(journey, connectDraft.source)}
            <span className="mx-1 text-[var(--color-muted-soft)]">/</span>
            <span className="font-semibold text-[var(--color-ink)]">{connectDraft.outcome}</span>
          </span>
          <span className="hidden text-[var(--color-muted-soft)] sm:inline">click target step</span>
          <Button onClick={onCancelConnect} variant="secondary" size="xs">
            cancel
          </Button>
        </div>
      )}
      <ReactFlow
        key={journey?.id || 'empty-flow'}
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        fitView
        fitViewOptions={{ padding: 0.28, includeHiddenNodes: false }}
        minZoom={0.25}
        maxZoom={1.4}
        nodesDraggable={false}
        panOnDrag
        panOnScroll
        selectionOnDrag={false}
        defaultEdgeOptions={{ interactionWidth: 28 }}
        proOptions={{ hideAttribution: true }}
        onMoveStart={(event) => {
          if (event) scheduleReturnDismissal();
        }}
        onNodeClick={(_, node) => {
          const originalID = (node.data as { originalId?: string }).originalId;
          if (connectDraft) {
            onConnectTarget(originalID || node.id);
            return;
          }
          onSelectStep(originalID || node.id);
          if (originalID) {
            jumpToOriginal(originalID);
            return;
          }
        }}
        onPaneClick={() => {
          if (stepEditor) {
            if (stepEditor.mode === 'edit') onAutoSaveStepEditor(stepEditor);
            else onCancelStepEditor();
          }
          if (notesStepID) onCloseNotes();
        }}
      >
        <Background color="var(--color-border)" gap={28} />
        <Controls />
      </ReactFlow>
      {stepEditor && (
        <div
          className="motion-panel absolute bottom-4 right-4 top-4 z-20 w-[min(560px,calc(100%-2rem))]"
          onPointerDown={(event) => event.stopPropagation()}
          onClick={(event) => event.stopPropagation()}
        >
          <StepEditorPanel
            key={`${stepEditor.mode}-${stepEditor.stepID}`}
            realm={realm}
            editor={stepEditor}
            schemas={stepSchemas}
            scripts={scripts}
            journey={journey}
            onChange={onChangeStepEditor}
            onCancel={onCancelStepEditor}
            onSave={onSaveStepEditor}
          />
        </div>
      )}
      {notesStepID && journey?.steps?.[notesStepID] && (
        <div
          className="motion-panel absolute bottom-4 right-4 top-4 z-20 w-[min(440px,calc(100%-2rem))]"
          onPointerDown={(event) => event.stopPropagation()}
          onClick={(event) => event.stopPropagation()}
        >
          <StepNotesPanel
            stepName={stepDisplayName(journey, notesStepID)}
            notes={notes}
            onAdd={(note) => onAddNote(notesStepID, note)}
            onRemove={(index) => onRemoveNote(notesStepID, index)}
            onClose={onCloseNotes}
          />
        </div>
      )}
      {(returnViewport || parentJourneyName) && (
        <div className="absolute bottom-5 left-1/2 z-20 flex -translate-x-1/2 flex-col items-center gap-2">
          {returnViewport && (
            <Button
              onClick={returnToPreviousViewport}
              variant="secondary"
              size="lg"
              className={['shadow-lg shadow-[var(--color-border-subtle)] backdrop-blur transition-opacity duration-300', returnFading ? 'opacity-0' : 'opacity-100'].join(' ')}
              title="Return to the position and zoom before following the reference"
            >
              ← Back to previous view
            </Button>
          )}
          {parentJourneyName && (
            <Button
              onClick={onReturnToParentJourney}
              variant="secondary"
              size="lg"
              className="shadow-lg shadow-[var(--color-border-subtle)] backdrop-blur"
              title={`Return to ${parentJourneyName}`}
            >
              ← Back to {parentJourneyName}
            </Button>
          )}
        </div>
      )}
      {!journey && (
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center bg-[var(--color-surface-muted-transparent)]">
          <div className="rounded-3xl border border-[var(--color-border)] bg-[var(--color-surface)] px-6 py-5 text-center shadow-sm">
            <p className="font-semibold">{loading ? 'Loading journey…' : 'No journey selected'}</p>
            <p className="mt-1 text-sm text-[var(--color-muted)]">Select a realm and journey to render the flow.</p>
          </div>
        </div>
      )}
    </div>
  );
}
