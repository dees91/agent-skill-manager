import { AppViewport } from '../components/AppViewport'
import { SceneLayer } from '../components/SceneLayer'
import { Callout } from '../components/Typography'
import { colors } from '../theme'
import { copyString, getScene, sceneDuration } from '../storyboard'

const scene = getScene('sources')

export function SourcesScene() {
  return (
    <SceneLayer durationInFrames={sceneDuration(scene)}>
      <AppViewport asset={scene.assets?.[0] ?? ''} zoom={[1.01, 1.075]} panX={[0, -20]} panY={[-8, 12]} />
      <div style={{ position: 'absolute', left: 108, bottom: 68 }}>
        <Callout delay={18} accent={colors.blue}>{copyString(scene, 'callout')}</Callout>
      </div>
    </SceneLayer>
  )
}
