<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Globe } from 'lucide-vue-next'
import type { Icon } from '@/lib/types'
import { ICONS } from '@/lib/icons'
import { tintBg, tintColor, tintFor } from '@/lib/tints'

const props = withDefaults(defineProps<{ icon: Icon; title: string; tone: 'light' | 'dark'; size?: number }>(), { size: 40 })

const failed = ref(false)
watch(() => props.icon.value, () => (failed.value = false))

const tint = computed(() => props.icon.color || tintFor(props.title))
const showImage = computed(() => (props.icon.type === 'image' || props.icon.type === 'auto') && props.icon.value && !failed.value)
const letters = computed(() => {
  const src = props.icon.type === 'text' && props.icon.value ? props.icon.value : props.title
  const t = src.trim()
  // 中文取首字，英文取首字母（最多两位）
  return /^[\x00-\x7F]/.test(t) ? t.slice(0, props.icon.type === 'text' ? 2 : 1).toUpperCase() : Array.from(t)[0] ?? '?'
})
const radius = computed(() => Math.round(props.size / 4))
</script>

<template>
  <div
    class="flex shrink-0 items-center justify-center overflow-hidden"
    :style="{
      width: size + 'px',
      height: size + 'px',
      borderRadius: radius + 'px',
      background: showImage ? (tone === 'dark' ? 'rgba(255,255,255,0.08)' : '#F4F4F5') : tintBg(tint, tone),
    }"
  >
    <img
      v-if="showImage"
      :src="icon.value"
      alt=""
      loading="lazy"
      class="object-contain"
      :style="{ width: size * 0.66 + 'px', height: size * 0.66 + 'px' }"
      @error="failed = true"
    />
    <component
      :is="ICONS[icon.value] ?? Globe"
      v-else-if="icon.type === 'lucide'"
      :size="Math.round(size / 2)"
      :stroke-width="1.8"
      :color="tintColor(tint, tone)"
    />
    <span
      v-else
      class="font-semibold leading-none"
      :style="{ color: tintColor(tint, tone), fontSize: Math.round(size * (letters.length > 1 ? 0.36 : 0.44)) + 'px' }"
    >{{ letters }}</span>
  </div>
</template>
