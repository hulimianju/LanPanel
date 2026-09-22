<script setup lang="ts">
import { DialogClose, DialogContent, DialogDescription, DialogOverlay, DialogPortal, DialogRoot, DialogTitle } from 'reka-ui'
import { X } from 'lucide-vue-next'

const open = defineModel<boolean>('open', { default: false })
defineProps<{ title: string; description?: string; width?: string }>()
</script>

<template>
  <DialogRoot v-model:open="open">
    <DialogPortal>
      <DialogOverlay class="lp-overlay fixed inset-0 z-40 bg-[var(--overlay)]" />
      <DialogContent
        class="lp-dialog fixed left-1/2 top-1/2 z-50 flex max-h-[calc(100dvh-32px)] w-[calc(100vw-32px)] -translate-x-1/2 -translate-y-1/2 flex-col rounded-xl border border-line bg-surface text-fg shadow-pop outline-none"
        :style="{ maxWidth: width ?? '480px' }"
      >
        <div class="flex items-start gap-3 px-5 pt-5 pb-3">
          <div class="flex grow flex-col gap-1">
            <DialogTitle class="m-0 text-base font-semibold">{{ title }}</DialogTitle>
            <DialogDescription v-if="description" class="m-0 text-[13px] text-fg-3">{{ description }}</DialogDescription>
          </div>
          <DialogClose aria-label="关闭" class="lp-focus -mr-1 -mt-1 flex size-8 items-center justify-center rounded-sm text-fg-3 hover:bg-surface-2 hover:text-fg">
            <X class="size-4" />
          </DialogClose>
        </div>
        <div class="min-h-0 overflow-y-auto px-5 pb-5"><slot /></div>
        <div v-if="$slots.footer" class="flex justify-end gap-2 border-t border-line px-5 py-3.5"><slot name="footer" /></div>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<style>
.lp-overlay[data-state='open'] { animation: lp-fade 150ms ease-out; }
.lp-dialog[data-state='open'] { animation: lp-pop 180ms cubic-bezier(0.16, 1, 0.3, 1); }
@keyframes lp-fade { from { opacity: 0; } }
@keyframes lp-pop { from { opacity: 0; transform: translate(-50%, -48%) scale(0.97); } }
</style>
