<script setup lang="ts">
import { computed } from 'vue'
import { SliderRange, SliderRoot, SliderThumb, SliderTrack } from 'reka-ui'

const model = defineModel<number>({ required: true })
const props = defineProps<{ min: number; max: number; step?: number; label: string }>()
const arr = computed({
  get: () => [model.value],
  set: (v: number[] | undefined) => (model.value = v?.[0] ?? props.min),
})
</script>

<template>
  <SliderRoot v-model="arr" :min="min" :max="max" :step="step ?? 1" class="relative flex h-5 w-full touch-none select-none items-center">
    <SliderTrack class="relative h-1.5 grow overflow-hidden rounded-full bg-surface-2">
      <SliderRange class="absolute h-full rounded-full bg-accent" />
    </SliderTrack>
    <SliderThumb :aria-label="label" class="lp-focus block size-4 rounded-full border border-line-strong bg-white shadow-sm" />
  </SliderRoot>
</template>
