import { Composition } from 'remotion'
import { SkillManagerOverview } from './SkillManagerOverview'
import { VIDEO } from './storyboard'

export function Root() {
  return (
    <Composition
      id="SkillManagerOverview"
      component={SkillManagerOverview}
      durationInFrames={VIDEO.durationInFrames}
      fps={VIDEO.fps}
      width={VIDEO.width}
      height={VIDEO.height}
    />
  )
}
