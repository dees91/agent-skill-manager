import type { CSSProperties } from 'react'

export const colors = {
  canvas: '#161617',
  chrome: '#212123',
  deep: '#0D0D0E',
  subtle: '#262627',
  border: '#38383B',
  text: '#F2F2F2',
  muted: '#A7A7AC',
  cyan: '#50B0E0',
  blue: '#6090E0',
  orange: '#E08050',
  green: '#67C587',
} as const

export const fonts = {
  sans: '-apple-system, BlinkMacSystemFont, "SF Pro Display", system-ui, sans-serif',
  mono: 'ui-monospace, "SFMono-Regular", Menlo, Monaco, monospace',
} as const

export const fullFrame: CSSProperties = {
  position: 'absolute',
  inset: 0,
  width: '100%',
  height: '100%',
}
