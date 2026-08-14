import type { CSSProperties, ReactNode } from 'react'
import { AbsoluteFill, Easing, interpolate, spring, useCurrentFrame, useVideoConfig } from 'remotion'
import { colors, fonts } from '../theme'

type SceneLayerProps = {
  children: ReactNode
  durationInFrames: number
  exitFrames?: number
  style?: CSSProperties
}

export function SceneLayer({ children, durationInFrames, exitFrames = 10, style }: SceneLayerProps) {
  const frame = useCurrentFrame()
  const { fps } = useVideoConfig()
  const entrance = spring({ frame, fps, config: { damping: 20, stiffness: 115, mass: 0.85 } })
  const exit = interpolate(
    frame,
    [durationInFrames - exitFrames, durationInFrames],
    [1, 0],
    { extrapolateLeft: 'clamp', extrapolateRight: 'clamp', easing: Easing.inOut(Easing.ease) },
  )

  return (
    <AbsoluteFill
      style={{
        backgroundColor: colors.canvas,
        color: colors.text,
        fontFamily: fonts.sans,
        opacity: Math.min(entrance, exit),
        overflow: 'hidden',
        ...style,
      }}
    >
      <AbsoluteFill
        style={{
          backgroundImage: `radial-gradient(circle at 74% 24%, ${colors.blue}16 0, transparent 38%), radial-gradient(circle at 18% 78%, ${colors.cyan}10 0, transparent 34%)`,
        }}
      />
      {children}
    </AbsoluteFill>
  )
}
