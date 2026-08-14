import { AppViewport } from '../components/AppViewport'
import { SceneLayer } from '../components/SceneLayer'
import { Callout } from '../components/Typography'
import { copyString, getScene, sceneDuration } from '../storyboard'

const scene = getScene('dashboard')

export function DashboardScene() {
  return (
    <SceneLayer durationInFrames={sceneDuration(scene)}>
      <AppViewport asset={scene.assets?.[0] ?? ''} zoom={[1.01, 1.09]} panX={[0, -22]} panY={[0, 18]} />
      <div style={{ position: 'absolute', left: 112, bottom: 70 }}>
        <Callout delay={20}>{copyString(scene, 'callout')}</Callout>
      </div>
    </SceneLayer>
  )
}
