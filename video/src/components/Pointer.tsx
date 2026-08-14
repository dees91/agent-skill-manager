import { interpolate, spring, useCurrentFrame, useVideoConfig } from 'remotion'
import { colors } from '../theme'

type PointerProps = {
  from: readonly [number, number]
  to: readonly [number, number]
  moveStart: number
  moveEnd: number
  clickFrame: number
  label: string
}

export function Pointer({ from, to, moveStart, moveEnd, clickFrame, label }: PointerProps) {
  const frame = useCurrentFrame()
  const { fps } = useVideoConfig()
  const movement = spring({ frame: frame - moveStart, fps, durationInFrames: Math.max(1, moveEnd - moveStart), config: { damping: 24, stiffness: 90 } })
  const x = interpolate(movement, [0, 1], [from[0], to[0]])
  const y = interpolate(movement, [0, 1], [from[1], to[1]])
  const clickDistance = Math.abs(frame - clickFrame)
  const ring = interpolate(clickDistance, [0, 10], [1, 0], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' })
  const visible = interpolate(frame, [moveStart - 4, moveStart + 2], [0, 1], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' })

  return (
    <div aria-label={label} style={{ position: 'absolute', left: x, top: y, width: 40, height: 50, opacity: visible, filter: 'drop-shadow(0 8px 10px #000A)' }}>
      <div style={{ position: 'absolute', left: -25, top: -25, width: 62, height: 62, borderRadius: '50%', border: `4px solid ${colors.orange}`, opacity: ring, transform: `scale(${1.1 + (1 - ring) * 0.7})` }} />
      <svg viewBox="0 0 32 42" width="40" height="52" aria-hidden="true">
        <path d="M2 2v31l8.1-7.3 5.9 13.1 6.2-2.8-5.8-12.9H29L2 2Z" fill="#F2F2F2" stroke="#0D0D0E" strokeWidth="2.5" strokeLinejoin="round" />
      </svg>
    </div>
  )
}
