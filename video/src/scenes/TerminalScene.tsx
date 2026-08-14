import { interpolate, spring, useCurrentFrame, useVideoConfig } from 'remotion'
import { SceneLayer } from '../components/SceneLayer'
import { Eyebrow, Headline } from '../components/Typography'
import { colors, fonts } from '../theme'
import { copyLines, copyString, getScene, sceneDuration } from '../storyboard'

const scene = getScene('terminal')

export function TerminalScene() {
  const frame = useCurrentFrame()
  const { fps } = useVideoConfig()
  const panel = spring({ frame: frame - 3, fps, config: { damping: 18, stiffness: 100 } })
  const command = copyString(scene, 'command')
  const commandProgress = Math.floor(interpolate(frame, [11, 38], [0, command.length], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' }))
  const output = copyLines(scene, 'output')

  return (
    <SceneLayer durationInFrames={sceneDuration(scene)}>
      <div style={{ position: 'absolute', left: 106, top: 210, display: 'flex', flexDirection: 'column', gap: 24 }}>
        <Eyebrow delay={5}>{copyString(scene, 'eyebrow')}</Eyebrow>
        <Headline width={730} delay={8}>{copyString(scene, 'headline')}</Headline>
      </div>

      <div style={{ position: 'absolute', left: 905, top: 170, width: 850, height: 650, borderRadius: 26, overflow: 'hidden', border: `1px solid ${colors.border}`, background: colors.deep, boxShadow: '0 46px 130px #000A', opacity: panel, transform: `perspective(1300px) rotateY(${-4 * (1 - panel)}deg) translateX(${(1 - panel) * 80}px)` }}>
        <div style={{ height: 62, display: 'flex', alignItems: 'center', gap: 14, padding: '0 24px', background: colors.chrome, borderBottom: `1px solid ${colors.border}` }}>
          {['#E06C75', '#E5C07B', '#67C587'].map((color) => <span key={color} style={{ width: 15, height: 15, borderRadius: '50%', background: color }} />)}
          <span style={{ marginLeft: 16, color: colors.muted, fontSize: 19 }}>local shell</span>
        </div>
        <div style={{ padding: '54px 58px', fontFamily: fonts.mono, fontSize: 31, lineHeight: 1.75 }}>
          <div style={{ color: colors.text, minHeight: 58 }}>{command.slice(0, commandProgress)}<span style={{ color: colors.orange, opacity: frame % 14 < 9 ? 1 : 0 }}>▋</span></div>
          <div style={{ marginTop: 26, display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '8px 36px' }}>
            {output.map((line, index) => {
              const visible = interpolate(frame, [40 + index * 6, 46 + index * 6], [0, 1], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' })
              return <div key={line} style={{ color: index === 0 ? colors.green : index === 2 ? colors.cyan : colors.muted, opacity: visible, transform: `translateY(${(1 - visible) * 12}px)` }}>{line}</div>
            })}
          </div>
        </div>
      </div>
    </SceneLayer>
  )
}
