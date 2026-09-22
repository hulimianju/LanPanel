<script setup lang="ts">
import { computed } from 'vue'
import { useApp } from '@/stores/app'
import Wallpaper from '@/components/panel/Wallpaper.vue'

defineProps<{ title: string; subtitle: string }>()
const app = useApp()
const wp = computed(() => app.settings!.wallpaper)
</script>

<template>
  <div
    class="lp-surface relative isolate flex min-h-dvh items-center justify-center p-4"
    :data-tone="app.surfaceTone"
    :data-wp="wp.type === 'image' ? 'image' : 'plain'"
  >
    <Wallpaper :wallpaper="wp" :tone="app.surfaceTone" />
    <!-- 表单卡片跟随界面主题，而不是壁纸色调，保证输入控件一致 -->
    <div class="w-full max-w-[380px] rounded-xl border border-line bg-surface p-8 text-fg shadow-pop">
      <div class="mb-6 flex flex-col items-center gap-3 text-center">
        <img src="/favicon.svg" alt="" class="size-14" />
        <h1 class="m-0 text-xl font-semibold">{{ title }}</h1>
        <p class="m-0 text-[13px] text-fg-3">{{ subtitle }}</p>
      </div>
      <slot />
    </div>
  </div>
</template>
