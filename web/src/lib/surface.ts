import type { Settings } from './types'
import { hexLuminance, TONE_THRESHOLD } from './luminance'

export type ResolvedTone = 'light' | 'dark'

/**
 * 计算面板表面色调：
 * - 无壁纸：跟随界面主题
 * - 纯色壁纸：按颜色亮度
 * - 图片壁纸：手动指定优先；自动时按采样亮度（未知时按深色处理，深色卡片在多数照片上更稳）
 */
export function resolveTone(s: Settings, uiTheme: ResolvedTone, sampled?: number): ResolvedTone {
  const wp = s.wallpaper
  if (wp.type === 'none') return uiTheme
  if (wp.tone !== 'auto') return wp.tone
  const lum = wp.type === 'color' ? hexLuminance(wp.value) : sampled ?? (wp.luminance >= 0 ? wp.luminance : -1)
  if (lum < 0) return 'dark'
  return lum > TONE_THRESHOLD ? 'light' : 'dark'
}
