<script setup lang="ts">
defineProps<{ label: string; hint?: string; error?: string; for?: string }>()
</script>

<template>
  <div class="flex flex-col gap-1.5">
    <label :for="$props.for" class="text-xs font-medium text-fg-2">{{ label }}</label>
    <slot />
    <p v-if="error" class="text-xs text-danger-fg">{{ error }}</p>
    <p v-else-if="hint" class="text-xs text-fg-3">{{ hint }}</p>
  </div>
</template>
