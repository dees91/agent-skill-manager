import { AbsoluteFill, Img, staticFile } from 'remotion'
import { SOCIAL_PREVIEW } from './storyboard'
import { colors, fonts } from './theme'

export function SocialPreview() {
  return (
    <AbsoluteFill
      style={{
        backgroundColor: colors.canvas,
        color: colors.text,
        fontFamily: fonts.sans,
        overflow: 'hidden',
      }}
    >
      <div
        style={{
          position: 'absolute',
          inset: 0,
          background:
            'radial-gradient(circle at 22% 45%, rgba(80,176,224,0.14), transparent 34%), radial-gradient(circle at 85% 8%, rgba(224,128,80,0.10), transparent 30%)',
        }}
      />
      <div
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          width: 14,
          height: 640,
          backgroundColor: colors.cyan,
        }}
      />

      <div
        style={{
          position: 'absolute',
          left: 68,
          top: 76,
          width: 390,
          height: 488,
          borderRadius: 34,
          border: `1px solid ${colors.border}`,
          backgroundColor: colors.chrome,
          boxShadow: '0 28px 70px rgba(0,0,0,0.34)',
          padding: '38px 34px',
          boxSizing: 'border-box',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 22 }}>
          <Img
            name="Skill Manager icon"
            src={staticFile(SOCIAL_PREVIEW.asset)}
            style={{ width: 104, height: 104, borderRadius: 24 }}
          />
          <div>
            <div
              style={{
                color: colors.text,
                fontSize: 28,
                fontWeight: 700,
                letterSpacing: -0.5,
              }}
            >
              {SOCIAL_PREVIEW.copy.product}
            </div>
            <div
              style={{
                color: colors.muted,
                fontFamily: fonts.mono,
                fontSize: 15,
                marginTop: 8,
              }}
            >
              {SOCIAL_PREVIEW.copy.inventory}
            </div>
          </div>
        </div>

        <div
          style={{
            display: 'grid',
            gridTemplateColumns: '1fr 62px 62px',
            gap: 10,
            color: colors.muted,
            fontSize: 12,
            fontWeight: 700,
            letterSpacing: 1.2,
            marginTop: 38,
            padding: '0 10px 9px',
            textTransform: 'uppercase',
          }}
        >
          <span>Skill</span>
          <span style={{ textAlign: 'center' }}>Claude</span>
          <span style={{ textAlign: 'center' }}>Codex</span>
        </div>

        {SOCIAL_PREVIEW.rows.map((skill) => (
          <div
            key={skill.name}
            style={{
              display: 'grid',
              gridTemplateColumns: '1fr 62px 62px',
              alignItems: 'center',
              gap: 10,
              height: 70,
              borderTop: `1px solid ${colors.border}`,
              padding: '0 10px',
              boxSizing: 'border-box',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0 }}>
              <span
                style={{
                  width: 9,
                  height: 9,
                  borderRadius: 9,
                  backgroundColor: colors[skill.accent],
                  flex: '0 0 auto',
                }}
              />
              <span
                style={{
                  color: colors.text,
                  fontFamily: fonts.mono,
                  fontSize: 12.5,
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                }}
              >
                {skill.name}
              </span>
            </div>
            {[skill.claude, skill.codex].map((state, index) => (
              <span
                key={`${skill.name}-${index}`}
                style={{
                  justifySelf: 'center',
                  minWidth: 42,
                  borderRadius: 14,
                  backgroundColor: state === 'ON' ? 'rgba(103,197,135,0.14)' : colors.subtle,
                  color: state === 'ON' ? colors.green : colors.muted,
                  fontFamily: fonts.mono,
                  fontSize: 12,
                  fontWeight: 700,
                  padding: '6px 8px',
                  textAlign: 'center',
                }}
              >
                {state}
              </span>
            ))}
          </div>
        ))}
      </div>

      <div
        style={{
          position: 'absolute',
          left: 524,
          top: 110,
          width: 674,
        }}
      >
        <div
          style={{
            color: colors.cyan,
            fontFamily: fonts.mono,
            fontSize: 20,
            fontWeight: 700,
            letterSpacing: 2.6,
            textTransform: 'uppercase',
          }}
        >
          {SOCIAL_PREVIEW.copy.product}
        </div>
        <div
          style={{
            color: colors.text,
            fontSize: 78,
            fontWeight: 760,
            letterSpacing: -3.7,
            lineHeight: 0.98,
            marginTop: 30,
            maxWidth: 660,
            whiteSpace: 'pre-line',
          }}
        >
          {SOCIAL_PREVIEW.copy.headline.replace(', ', ',\n')}
        </div>
        <div
          style={{
            color: colors.muted,
            fontSize: 27,
            fontWeight: 500,
            letterSpacing: -0.4,
            marginTop: 34,
          }}
        >
          {SOCIAL_PREVIEW.copy.supporting}
        </div>
        <div
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: 12,
            border: `1px solid ${colors.border}`,
            borderRadius: 22,
            backgroundColor: colors.chrome,
            color: colors.text,
            fontFamily: fonts.mono,
            fontSize: 18,
            marginTop: 46,
            padding: '12px 18px',
          }}
        >
          <span style={{ width: 8, height: 8, borderRadius: 8, backgroundColor: colors.orange }} />
          {SOCIAL_PREVIEW.copy.surfaces}
        </div>
      </div>
    </AbsoluteFill>
  )
}
