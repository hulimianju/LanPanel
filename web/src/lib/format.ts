export function bytes(n: number, digits = 1) {
  if (!Number.isFinite(n) || n < 0) return '-'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  while (n >= 1024 && i < u.length - 1) {
    n /= 1024
    i++
  }
  return `${n.toFixed(i === 0 || n >= 100 ? 0 : digits)} ${u[i]}`
}

export function duration(sec: number) {
  if (!sec) return '-'
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (d) return `${d} 天 ${h} 小时`
  if (h) return `${h} 小时 ${m} 分`
  return `${m} 分钟`
}

/** 卡片副标题：去掉协议和末尾斜杠，只显示主机与端口。 */
export function shortUrl(u: string) {
  return u.replace(/^[a-z]+:\/\//i, '').replace(/\/$/, '')
}
