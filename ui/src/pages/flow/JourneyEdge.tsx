import { BaseEdge, type EdgeProps } from '@xyflow/react'

type JourneyEdgeData = {
  sourceY?: number
  targetY?: number
  laneOffset?: number
  reference?: boolean
}

export function JourneyEdge(props: EdgeProps) {
  const { sourceX, sourceY, targetX, targetY, markerEnd, style, data } = props
  const edgeData = (data || {}) as JourneyEdgeData
  if (edgeData.reference) {
    const distance = Math.max(90, Math.abs(targetX - sourceX))
    const controlDistance = Math.min(120, distance * 0.38)
    const path = `M ${sourceX} ${sourceY} C ${sourceX + controlDistance} ${sourceY}, ${
      targetX - controlDistance
    } ${targetY}, ${targetX} ${targetY}`
    return <BaseEdge path={path} markerEnd={markerEnd} style={style} />
  }

  const offset = clamp(edgeData.laneOffset || 0, -22, 22)
  const distance = Math.max(120, Math.abs(targetX - sourceX))
  const controlDistance = Math.min(220, distance * 0.42)

  const path = `M ${sourceX} ${sourceY} C ${sourceX + controlDistance} ${sourceY + offset}, ${
    targetX - controlDistance
  } ${targetY - offset}, ${targetX} ${targetY}`

  return <BaseEdge path={path} markerEnd={markerEnd} style={style} />
}

function clamp(value: number, min: number, max: number) {
  return Math.max(min, Math.min(max, value))
}
