import { Img, interpolate, staticFile, useCurrentFrame } from 'remotion'
import { colors } from '../theme'

type AppViewportProps = {
  asset: string
  zoom?: readonly [number, number]
  panX?: readonly [number, number]
  panY?: readonly [number, number]
  imageOpacity?: number
}

export function AppViewport({ asset, zoom = [1, 1.035], panX = [0, 0], panY = [0, 0], imageOpacity = 1 }: AppViewportProps) {
  const frame = useCurrentFrame()
  const scale = interpolate(frame, [0, 100], [...zoom], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' })
  const x = interpolate(frame, [0, 100], [...panX], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' })
  const y = interpolate(frame, [0, 100], [...panY], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' })

  return (
    <div
      style={{
        position: 'absolute',
        left: 210,
        top: 74,
        width: 1500,
        height: 900,
        overflow: 'hidden',
        borderRadius: 28,
        border: `1px solid ${colors.border}`,
        background: colors.chrome,
        boxShadow: '0 46px 140px #000000A8, 0 0 0 1px #FFFFFF08 inset',
        opacity: imageOpacity,
        transform: 'perspective(1700px) rotateX(1.1deg)',
        transformOrigin: 'center center',
      }}
    >
      <Img
        src={staticFile(asset)}
        style={{
          width: 1500,
          height: 1000,
          objectFit: 'cover',
          objectPosition: 'top center',
          transform: `translate(${x}px, ${y}px) scale(${scale})`,
          transformOrigin: 'center 44%',
        }}
      />
    </div>
  )
}
