<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ChevronDown, Search } from 'lucide-vue-next'
import { DropdownMenuContent, DropdownMenuItem, DropdownMenuPortal, DropdownMenuRoot, DropdownMenuTrigger } from 'reka-ui'
import type { Settings } from '@/lib/types'

const query = defineModel<string>({ default: '' })
const props = defineProps<{ engine: Settings['searchEngine']; customUrl?: string }>()
const emit = defineEmits<{ submit: [] }>()

const ENGINES = {
  bing: { name: 'Bing', url: 'https://www.bing.com/search?q=%s' },
  baidu: { name: '百度', url: 'https://www.baidu.com/s?wd=%s' },
  google: { name: 'Google', url: 'https://www.google.com/search?q=%s' },
  duckduckgo: { name: 'DuckDuckGo', url: 'https://duckduckgo.com/?q=%s' },
} as const
type EngineKey = keyof typeof ENGINES

/** 访客可临时切换搜索引擎（不保存到服务端）。 */
const current = ref<EngineKey | 'custom'>(props.engine)
const label = computed(() => (current.value === 'custom' ? '自定义' : ENGINES[current.value].name))
const input = ref<HTMLInputElement>()

function webSearch() {
  const q = query.value.trim()
  if (!q) return
  const tpl = current.value === 'custom' ? props.customUrl || ENGINES.bing.url : ENGINES[current.value].url
  window.open(tpl.replace('%s', encodeURIComponent(q)), '_blank', 'noopener')
}
defineExpose({ webSearch })

function onKey(e: KeyboardEvent) {
  const t = e.target as HTMLElement
  if (e.key === '/' && !['INPUT', 'TEXTAREA', 'SELECT'].includes(t.tagName) && !t.isContentEditable) {
    e.preventDefault()
    input.value?.focus()
  }
}
onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <form role="search" class="lp-glass-strong flex h-[52px] w-full max-w-[640px] items-center gap-3 rounded-[14px] pl-[18px] pr-2 transition-shadow focus-within:shadow-[0_0_0_3px_var(--focus)]" @submit.prevent="emit('submit')">
    <Search class="size-[18px] shrink-0 text-s-fg-3" />
    <input
      ref="input"
      v-model="query"
      type="search"
      placeholder="搜索应用，回车在网页中搜索"
      aria-label="搜索"
      class="h-full min-w-0 grow border-0 bg-transparent text-[15px] text-s-fg outline-none placeholder:text-s-fg-3"
      @keydown.esc="query = ''"
    />
    <kbd class="hidden h-6 items-center rounded-xs border border-s-border px-2 font-mono text-xs text-s-fg-3 sm:flex">/</kbd>
    <DropdownMenuRoot>
      <DropdownMenuTrigger class="lp-focus flex h-9 items-center gap-1.5 rounded-[9px] px-3 text-[13px] text-s-fg-2 hover:bg-s-glass-hover hover:text-s-fg" aria-label="选择搜索引擎">
        {{ label }}<ChevronDown class="size-3.5" />
      </DropdownMenuTrigger>
      <DropdownMenuPortal>
        <DropdownMenuContent align="end" :side-offset="8" class="z-50 min-w-36 rounded-md border border-line bg-surface p-1 text-[13px] text-fg shadow-pop">
          <DropdownMenuItem
            v-for="(e, k) in ENGINES"
            :key="k"
            class="flex h-8 cursor-pointer items-center rounded-xs px-2.5 outline-none data-[highlighted]:bg-surface-2"
            @select="current = k"
          >{{ e.name }}</DropdownMenuItem>
          <DropdownMenuItem
            v-if="customUrl"
            class="flex h-8 cursor-pointer items-center rounded-xs px-2.5 outline-none data-[highlighted]:bg-surface-2"
            @select="current = 'custom'"
          >自定义</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenuPortal>
    </DropdownMenuRoot>
  </form>
</template>

<style>
input[type='search']::-webkit-search-cancel-button { -webkit-appearance: none; }
</style>
