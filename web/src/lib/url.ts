/** 与后端 internal/linkage 保持一致的地址解析。 */

function hostSpan(raw: string): [number, number] | null {
  const i = raw.indexOf('://')
  if (i <= 0) return null
  let start = i + 3
  let end = raw.length
  const m = raw.slice(start).search(/[/?#]/)
  if (m >= 0) end = start + m
  let auth = raw.slice(start, end)
  const at = auth.lastIndexOf('@')
  if (at >= 0) {
    start += at + 1
    auth = raw.slice(start, end)
  }
  if (auth.startsWith('[')) return null
  const c = auth.lastIndexOf(':')
  if (c >= 0) end = start + c
  return end > start ? [start, end] : null
}

/** 地址中的 IPv4 主机；域名返回 null。 */
export function hostIP(raw: string): string | null {
  const s = hostSpan(raw)
  if (!s) return null
  const h = raw.slice(s[0], s[1])
  return /^\d{1,3}(\.\d{1,3}){3}$/.test(h) ? h : null
}

const DEFAULT_PORTS: Record<string, number> = { http: 80, https: 443, ssh: 22, ftp: 21, smb: 445, rtsp: 554, vnc: 5900, afp: 548 }

/** 地址中的端口；未写端口时按协议推断。 */
export function urlPort(raw: string): number {
  const s = hostSpan(raw)
  if (!s) return 0
  const rest = raw.slice(s[1])
  if (rest.startsWith(':')) {
    const n = parseInt(rest.slice(1), 10)
    return n > 0 && n < 65536 ? n : 0
  }
  return DEFAULT_PORTS[raw.slice(0, raw.indexOf('://')).toLowerCase()] ?? 0
}
