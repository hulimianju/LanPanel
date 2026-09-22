/**
 * 壁纸亮度分析：决定面板用深色还是浅色调，并估算文字对比度。
 * 采用 WCAG 相对亮度（sRGB 线性化），与对比度公式保持一致。
 */

function lin(c: number) {
  const s = c / 255
  return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4)
}

export function relLuminance(r: number, g: number, b: number) {
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b)
}

export function hexLuminance(hex: string) {
  const m = /^#?([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(hex)
  if (!m) return 0
  return relLuminance(parseInt(m[1], 16), parseInt(m[2], 16), parseInt(m[3], 16))
}

export function contrast(l1: number, l2: number) {
  const [a, b] = l1 > l2 ? [l1, l2] : [l2, l1]
  return (a + 0.05) / (b + 0.05)
}

/**
 * 采样图片亮度。时钟与搜索框所在的中上区域权重 0.6，整图 0.4；
 * 同时取该区域较亮的 90 分位，避免局部高光让浅色文字看不清。
 */
export function sampleImageLuminance(src: string): Promise<number> {
  return new Promise((resolve, reject) => {
    const img = new Image()
    img.decoding = 'async'
    img.onload = () => {
      const W = 64
      const H = Math.max(16, Math.round((64 * img.naturalHeight) / Math.max(1, img.naturalWidth)))
      const canvas = document.createElement('canvas')
      canvas.width = W
      canvas.height = H
      const ctx = canvas.getContext('2d', { willReadFrequently: true })
      if (!ctx) return reject(new Error('canvas 不可用'))
      ctx.drawImage(img, 0, 0, W, H)
      const { data } = ctx.getImageData(0, 0, W, H)
      let all = 0
      const region: number[] = []
      for (let y = 0; y < H; y++) {
        for (let x = 0; x < W; x++) {
          const i = (y * W + x) * 4
          const l = relLuminance(data[i], data[i + 1], data[i + 2])
          all += l
          if (y > H * 0.08 && y < H * 0.62 && x > W * 0.18 && x < W * 0.82) region.push(l)
        }
      }
      all /= W * H
      region.sort((a, b) => a - b)
      const avg = region.reduce((a, b) => a + b, 0) / Math.max(1, region.length)
      const p90 = region[Math.floor(region.length * 0.9)] ?? avg
      resolve(Math.min(1, 0.4 * all + 0.4 * avg + 0.2 * p90))
    }
    img.onerror = () => reject(new Error('图片加载失败'))
    img.src = src
  })
}

/** 深色调文字（近白）与浅色调文字（近黑）的相对亮度。 */
const FG_ON_DARK = hexLuminance('#F7F7F8')
const FG_ON_LIGHT = hexLuminance('#111113')

/** 加遮罩后的背景亮度：深色调叠黑色，浅色调叠白色。 */
export function effectiveLuminance(lum: number, tone: 'light' | 'dark', dim: number) {
  const a = dim / 100
  return tone === 'dark' ? lum * (1 - a) : lum * (1 - a) + a
}

/**
 * 以卡片外的裸露文字（时钟、分组标题）为准估算对比度，
 * 并给出达到 4.5:1 所需的最小遮罩强度（0-80）。
 */
export function assessReadability(lum: number, tone: 'light' | 'dark', dim: number) {
  const fg = tone === 'dark' ? FG_ON_DARK : FG_ON_LIGHT
  const ratio = contrast(fg, effectiveLuminance(lum, tone, dim))
  let recommended = dim
  if (ratio < 4.5) {
    recommended = 80
    for (let d = 0; d <= 80; d += 5) {
      if (contrast(fg, effectiveLuminance(lum, tone, d)) >= 4.5) {
        recommended = d
        break
      }
    }
  }
  return { ratio, recommended, ok: ratio >= 4.5 }
}

/** 亮度阈值：高于此值用浅色调（深色文字）。 */
// 白字与黑字对比度相等的交叉点约为 0.18，略向深色调偏移（深色卡片更耐看），不足部分由遮罩补足
export const TONE_THRESHOLD = 0.25
