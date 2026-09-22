<script setup lang="ts">
import { CircleAlert, CircleCheck, Info } from 'lucide-vue-next'
import { toasts } from '@/lib/toast'
</script>

<template>
  <div class="pointer-events-none fixed inset-x-0 bottom-5 z-[60] flex flex-col items-center gap-2 px-4" aria-live="polite">
    <TransitionGroup name="lp-toast">
      <div
        v-for="t in toasts"
        :key="t.id"
        class="pointer-events-auto flex max-w-md items-center gap-2 rounded-md border border-line bg-surface px-3.5 py-2.5 text-[13px] text-fg shadow-pop"
      >
        <CircleCheck v-if="t.kind === 'success'" class="size-4 shrink-0 text-success" />
        <CircleAlert v-else-if="t.kind === 'error'" class="size-4 shrink-0 text-danger" />
        <Info v-else class="size-4 shrink-0 text-accent" />
        <span>{{ t.text }}</span>
      </div>
    </TransitionGroup>
  </div>
</template>

<style>
.lp-toast-enter-active, .lp-toast-leave-active { transition: all 200ms ease; }
.lp-toast-enter-from, .lp-toast-leave-to { opacity: 0; transform: translateY(8px); }
</style>
