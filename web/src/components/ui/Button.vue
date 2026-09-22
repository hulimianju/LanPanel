<script setup lang="ts">
import { LoaderCircle } from 'lucide-vue-next'

withDefaults(
  defineProps<{
    variant?: 'primary' | 'secondary' | 'ghost' | 'danger'
    size?: 'sm' | 'md'
    loading?: boolean
    disabled?: boolean
    type?: 'button' | 'submit'
    iconOnly?: boolean
  }>(),
  { variant: 'secondary', size: 'md', type: 'button' },
)
</script>

<template>
  <button
    :type="type"
    :disabled="disabled || loading"
    class="lp-focus inline-flex shrink-0 items-center justify-center gap-1.5 whitespace-nowrap rounded-sm font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-55"
    :class="[
      size === 'sm' ? 'h-8 text-[13px]' : 'h-9 text-[13px]',
      iconOnly ? (size === 'sm' ? 'w-8' : 'w-9') : size === 'sm' ? 'px-3' : 'px-3.5',
      {
        'bg-accent text-accent-fg hover:bg-accent-hover': variant === 'primary',
        'border border-line bg-surface text-fg hover:bg-surface-hover': variant === 'secondary',
        'text-fg-2 hover:bg-surface-2 hover:text-fg': variant === 'ghost',
        'border border-line bg-surface text-danger-fg hover:bg-danger-soft': variant === 'danger',
      },
    ]"
  >
    <LoaderCircle v-if="loading" class="size-4 animate-spin" />
    <slot />
  </button>
</template>
