<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { api } from '@/lib/api'
import type { SystemInfo } from '@/lib/types'
import { bytes, duration } from '@/lib/format'
import { tintColor } from '@/lib/tints'

const props = defineProps<{ tone: 'light' | 'dark' }>()

const info = ref<SystemInfo | null>(null)
const cpuHist = ref<number[]>([])
const netHist = ref<number[]>([])
let timer: number | undefined

async function poll() {
  if (document.visibilityState !== 'visible') return
  try {
    const s = await api.get<SystemInfo>('/api/system')
    info.value = s
    if (s.cpuPercent >= 0) cpuHist.value = [...cpuHist.value, s.cpuPercent].slice(-24)
    netHist.value = [...netHist.value, s.netRx + s.netTx].slice(-24)
  } catch {
    /* 忽略，下次重试 */
  }
}
onMounted(() => {
  poll()
  timer = window.setInterval(poll, 3000)
  document.addEventListener('visibilitychange', poll)
})
onBeforeUnmount(() => {
  clearInterval(timer)
  document.removeEventListener('visibilitychange', poll)
})

function spark(values: number[], fixedMax?: number) {
  if (values.length < 2) return ''
  const max = fixedMax ?? Math.max(...values, 1)
  return values
    .map((v, i) => `${((i / (values.length - 1)) * 84).toFixed(1)},${(30 - (v / max) * 26).toFixed(1)}`)
    .join(' ')
}

const cards = computed(() => {
  const s = info.value
  if (!s) return []
  const hasCpu = s.cpuPercent >= 0
  return [
    {
      label: `本机 · ${s.hostname}`,
      value: hasCpu ? `CPU ${s.cpuPercent.toFixed(0)}%` : `${s.os}/${s.arch}`,
      sub: s.memTotal ? `内存 ${bytes(s.memUsed)} / ${bytes(s.memTotal)}` : '仅 Linux 提供 CPU 与内存数据',
      color: tintColor('blue', props.tone),
      points: spark(cpuHist.value, 100),
    },
    {
      label: '网络吞吐',
      value: `↓ ${bytes(s.netRx)}/s`,
      sub: `↑ ${bytes(s.netTx)}/s`,
      color: tintColor('teal', props.tone),
      points: spark(netHist.value),
    },
    { label: '运行时长', value: duration(s.uptime), sub: `${s.os} · ${s.arch}`, color: '', points: '' },
    { label: 'LanPanel', value: bytes(s.selfMem), sub: '本程序内存占用', color: '', points: '' },
  ]
})
</script>

<template>
  <div v-if="cards.length" class="grid grid-cols-2 gap-3 lg:grid-cols-4">
    <div v-for="c in cards" :key="c.label" class="lp-glass flex items-center gap-3.5 rounded-lg px-4 py-3.5">
      <div class="flex min-w-0 grow flex-col gap-1">
        <span class="truncate text-xs text-s-fg-3">{{ c.label }}</span>
        <span class="truncate text-lg font-semibold tabular-nums tracking-[-0.01em] text-s-fg sm:text-xl">{{ c.value }}</span>
        <span class="truncate text-xs text-s-fg-2">{{ c.sub }}</span>
      </div>
      <svg v-if="c.points" class="hidden shrink-0 sm:block" width="84" height="32" viewBox="0 0 84 32" fill="none" aria-hidden="true">
        <polyline :points="c.points" :stroke="c.color" stroke-width="1.6" stroke-linejoin="round" stroke-linecap="round" />
      </svg>
    </div>
  </div>
</template>
