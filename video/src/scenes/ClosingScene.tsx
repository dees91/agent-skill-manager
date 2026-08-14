import { Img, interpolate, spring, staticFile, useCurrentFrame, useVideoConfig } from 'remotion'
import { SceneLayer } from '../components/SceneLayer'
import { colors } from '../theme'
import { copyString, getScene, sceneDuration } from '../storyboard'

const scene = getScene('closing')

export function ClosingScene() {
  const frame = useCurrentFrame()
  const { fps } = useVideoConfig()
  const icon = spring({ frame: frame - 3, fps, config: { damping: 16, stiffness: 105, mass: 0.9 } })
  const copy = spring({ frame: frame - 10, fps, config: { damping: 18, stiffness: 95 } })
  const finalFade = interpolate(frame, [66, 82], [1, 0], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' })

  return (
    <SceneLayer durationInFrames={sceneDuration(scene)} exitFrames={1} style={{ opacity: finalFade }}>
      <div style={{ position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 72, padding: '0 130px' }}>
        <Img src={staticFile(scene.assets?.[0] ?? '')} style={{ width: 270, height: 270, opacity: icon, transform: `scale(${0.82 + icon * 0.18}) rotate(${-4 * (1 - icon)}deg)`, filter: `drop-shadow(0 40px 80px ${colors.cyan}18)` }} />
        <div style={{ width: 1050, opacity: copy, transform: `translateY(${(1 - copy) * 28}px)` }}>
          <div style={{ color: colors.cyan, fontSize: 30, fontWeight: 760, letterSpacing: 4 }}>{copyString(scene, 'product').toUpperCase()}</div>
          <div style={{ marginTop: 20, fontSize: 78, lineHeight: 1.06, fontWeight: 760, letterSpacing: -3.5 }}>{copyString(scene, 'headline')}</div>
          <div style={{ marginTop: 34, color: colors.muted, fontSize: 31, fontWeight: 580 }}>{copyString(scene, 'footer')}</div>
        </div>
      </div>
    </SceneLayer>
  )
}
