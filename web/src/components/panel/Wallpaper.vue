<script setup lang="ts">
import { computed } from 'vue'
import type { Wallpaper } from '@/lib/types'

const props = defineProps<{ wallpaper: Wallpaper; tone: 'light' | 'dark' }>()

const imageStyle = computed(() => {
  const blur = props.wallpaper.blur
  return {
    backgroundImage: `url("${props.wallpaper.value}")`,
    filter: blur ? `blur(${blur}px)` : undefined,
    // 模糊会在边缘露出透明区域，放大一点遮住
    transform: blur ? `scale(${1 + blur / 150})` : undefined,
  }
})

// 遮罩颜色与色调一致：深色调叠黑，浅色调叠白，都是在拉开与文字的对比度
const scrimStyle = computed(() => ({
  backgroundColor: props.tone === 'dark' ? `rgba(0,0,0,${props.wallpaper.dim / 100})` : `rgba(255,255,255,${props.wallpaper.dim / 100})`,
}))
</script>

<template>
  <div class="pointer-events-none fixed inset-0 -z-10 overflow-hidden" aria-hidden="true">
    <div v-if="wallpaper.type === 'image' && wallpaper.value" class="absolute inset-0 bg-cover bg-center" :style="imageStyle" />
    <div v-else-if="wallpaper.type === 'color'" class="absolute inset-0" :style="{ backgroundColor: wallpaper.value }" />
    <div v-else class="lp-dots absolute inset-0" />
    <div v-if="wallpaper.type === 'image'" class="absolute inset-0" :style="scrimStyle" />
  </div>
</template>
