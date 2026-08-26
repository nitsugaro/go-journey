import { Handle, Position, type NodeProps } from '@xyflow/react'
import { FileText } from 'lucide-react'
import type { CSSProperties } from 'react'
import { IconButton } from '../../components/Button'
import { formatStepTypeLabel } from './utils/displayUtils'

export type JourneyStepNodeData = {
  id: string
  name: string
  stepType: string
  color?: {
    stroke: string
    soft: string
    text: string
  }
  reference?: boolean
  originalId?: string
  start?: boolean
  subEntry?: boolean
  terminal?: boolean
  endJourney?: boolean
  needsOutcomeConfiguration?: boolean
  connectingOutcome?: { source: string; outcome: string }
  outcomes: Array<{ id: string; name: string; target: string }>
  noteCount?: number
  onSetStart?: (stepID: string) => void
  onConfigure?: (stepID: string) => void
  onOpenNotes?: (stepID: string) => void
  onConnect?: (stepID: string, outcome: string) => void
  onBreak?: (stepID: string, outcome: string) => void
  onAddStep?: (stepID: string, outcome: string) => void
  onDeleteBranch?: (stepID: string) => void
  onOpenSubJourney?: (journeyID: string) => void
  subJourneyId?: string
  subJourneyName?: string
}

export function JourneyStepNode({ data }: NodeProps) {
  const step = data as JourneyStepNodeData
  const endKind = step.stepType.toLowerCase()
  const successEnd = step.endJourney && endKind === 'success'
  const failureEnd = step.endJourney && endKind === 'failure'
  const nodeColor = step.color || { stroke: 'var(--color-blue)', soft: 'var(--color-blue-soft)', text: 'var(--color-blue)' }
  const accent = successEnd
    ? 'var(--color-green)'
    : failureEnd
      ? 'var(--color-red)'
      : step.terminal
        ? 'var(--color-green)'
    : step.reference
      ? 'var(--color-muted)'
    : step.subEntry
      ? 'var(--color-red)'
      : step.start
        ? nodeColor.stroke
        : 'var(--color-ink)'
  const startStyle = !step.reference && step.start
    ? {
        borderColor: nodeColor.stroke,
        boxShadow: `0 12px 28px color-mix(in srgb, ${nodeColor.stroke} 18%, transparent)`,
      } as CSSProperties
    : undefined

  return (
    <div
      className={[
        'text-left shadow-sm',
        step.reference
          ? 'w-72 rounded-2xl border border-dashed border-[var(--color-muted-faint)] bg-[var(--color-surface-overlay)] p-3.5 text-[var(--color-muted)]'
          : successEnd
            ? 'w-80 rounded-3xl border border-[var(--color-green-border)] bg-gradient-to-br from-[var(--color-surface)] to-[var(--color-green-soft)] p-4'
            : failureEnd
              ? 'w-80 rounded-3xl border border-[var(--color-red-border)] bg-gradient-to-br from-[var(--color-surface)] to-[var(--color-red-soft)] p-4'
              : step.start
                ? 'w-80 rounded-3xl border-2 bg-[var(--color-surface)] p-4'
                : 'w-80 rounded-3xl border border-[var(--color-border)] bg-[var(--color-surface)] p-4',
      ].join(' ')}
      style={startStyle}
    >
      <Handle
        type="target"
        position={Position.Left}
        className={[
          '!h-3 !w-3 !border-2 !border-[var(--color-surface)]',
        ].join(' ')}
        style={{ backgroundColor: step.reference ? 'var(--color-muted-soft)' : nodeColor.stroke }}
      />
      <div className={['relative min-h-12', step.reference ? 'pr-10' : 'pr-40'].join(' ')}>
        <div className="min-w-0">
          <p className="flex items-center gap-1.5 text-xs font-semibold uppercase tracking-[0.16em]" style={{ color: accent }}>
            {step.reference && <span aria-hidden="true">↗</span>}
            {successEnd && <span aria-hidden="true">✓</span>}
            {failureEnd && <span aria-hidden="true">!</span>}
            {formatStepTypeLabel(step.stepType)}
          </p>
          <h3
            className={['mt-1 font-semibold', step.reference ? 'text-base leading-snug text-[var(--color-ink)]' : 'truncate text-base text-[var(--color-ink)]'].join(' ')}
            style={step.reference ? {
              display: '-webkit-box',
              WebkitLineClamp: 2,
              WebkitBoxOrient: 'vertical',
              overflow: 'hidden',
            } : undefined}
            title={step.name}
          >
            {step.name}
          </h3>
        </div>
        {step.reference ? (
          <span className="absolute right-0 top-0 rounded-full bg-[var(--color-surface)] px-2 py-1 text-[10px] font-semibold text-[var(--color-muted)]">⌖</span>
        ) : (
          <div className="absolute right-0 top-0 flex items-center gap-1">
            <IconButton
              onClick={(event) => {
                event.stopPropagation()
                step.onSetStart?.(step.id)
              }}
              variant={step.start ? 'primary' : 'secondary'}
              size="xs"
              style={step.start ? { backgroundColor: nodeColor.stroke } : undefined}
              label={step.start ? 'Start step' : 'Set as start'}
            >
              ◎
            </IconButton>
            <IconButton
              onClick={(event) => {
                event.stopPropagation()
                step.onConfigure?.(step.id)
              }}
              label="Configure step"
              variant="secondary"
              size="xs"
            >
              ✎
            </IconButton>
            <IconButton
              onClick={(event) => {
                event.stopPropagation()
                step.onOpenNotes?.(step.id)
              }}
              label={step.noteCount ? `${step.noteCount} notes` : 'Add note'}
              variant={step.noteCount ? 'warning' : 'secondary'}
              size="xs"
              className="relative"
            >
              <NoteIcon />
              {Boolean(step.noteCount) && (
                <span className="absolute -right-1 -top-1 grid h-4 min-w-4 place-items-center rounded-full bg-[var(--color-warning)] px-1 text-[9px] font-bold text-[var(--color-white)]">
                  {step.noteCount}
                </span>
              )}
            </IconButton>
            {!step.start && (
              <IconButton
                onClick={(event) => {
                  event.stopPropagation()
                  step.onDeleteBranch?.(step.id)
                }}
                label="Delete this step and private child branch"
                variant="secondary"
                size="xs"
              >
                ⌫
              </IconButton>
            )}
            {step.subJourneyId && (
              <IconButton
                onClick={(event) => {
                  event.stopPropagation()
                  step.onOpenSubJourney?.(step.subJourneyId || '')
                }}
                label={`Open ${step.subJourneyName || 'sub-journey'}`}
                variant="secondary"
                size="xs"
              >
                ↗
              </IconButton>
            )}
          </div>
        )}
      </div>
      <div className="mt-4 space-y-2">
        {step.reference ? (
          <div className="rounded-xl bg-[var(--color-surface)] px-3 py-2 text-xs font-medium text-[var(--color-muted)]">
            jump to original
          </div>
        ) : step.endJourney ? (
          <div
            className={[
              'rounded-2xl px-3 py-2 text-xs font-semibold',
              failureEnd
                ? 'bg-[var(--color-red-soft)] text-[var(--color-red)]'
                : 'bg-[var(--color-green-soft)] text-[var(--color-green)]',
            ].join(' ')}
          >
            {successEnd ? 'journey success' : failureEnd ? 'journey failure' : 'end journey'}
          </div>
        ) : step.outcomes.length === 0 && step.needsOutcomeConfiguration ? (
          <div className="rounded-2xl bg-[var(--color-warning-soft)] px-3 py-2 text-xs font-semibold text-[var(--color-warning)] ring-1 ring-[var(--color-warning-border)]">
            outcomes not configured
          </div>
        ) : step.outcomes.length === 0 ? (
          <div className="rounded-2xl bg-[var(--color-green-soft)] px-3 py-2 text-xs font-medium text-[var(--color-green)]">
            terminal
          </div>
        ) : (
          step.outcomes.map((outcome) => {
            const selected = step.connectingOutcome?.source === step.id && step.connectingOutcome.outcome === outcome.name
            const selectedConnected = selected && outcome.target
            return (
            <div
              key={outcome.id}
              onClick={(event) => {
                event.stopPropagation()
                step.onConnect?.(step.id, outcome.name)
              }}
              className={[
                'group relative cursor-crosshair rounded-2xl py-2 pl-3 pr-20 text-xs transition',
                selected
                  ? 'ring-2'
                  : 'hover:brightness-[0.98]',
              ].join(' ')}
              style={{
                backgroundColor: selectedConnected ? nodeColor.stroke : nodeColor.soft,
                color: selectedConnected ? 'var(--color-white)' : nodeColor.text,
                ...(selected ? { '--tw-ring-color': nodeColor.stroke } as CSSProperties : {}),
              }}
              title={`Connect or reconnect ${outcome.name}`}
            >
              <span className="font-semibold">{outcome.name}</span>
              <IconButton
                onClick={(event) => {
                  event.stopPropagation()
                  step.onAddStep?.(step.id, outcome.name)
                }}
                className="absolute right-10 top-1/2 -translate-y-1/2"
                label={`Add step from ${outcome.name}`}
                variant="secondary"
                size="xs"
              >
                +
              </IconButton>
              <IconButton
                onClick={(event) => {
                  event.stopPropagation()
                  step.onBreak?.(step.id, outcome.name)
                }}
                className="absolute right-3 top-1/2 -translate-y-1/2"
                label={`Break connection from ${outcome.name}`}
                variant="ghost"
                size="xs"
              >
                ⊘
              </IconButton>
              <Handle
                id={outcome.id}
                type="source"
                position={Position.Right}
                className="!right-[-22px] !h-3 !w-3 !border-2 !border-[var(--color-surface)]"
                style={{ backgroundColor: nodeColor.stroke }}
              />
            </div>
            )
          })
        )}
      </div>
    </div>
  )
}

function NoteIcon() {
  return <FileText size={14} strokeWidth={2} aria-hidden="true" />
}
