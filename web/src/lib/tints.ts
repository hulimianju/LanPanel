import type { Tint } from './types'

/** 图标色调：深色表面用高明度色，浅色表面用低明度色；底色为同色 14% 透明度。 */
const TINTS: Record<Tint, { dark: string; light: string }> = {
  blue: { dark: '#8FA8FF', light: '#3A5BD9' },
  teal: { dark: '#5ED3BA', light: '#0F7B63' },
  amber: { dark: '#F2BD62', light: '#B45309' },
  violet: { dark: '#B4A1FF', light: '#6D4ED8' },
  rose: { dark: '#F79AAB', light: '#B4235A' },
  green: { dark: '#7BD88F', light: '#1E7F3A' },
  slate: { dark: '#A8B0BD', light: '#4B5563' },
}

export const TINT_NAMES = Object.keys(TINTS) as Tint[]

export function tintColor(t: Tint | '' | undefined, tone: 'light' | 'dark') {
  return TINTS[t || 'blue'][tone]
}

export function tintBg(t: Tint | '' | undefined, tone: 'light' | 'dark', alpha = tone === 'dark' ? 0.16 : 0.12) {
  const hex = tintColor(t, tone)
  const n = parseInt(hex.slice(1), 16)
  return `rgba(${n >> 16}, ${(n >> 8) & 255}, ${n & 255}, ${alpha})`
}

/** 根据标题生成稳定的默认色调。 */
export function tintFor(text: string): Tint {
  let h = 0
  for (const ch of text) h = (h * 31 + ch.codePointAt(0)!) >>> 0
  return TINT_NAMES[h % TINT_NAMES.length]
}
