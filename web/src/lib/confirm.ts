import { reactive } from 'vue'

interface ConfirmState {
  open: boolean
  title: string
  message: string
  confirmText: string
  danger: boolean
  resolve?: (v: boolean) => void
}

export const confirmState = reactive<ConfirmState>({
  open: false, title: '', message: '', confirmText: '确定', danger: false,
})

/** 以 Promise 形式弹出确认框。 */
export function confirm(opts: { title: string; message?: string; confirmText?: string; danger?: boolean }) {
  return new Promise<boolean>((resolve) => {
    confirmState.resolve?.(false)
    Object.assign(confirmState, {
      open: true, title: opts.title, message: opts.message ?? '', confirmText: opts.confirmText ?? '确定',
      danger: opts.danger ?? false, resolve,
    })
  })
}
