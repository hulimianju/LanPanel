<script setup lang="ts">
const model = defineModel<string>({ default: '' })
defineProps<{ mono?: boolean; placeholder?: string; type?: string; invalid?: boolean; autofocus?: boolean; autocomplete?: string }>()
</script>

<template>
  <input
    v-model="model"
    :type="type ?? 'text'"
    :placeholder="placeholder"
    :autofocus="autofocus"
    :autocomplete="autocomplete"
    :aria-invalid="invalid || undefined"
    class="h-9 w-full min-w-0 rounded-sm border bg-surface px-3 text-[13px] text-fg outline-none transition-[border-color,box-shadow] placeholder:text-fg-3 focus:border-accent focus:shadow-[0_0_0_3px_var(--focus)]"
    :class="[mono ? 'font-mono' : '', invalid ? 'border-danger' : 'border-line']"
  />
</template>
