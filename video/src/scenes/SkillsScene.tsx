import { interpolate, useCurrentFrame } from 'remotion'
import { AppViewport } from '../components/AppViewport'
import { Pointer } from '../components/Pointer'
import { SceneLayer } from '../components/SceneLayer'
import { Callout } from '../components/Typography'
import { colors } from '../theme'
import { copyString, getScene, sceneDuration } from '../storyboard'

const scene = getScene('skills')

export function SkillsScene() {
  const frame = useCurrentFrame()
  const beforeOpacity = interpolate(frame, [44, 50], [1, 0], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' })
  const pendingOpacity = Math.min(
    interpolate(frame, [44, 50], [0, 1], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' }),
    interpolate(frame, [105, 111], [1, 0], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' }),
  )
  const afterOpacity = interpolate(frame, [105, 111], [0, 1], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' })

  return (
    <SceneLayer durationInFrames={sceneDuration(scene)}>
      <AppViewport asset={scene.assets?.[0] ?? ''} zoom={[1.035, 1.06]} panX={[0, -16]} panY={[-30, -135]} imageOpacity={beforeOpacity} />
      <AppViewport asset={scene.assets?.[1] ?? ''} zoom={[1.035, 1.06]} panX={[0, -16]} panY={[-30, -135]} imageOpacity={pendingOpacity} />
      <AppViewport asset={scene.assets?.[2] ?? ''} zoom={[1.035, 1.06]} panX={[0, -16]} panY={[-30, -135]} imageOpacity={afterOpacity} />

      {frame < 70 && (
        <Pointer from={[1740, 820]} to={[1486, 491]} moveStart={10} moveEnd={36} clickFrame={43} label={copyString(scene, 'pointerToggle')} />
      )}
      {frame >= 55 && frame < 125 && (
        <Pointer from={[1510, 510]} to={[1645, 930]} moveStart={60} moveEnd={87} clickFrame={101} label={copyString(scene, 'pointerApply')} />
      )}

      <div style={{ position: 'absolute', left: 108, bottom: 68 }}>
        {frame < 62 && <Callout delay={12} accent={colors.cyan}>{copyString(scene, 'calloutStage')}</Callout>}
        {frame >= 55 && frame < 122 && <Callout delay={55} accent={colors.orange}>{copyString(scene, 'calloutApply')}</Callout>}
        {frame >= 112 && <Callout delay={112} accent={colors.green}>{copyString(scene, 'calloutRestore')}</Callout>}
      </div>
    </SceneLayer>
  )
}
