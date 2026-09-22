<script setup lang="ts">
import { computed } from 'vue'
import { Link2, Pencil, Trash2 } from 'lucide-vue-next'
import type { Item, ItemStatus } from '@/lib/types'
import { itemUrl } from '@/lib/address'
import { shortUrl } from '@/lib/format'
import AppIcon from './AppIcon.vue'

const props = defineProps<{ item: Item; tone: 'light' | 'dark'; mode: 'lan' | 'wan'; editing: boolean; compact: boolean; status?: ItemStatus }>()
const emit = defineEmits<{ edit: [Item]; remove: [Item] }>()

const url = computed(() => itemUrl(props.item, props.mode))
const usingFallback = computed(() => (props.mode === 'wan' ? !props.item.urlWan : !props.item.urlLan))
const tag = computed(() => (props.editing ? 'div' : 'a'))

const STATE = {
  online: { color: '#3DD68C', text: '在线' },
  warn: { color: '#F5B83D', text: '服务未响应' },
  offline: { color: '#8B8D98', text: '离线' },
} as const
const dot = computed(() => (props.status ? STATE[props.status.state] : null))
const tooltip = computed(() => {
  const base = props.item.desc ? `${props.item.title} — ${props.item.desc}` : props.item.title
  const s = props.status
  if (!s) return base
  const dev = s.device ? `${s.device}（${s.ip}）` : ''
  const lines = [base, [STATE[s.state].text, dev].filter(Boolean).join(' · ')]
  if (s.note) lines.push(s.note)
  if (s.bound) lines.push('已绑定设备，IP 变化时自动更新地址')
  return lines.join('\n')
})
</script>

<template>
  <component
    :is="tag"
    :href="editing ? undefined : url"
    :target="!editing && item.openMode === 'new' ? '_blank' : undefined"
    :rel="!editing && item.openMode === 'new' ? 'noopener noreferrer' : undefined"
    :title="tooltip"
    class="lp-tile group/tile relative flex min-w-0 flex-col items-center gap-1.5 rounded-lg text-center outline-none transition-[background-color,transform] duration-150 focus-visible:shadow-[0_0_0_3px_var(--focus)] sm:lp-glass sm:flex-row sm:gap-3 sm:text-left sm:hover:bg-s-glass-hover"
    :class="[
      compact ? 'sm:h-14 sm:px-3' : 'sm:h-[68px] sm:px-3.5',
      editing ? 'cursor-grab active:cursor-grabbing lp-wiggle' : 'sm:hover:-translate-y-px',
      status?.state === 'offline' && !editing ? 'opacity-55' : '',
    ]"
  >
    <!-- 手机端图标直接叠在壁纸上，垫一层毛玻璃保证辨识度 -->
    <span class="relative shrink-0 max-sm:lp-glass max-sm:rounded-[15px]">
      <AppIcon :icon="item.icon" :title="item.title" :tone="tone" :size="compact ? 34 : 40" class="max-sm:!size-14 max-sm:!rounded-[15px]" />
      <!-- 手机端：状态点放在图标右上角 -->
      <span v-if="dot && !editing" class="absolute -right-0.5 -top-0.5 size-3 rounded-full border-2 border-[var(--s-base,#0A0B0E)] sm:hidden" :style="{ background: dot.color }" />
    </span>
    <div class="flex min-w-0 max-w-full grow flex-col gap-0.5">
      <span class="truncate text-[11.5px] text-s-fg max-sm:lp-text-shadow sm:text-sm sm:font-medium">{{ item.title }}</span>
      <span v-if="!compact" class="hidden truncate font-mono text-xs text-s-fg-3 sm:block">
        {{ shortUrl(url) }}<span v-if="usingFallback && url" class="ml-1 font-sans">（{{ mode === 'wan' ? '内网' : '外网' }}）</span>
      </span>
    </div>
    <div v-if="!editing && (dot || item.deviceMac)" class="hidden shrink-0 flex-col items-end gap-2 self-stretch py-3.5 sm:flex">
      <span v-if="dot" class="size-[7px] rounded-full" :style="{ background: dot.color }" :aria-label="dot.text" />
      <span v-else class="size-[7px]" />
      <Link2 v-if="item.deviceMac" class="size-3.5 text-s-chip-fg" aria-label="已绑定设备，IP 变化时自动更新" />
    </div>
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
