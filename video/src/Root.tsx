import { Composition, Still } from 'remotion'
import { SkillManagerOverview } from './SkillManagerOverview'
import { SocialPreview } from './SocialPreview'
import { VIDEO } from './storyboard'

export function Root() {
  return (
    <>
      <Composition
        id="SkillManagerOverview"
        component={SkillManagerOverview}
        durationInFrames={VIDEO.durationInFrames}
        fps={VIDEO.fps}
        width={VIDEO.width}
        height={VIDEO.height}
      />
      <Still id="SkillManagerSocialPreview" component={SocialPreview} width={1280} height={640} />
    </>
  )
}
