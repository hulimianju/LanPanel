<script setup lang="ts" generic="T extends string">
const model = defineModel<T>({ required: true })
defineProps<{ options: { value: T; label: string }[]; label?: string; block?: boolean }>()
</script>

<template>
  <div role="radiogroup" :aria-label="label" class="inline-flex gap-0.5 rounded-[9px] bg-surface-2 p-[3px]" :class="block ? 'flex w-full' : 'self-start'">
    <button
      v-for="o in options"
      :key="o.value"
      type="button"
      role="radio"
      :aria-checked="model === o.value"
      class="lp-focus h-7 rounded-xs px-3 text-xs transition-colors"
      :class="[
        block ? 'flex-1' : '',
        model === o.value ? 'bg-surface font-medium text-fg shadow-[0_1px_2px_rgba(0,0,0,0.08)]' : 'text-fg-2 hover:text-fg',
      ]"
      @click="model = o.value"
    >
      {{ o.label }}
    </button>
  </div>
</template>
