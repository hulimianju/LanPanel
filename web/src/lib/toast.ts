import { reactive } from 'vue'

export interface Toast {
  id: number
  kind: 'success' | 'error' | 'info'
  text: string
}

export const toasts = reactive<Toast[]>([])
let seq = 0

export function toast(text: string, kind: Toast['kind'] = 'success', ms = 2600) {
  const t = { id: ++seq, kind, text }
  toasts.push(t)
  setTimeout(() => {
    const i = toasts.findIndex((x) => x.id === t.id)
    if (i >= 0) toasts.splice(i, 1)
  }, ms)
}

export function toastError(e: unknown) {
  toast(e instanceof Error ? e.message : String(e), 'error', 4000)
}
