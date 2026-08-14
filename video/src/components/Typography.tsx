import type { ReactNode } from 'react'
import { interpolate, spring, useCurrentFrame, useVideoConfig } from 'remotion'
import { colors } from '../theme'

export function Eyebrow({ children, delay = 0 }: { children: ReactNode; delay?: number }) {
  const frame = useCurrentFrame()
  const { fps } = useVideoConfig()
  const progress = spring({ frame: frame - delay, fps, config: { damping: 18, stiffness: 130 } })
  return (
    <div style={{ color: colors.cyan, fontSize: 24, fontWeight: 750, letterSpacing: 4.5, opacity: progress, transform: `translateY(${(1 - progress) * 16}px)` }}>
      {children}
    </div>
  )
}

export function Headline({ children, delay = 4, width = 1120 }: { children: ReactNode; delay?: number; width?: number }) {
  const frame = useCurrentFrame()
  const { fps } = useVideoConfig()
  const progress = spring({ frame: frame - delay, fps, config: { damping: 17, stiffness: 105, mass: 0.9 } })
  return (
    <div style={{ width, fontSize: 88, lineHeight: 1.04, fontWeight: 760, letterSpacing: -4.2, opacity: progress, transform: `translateY(${(1 - progress) * 30}px)` }}>
      {children}
    </div>
  )
}

export function Callout({ children, delay = 0, accent = colors.orange }: { children: ReactNode; delay?: number; accent?: string }) {
  const frame = useCurrentFrame()
  const opacity = interpolate(frame, [delay, delay + 8], [0, 1], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' })
  const scale = interpolate(frame, [delay, delay + 10], [0.94, 1], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' })
  return (
    <div style={{ display: 'inline-flex', alignItems: 'center', gap: 14, padding: '18px 26px', border: `1px solid ${accent}80`, borderRadius: 999, background: '#161617E8', boxShadow: '0 18px 70px #00000070', fontSize: 34, fontWeight: 700, opacity, transform: `scale(${scale})` }}>
      <span style={{ width: 12, height: 12, borderRadius: '50%', background: accent, boxShadow: `0 0 24px ${accent}` }} />
      {children}
    </div>
  )
}
