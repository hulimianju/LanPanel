<script setup lang="ts">
import { computed } from 'vue'
import Dialog from './Dialog.vue'
import Button from './Button.vue'
import { confirmState } from '@/lib/confirm'

const open = computed({
  get: () => confirmState.open,
  set: (v) => {
    if (!v) done(false)
  },
})

function done(v: boolean) {
  confirmState.open = false
  confirmState.resolve?.(v)
  confirmState.resolve = undefined
}
</script>

<template>
  <Dialog v-model:open="open" :title="confirmState.title" width="400px">
    <p v-if="confirmState.message" class="m-0 text-[13px] leading-relaxed text-fg-2">{{ confirmState.message }}</p>
    <template #footer>
      <Button @click="done(false)">取消</Button>
      <Button :variant="confirmState.danger ? 'danger' : 'primary'" @click="done(true)">{{ confirmState.confirmText }}</Button>
    </template>
  </Dialog>
</template>
