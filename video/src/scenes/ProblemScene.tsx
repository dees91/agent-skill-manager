import { interpolate, spring, useCurrentFrame, useVideoConfig } from 'remotion'
import { SceneLayer } from '../components/SceneLayer'
import { Eyebrow, Headline } from '../components/Typography'
import { colors, fonts } from '../theme'
import { copyLines, copyString, getScene, sceneDuration } from '../storyboard'

const scene = getScene('problem')

export function ProblemScene() {
  const frame = useCurrentFrame()
  const { fps } = useVideoConfig()
  const settle = spring({ frame: frame - 8, fps, config: { damping: 18, stiffness: 95, mass: 0.9 } })
  const names = copyLines(scene, 'skillNames')

  return (
    <SceneLayer durationInFrames={sceneDuration(scene)}>
      <div style={{ position: 'absolute', left: 110, top: 124, display: 'flex', flexDirection: 'column', gap: 26 }}>
        <Eyebrow>{copyString(scene, 'eyebrow')}</Eyebrow>
        <Headline width={1050}>{copyString(scene, 'headline')}</Headline>
      </div>

      <FolderCard label="CLAUDE CODE" path={copyString(scene, 'claudePath')} x={1120} y={180} rotate={-4 * (1 - settle)} accent={colors.cyan} progress={settle} />
      <FolderCard label="CODEX" path={copyString(scene, 'codexPath')} x={1260} y={525} rotate={4.5 * (1 - settle)} accent={colors.blue} progress={settle} />

      {names.map((name, index) => {
        const enter = spring({ frame: frame - 24 - index * 5, fps, config: { damping: 17, stiffness: 125 } })
        const x = [760, 990, 870][index] ?? 820
        const y = [620, 735, 845][index] ?? 700
        return (
          <div key={name} style={{ position: 'absolute', left: x, top: y, padding: '14px 20px', borderRadius: 12, background: colors.deep, border: `1px solid ${colors.border}`, color: colors.muted, fontFamily: fonts.mono, fontSize: 20, opacity: enter, transform: `translateY(${(1 - enter) * 28}px)` }}>
            {name}
          </div>
        )
      })}

      <div style={{ position: 'absolute', left: 1050, top: 465, width: 185, height: 2, background: `linear-gradient(90deg, transparent, ${colors.orange}, transparent)`, opacity: interpolate(frame, [28, 45], [0, 0.9], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' }) }} />
    </SceneLayer>
  )
}

function FolderCard({ label, path, x, y, rotate, accent, progress }: { label: string; path: string; x: number; y: number; rotate: number; accent: string; progress: number }) {
  return (
    <div style={{ position: 'absolute', left: x, top: y, width: 520, padding: 34, borderRadius: 24, background: colors.chrome, border: `1px solid ${colors.border}`, boxShadow: '0 34px 90px #0009', opacity: progress, transform: `perspective(900px) translate(${(1 - progress) * 120}px, ${(1 - progress) * 35}px) rotateY(${rotate}deg)` }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 16, color: accent, fontSize: 22, fontWeight: 750, letterSpacing: 2.8 }}>
        <span style={{ display: 'block', width: 38, height: 28, borderRadius: 6, background: accent, boxShadow: `0 0 30px ${accent}50` }} />
        {label}
      </div>
      <div style={{ marginTop: 26, color: colors.text, fontFamily: fonts.mono, fontSize: 31, fontWeight: 600 }}>{path}</div>
    </div>
  )
}
