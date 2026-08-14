import { AbsoluteFill, Sequence } from 'remotion'
import { colors } from './theme'
import { STORYBOARD, sceneDuration } from './storyboard'
import { ProblemScene } from './scenes/ProblemScene'
import { TerminalScene } from './scenes/TerminalScene'
import { DashboardScene } from './scenes/DashboardScene'
import { SkillsScene } from './scenes/SkillsScene'
import { SourcesScene } from './scenes/SourcesScene'
import { ClosingScene } from './scenes/ClosingScene'

const components = {
  problem: ProblemScene,
  terminal: TerminalScene,
  dashboard: DashboardScene,
  skills: SkillsScene,
  sources: SourcesScene,
  closing: ClosingScene,
} as const

export function SkillManagerOverview() {
  return (
    <AbsoluteFill style={{ backgroundColor: colors.canvas }}>
      {STORYBOARD.map((scene) => {
        const Component = components[scene.id]
        return (
          <Sequence key={scene.id} name={scene.id} from={scene.start} durationInFrames={sceneDuration(scene)}>
            <Component />
          </Sequence>
        )
      })}
    </AbsoluteFill>
  )
}
