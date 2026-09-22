export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
  }
}

type Listener = () => void
const unauthorizedListeners: Listener[] = []
/** 注册 401 回调（会话失效时跳转登录）。 */
export function onUnauthorized(fn: Listener) {
  unauthorizedListeners.push(fn)
}

async function request<T>(method: string, url: string, body?: unknown): Promise<T> {
  const init: RequestInit = { method, credentials: 'same-origin', headers: {} }
  if (body instanceof FormData) {
    init.body = body
  } else if (body !== undefined) {
    ;(init.headers as Record<string, string>)['Content-Type'] = 'application/json'
    init.body = JSON.stringify(body)
  }
  let res: Response
  try {
    res = await fetch(url, init)
  } catch {
    throw new ApiError(0, '无法连接服务器')
  }
  const text = await res.text()
  let data: any = null
  try {
    data = text ? JSON.parse(text) : null
  } catch {
    /* 非 JSON 响应 */
  }
  if (!res.ok) {
    if (res.status === 401 && !url.endsWith('/login')) unauthorizedListeners.forEach((f) => f())
    throw new ApiError(res.status, data?.error ?? `请求失败（${res.status}）`)
  }
  return data as T
}

export const api = {
  get: <T>(url: string) => request<T>('GET', url),
  post: <T>(url: string, body?: unknown) => request<T>('POST', url, body ?? {}),
  put: <T>(url: string, body?: unknown) => request<T>('PUT', url, body ?? {}),
  del: <T>(url: string) => request<T>('DELETE', url),
  upload: (file: File, kind: 'wallpaper' | 'icon') => {
    const fd = new FormData()
    fd.append('file', file)
    return request<{ url: string }>('POST', `/api/upload?kind=${kind}`, fd)
  },
}
