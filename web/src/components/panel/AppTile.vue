<script setup lang="ts">
import { computed } from 'vue'
import { Link2, Pencil, Trash2 } from 'lucide-vue-next'
import type { Item } from '@/lib/types'
import { itemUrl } from '@/lib/address'
import { shortUrl } from '@/lib/format'
import AppIcon from './AppIcon.vue'

const props = defineProps<{ item: Item; tone: 'light' | 'dark'; mode: 'lan' | 'wan'; editing: boolean; compact: boolean }>()
const emit = defineEmits<{ edit: [Item]; remove: [Item] }>()

const url = computed(() => itemUrl(props.item, props.mode))
const usingFallback = computed(() => (props.mode === 'wan' ? !props.item.urlWan : !props.item.urlLan))
const tag = computed(() => (props.editing ? 'div' : 'a'))
</script>

<template>
  <component
    :is="tag"
    :href="editing ? undefined : url"
    :target="!editing && item.openMode === 'new' ? '_blank' : undefined"
    :rel="!editing && item.openMode === 'new' ? 'noopener noreferrer' : undefined"
    :title="item.desc ? `${item.title} — ${item.desc}` : item.title"
    class="lp-tile group/tile relative flex min-w-0 flex-col items-center gap-1.5 rounded-lg text-center outline-none transition-[background-color,transform] duration-150 focus-visible:shadow-[0_0_0_3px_var(--focus)] sm:lp-glass sm:flex-row sm:gap-3 sm:text-left sm:hover:bg-s-glass-hover"
    :class="[
      compact ? 'sm:h-14 sm:px-3' : 'sm:h-[68px] sm:px-3.5',
      editing ? 'cursor-grab active:cursor-grabbing lp-wiggle' : 'sm:hover:-translate-y-px',
    ]"
  >
    <!-- 手机端图标直接叠在壁纸上，垫一层毛玻璃保证辨识度 -->
    <span class="shrink-0 max-sm:lp-glass max-sm:rounded-[15px]">
      <AppIcon :icon="item.icon" :title="item.title" :tone="tone" :size="compact ? 34 : 40" class="max-sm:!size-14 max-sm:!rounded-[15px]" />
    </span>
    <div class="flex min-w-0 max-w-full grow flex-col gap-0.5">
      <span class="truncate text-[11.5px] text-s-fg max-sm:lp-text-shadow sm:text-sm sm:font-medium">{{ item.title }}</span>
      <span v-if="!compact" class="hidden truncate font-mono text-xs text-s-fg-3 sm:block">
        {{ shortUrl(url) }}<span v-if="usingFallback && url" class="ml-1 font-sans">（{{ mode === 'wan' ? '内网' : '外网' }}）</span>
      </span>
    </div>
    <Link2 v-if="item.deviceMac && !editing" class="hidden size-3.5 shrink-0 text-s-chip-fg sm:block" aria-label="已绑定设备，IP 变化时自动更新" />
    <div v-if="editing" class="absolute -right-1.5 -top-1.5 flex gap-1 sm:static sm:ml-auto">
      <button type="button" class="lp-focus flex size-7 items-center justify-center rounded-full border border-s-border bg-s-glass-strong text-s-fg-2 hover:text-s-fg sm:rounded-md" aria-label="编辑" @click.stop="emit('edit', item)">
        <Pencil class="size-3.5" />
      </button>
      <button type="button" class="lp-focus flex size-7 items-center justify-center rounded-full border border-s-border bg-s-glass-strong text-s-fg-2 hover:text-danger sm:rounded-md" aria-label="删除" @click.stop="emit('remove', item)">
        <Trash2 class="size-3.5" />
      </button>
    </div>
  </component>
</template>

<style>
@keyframes lp-wiggle {
  0%, 100% { transform: rotate(-0.4deg); }
  50% { transform: rotate(0.4deg); }
}
@media (max-width: 639px) {
  .lp-wiggle { animation: lp-wiggle 0.3s ease-in-out infinite; }
}
.lp-tile.sortable-ghost { opacity: 0.35; }
.lp-tile.sortable-drag { opacity: 0.95; }
</style>
