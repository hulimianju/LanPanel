<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ChevronRight, LayoutGrid, List, ListChecks, Radar, RefreshCw, Search } from 'lucide-vue-next'
import { useDiscovery } from '@/stores/discovery'
import { api } from '@/lib/api'
import type { Device, ScanSummary } from '@/lib/types'
import { DEVICE_TYPES, TYPE_FILTERS, deviceName, deviceSubtitle, deviceType, relTime } from '@/lib/devices'
import { tintBg, tintColor } from '@/lib/tints'
import { useApp } from '@/stores/app'
import { toastError } from '@/lib/toast'
import Button from '@/components/ui/Button.vue'
import Switch from '@/components/ui/Switch.vue'
import DeviceDrawer from '@/components/discovery/DeviceDrawer.vue'

const app = useApp()
const disc = useDiscovery()
const tone = computed(() => app.uiTheme)
const loading = ref(true)
const lastScan = ref<ScanSummary | null>(null)

async function refresh() {
  try {
    await Promise.all([disc.loadDevices(), disc.loadStatus()])
    const r = await api.get<{ scans: ScanSummary[] }>('/api/discovery/scans')
    lastScan.value = r.scans.find((s) => s.mode === 'full') ?? r.scans[0] ?? null
  } catch (e) {
    toastError(e)
  } finally {
    loading.value = false
  }
}

// 扫描进行中时轮询进度，结束后刷新列表
let timer: number | undefined
function schedule() {
  clearTimeout(timer)
  timer = window.setTimeout(async () => {
    const wasRunning = disc.status?.running
    try {
      const st = await disc.loadStatus()
      if (wasRunning && !st.running) await refresh()
    } catch {
      /* 忽略 */
    }
    schedule()
  }, disc.status?.running ? 1500 : 15000)
}
onMounted(async () => {
  await refresh()
  schedule()
})
onBeforeUnmount(() => clearTimeout(timer))

async function startScan() {
  try {
    await disc.scan('full')
    schedule()
  } catch (e) {
    toastError(e)
  }
}

// ---- 筛选 ----
const query = ref('')
const typeFilter = ref('all')
const onlineOnly = ref(false)
const view = ref<'table' | 'grid'>('table')

function matchesQuery(d: Device, q: string) {
  return [deviceName(d), d.ip, d.mac, d.vendor, d.hostname, d.model, d.os, ...d.services.map((s) => s.name)]
    .some((v) => v?.toLowerCase().includes(q))
}

const filters = computed(() => {
  const all = disc.devices.filter((d) => !onlineOnly.value || d.online)
  return [
    { key: 'all', label: '全部', count: all.length },
    ...TYPE_FILTERS.map((f) => ({ key: f.key, label: f.label, count: all.filter((d) => f.types.includes(deviceType(d))).length })),
  ].filter((f) => f.key === 'all' || f.count > 0)
})

const visible = computed(() => {
  const q = query.value.trim().toLowerCase()
  const types = TYPE_FILTERS.find((f) => f.key === typeFilter.value)?.types
  return disc.devices.filter(
    (d) => (!onlineOnly.value || d.online) && (!types || types.includes(deviceType(d))) && (!q || matchesQuery(d, q)),
  )
})
watch(filters, (fs) => {
  if (!fs.some((f) => f.key === typeFilter.value)) typeFilter.value = 'all'
})

const summary = computed(() => disc.summary)
const headerLine = computed(() => {
  const parts: string[] = []
  const nets = disc.status?.subnets?.length ? disc.status.subnets : lastScan.value?.subnets
  if (nets?.length) parts.push(nets.join('、'))
  if (lastScan.value) {
    const secs = Math.max(1, Math.round((new Date(lastScan.value.end).getTime() - new Date(lastScan.value.start).getTime()) / 1000))
    parts.push(`上次扫描 ${relTime(lastScan.value.end)}，耗时 ${secs} 秒`)
  }
  const next = disc.status?.nextFull
  if (next && new Date(next).getTime() > Date.now()) {
    parts.push(`下次 ${Math.max(1, Math.round((new Date(next).getTime() - Date.now()) / 60000))} 分钟后`)
  }
  return parts.join(' · ') || '尚未扫描'
})

function ipChanged(d: Device) {
  return d.ipHistory.length > 1 && Date.now() - new Date(d.ipHistory[d.ipHistory.length - 1].from).getTime() < 7 * 86400e3
}
function prevIP(d: Device) {
  const prev = d.ipHistory[d.ipHistory.length - 2]?.ip ?? ''
  const [a, b] = [prev.split('.'), d.ip.split('.')]
  return a.slice(0, 3).join('.') === b.slice(0, 3).join('.') ? '.' + a[3] : prev
}

// ---- 抽屉 ----
const selectedKey = ref<string | null>(null)
const drawerOpen = computed({
  get: () => !!selectedKey.value,
  set: (v) => {
    if (!v) selectedKey.value = null
  },
})
</script>

<template>
  <div class="flex flex-col gap-5">
    <div class="flex flex-wrap items-end gap-3">
      <div class="flex min-w-0 grow flex-col gap-1">
        <h1 class="m-0 text-[22px] font-semibold tracking-[-0.01em]">设备发现</h1>
        <p class="m-0 truncate text-[13px] text-fg-3">{{ headerLine }}</p>
      </div>
      <RouterLink to="/admin/rules" class="lp-focus inline-flex h-9 items-center gap-2 rounded-sm border border-line bg-surface px-3.5 text-[13px] font-medium text-fg hover:bg-surface-hover">
        <ListChecks class="size-[15px] text-fg-2" />端口规则
      </RouterLink>
      <RouterLink
        v-if="disc.status?.running"
        to="/admin/scan"
        class="lp-focus inline-flex h-9 items-center gap-2 rounded-sm bg-accent-soft px-3.5 text-[13px] font-medium text-accent-soft-fg"
      >
        <RefreshCw class="size-[15px] animate-spin" />扫描中 {{ Math.round(disc.status.percent) }}%
      </RouterLink>
      <Button v-else variant="primary" @click="startScan"><RefreshCw class="size-[15px]" />立即扫描</Button>
    </div>

    <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <div class="flex flex-col gap-1.5 rounded-lg border border-line bg-surface p-4">
        <span class="text-xs text-fg-3">在线设备</span>
        <div class="flex items-baseline gap-2">
          <span class="text-[26px] font-semibold tabular-nums tracking-[-0.02em]">{{ summary?.online ?? '-' }}</span>
          <span class="text-xs text-fg-3">共 {{ summary?.total ?? 0 }} 台已知</span>
        </div>
      </div>
      <div class="flex flex-col gap-1.5 rounded-lg border border-line bg-surface p-4">
        <span class="text-xs text-fg-3">新设备</span>
        <div class="flex items-baseline gap-2">
          <span class="text-[26px] font-semibold tabular-nums tracking-[-0.02em]">{{ summary?.new ?? '-' }}</span>
          <button v-if="summary?.new" type="button" class="text-xs text-accent-soft-fg hover:underline" @click="disc.ackAll().catch(toastError)">全部标为已读</button>
          <span v-else class="text-xs text-fg-3">无待确认</span>
        </div>
      </div>
      <div class="flex flex-col gap-1.5 rounded-lg border border-line bg-surface p-4">
        <span class="text-xs text-fg-3">Web 服务</span>
        <div class="flex items-baseline gap-2">
          <span class="text-[26px] font-semibold tabular-nums tracking-[-0.02em]">{{ summary?.web ?? '-' }}</span>
          <span class="text-xs text-fg-3">来自 {{ summary?.webHosts ?? 0 }} 台设备</span>
        </div>
      </div>
      <div class="flex flex-col gap-1.5 rounded-lg border border-line bg-surface p-4">
        <span class="text-xs text-fg-3">IP 变化（7 天）</span>
        <div class="flex items-baseline gap-2">
          <span class="text-[26px] font-semibold tabular-nums tracking-[-0.02em]">{{ summary?.ipChanged ?? '-' }}</span>
          <span class="text-xs text-fg-3">按 MAC 追踪</span>
        </div>
      </div>
    </div>

    <div class="flex flex-wrap items-center gap-3">
      <label class="flex h-9 w-full items-center gap-2 rounded-sm border border-line bg-surface px-3 sm:w-72">
        <Search class="size-[15px] shrink-0 text-fg-3" />
        <input v-model="query" placeholder="搜索名称、IP、MAC、厂商、服务" class="min-w-0 grow border-0 bg-transparent text-[13px] text-fg outline-none placeholder:text-fg-3" />
      </label>
      <div role="radiogroup" aria-label="设备类型" class="flex max-w-full gap-0.5 overflow-x-auto rounded-[9px] bg-surface-2 p-[3px]">
        <button
          v-for="f in filters"
          :key="f.key"
          type="button"
          role="radio"
          :aria-checked="typeFilter === f.key"
          class="lp-focus flex h-7 shrink-0 items-center gap-1.5 rounded-xs px-2.5 text-xs"
          :class="typeFilter === f.key ? 'bg-surface font-medium text-fg shadow-[0_1px_2px_rgba(0,0,0,0.08)]' : 'text-fg-2 hover:text-fg'"
          @click="typeFilter = f.key"
        >
          {{ f.label }}<span class="tabular-nums text-fg-3">{{ f.count }}</span>
        </button>
      </div>
      <div class="grow" />
      <label class="flex cursor-pointer items-center gap-2 text-[13px] text-fg-2">
        <Switch v-model="onlineOnly" label="仅显示在线设备" />仅在线
      </label>
      <div role="radiogroup" aria-label="视图" class="flex rounded-[9px] bg-surface-2 p-[3px]">
        <button type="button" role="radio" :aria-checked="view === 'table'" aria-label="列表视图" class="lp-focus flex h-7 w-8 items-center justify-center rounded-xs" :class="view === 'table' ? 'bg-surface text-fg' : 'text-fg-3'" @click="view = 'table'">
          <List class="size-[15px]" />
        </button>
        <button type="button" role="radio" :aria-checked="view === 'grid'" aria-label="卡片视图" class="lp-focus flex h-7 w-8 items-center justify-center rounded-xs" :class="view === 'grid' ? 'bg-surface text-fg' : 'text-fg-3'" @click="view = 'grid'">
          <LayoutGrid class="size-[15px]" />
        </button>
      </div>
    </div>

    <!-- 空状态 -->
    <div v-if="!loading && !disc.devices.length" class="flex flex-col items-center gap-3 rounded-lg border border-line bg-surface px-8 py-16 text-center">
      <Radar class="size-9 text-fg-3" />
      <p class="m-0 text-[15px] font-medium">{{ disc.status?.running ? '正在扫描局域网…' : '还没有发现设备' }}</p>
      <p class="m-0 max-w-md text-[13px] text-fg-3">通过 ARP、mDNS、SSDP、NetBIOS、SNMP 与端口指纹识别局域网里的设备和服务，首次扫描大约需要 10–40 秒。</p>
      <Button v-if="!disc.status?.running" variant="primary" @click="startScan"><RefreshCw class="size-4" />开始扫描</Button>
    </div>

    <!-- 列表视图 -->
    <div v-else-if="view === 'table'" class="overflow-hidden rounded-lg border border-line bg-surface">
      <div class="overflow-x-auto">
        <div class="min-w-[980px]">
          <div class="grid h-[38px] grid-cols-[minmax(220px,1.3fr)_140px_180px_140px_minmax(200px,2fr)_92px_24px] items-center gap-4 border-b border-line bg-bg-subtle px-4 text-xs font-medium text-fg-3">
            <span>设备</span><span>IP 地址</span><span>MAC / 厂商</span><span>发现来源</span><span>服务</span><span>最近在线</span><span />
          </div>
          <button
            v-for="d in visible"
            :key="d.key"
            type="button"
            class="lp-focus grid min-h-[58px] w-full grid-cols-[minmax(220px,1.3fr)_140px_180px_140px_minmax(200px,2fr)_92px_24px] items-center gap-4 border-b border-line/60 px-4 py-2 text-left transition-colors last:border-b-0 hover:bg-surface-hover"
            :class="[selectedKey === d.key ? 'bg-accent-soft/60' : '', d.online ? '' : 'opacity-60']"
            @click="selectedKey = d.key"
          >
            <div class="flex min-w-0 items-center gap-2.5">
              <span class="flex size-[34px] shrink-0 items-center justify-center rounded-[9px]" :style="{ background: tintBg(DEVICE_TYPES[deviceType(d)].tint, tone) }">
                <component :is="DEVICE_TYPES[deviceType(d)].icon" class="size-[18px]" :stroke-width="1.8" :color="tintColor(DEVICE_TYPES[deviceType(d)].tint, tone)" />
              </span>
              <div class="flex min-w-0 flex-col gap-0.5">
                <div class="flex min-w-0 items-center gap-1.5">
                  <span class="truncate text-sm font-medium">{{ deviceName(d) }}</span>
                  <span v-if="!d.acked && !d.self" class="rounded-[5px] bg-accent-soft px-1.5 text-[11px] font-semibold leading-[18px] text-accent-soft-fg">新</span>
                </div>
                <span class="truncate text-xs text-fg-3">{{ deviceSubtitle(d) }}</span>
              </div>
            </div>
            <div class="flex flex-col gap-0.5">
              <span class="font-mono text-[13px]">{{ d.ip }}</span>
              <span v-if="ipChanged(d)" class="text-[11px] text-warning-fg">IP 已变化 · 原 {{ prevIP(d) }}</span>
            </div>
            <div class="flex min-w-0 flex-col gap-0.5">
              <span class="truncate font-mono text-xs text-fg-2">{{ d.mac ? d.mac.toUpperCase() : '未知（跨网段）' }}</span>
              <span class="truncate text-xs text-fg-3">{{ d.randomized ? '随机 MAC（隐私地址）' : d.vendor || '未知厂商' }}</span>
            </div>
            <div class="flex flex-wrap gap-1">
              <span v-for="s in d.sources.slice(0, 4)" :key="s" class="rounded-[5px] border border-line bg-bg-subtle px-1.5 font-mono text-[10.5px] leading-[18px] text-fg-2">{{ s }}</span>
            </div>
            <div class="flex min-w-0 flex-wrap gap-1.5">
              <span
                v-for="s in d.services.slice(0, 4)"
                :key="s.port"
                class="flex h-6 max-w-[180px] items-center gap-1 rounded-xs px-2 text-xs"
                :class="s.scheme === 'tcp' ? 'bg-surface-2 text-fg-2' : 'bg-accent-soft text-accent-soft-fg'"
              >
                <span class="truncate">{{ s.name }}</span><span class="font-mono text-[11px] opacity-75">:{{ s.port }}</span>
              </span>
              <span v-if="d.services.length > 4" class="flex h-6 items-center rounded-xs bg-surface-2 px-2 text-xs text-fg-3">+{{ d.services.length - 4 }}</span>
              <span v-if="!d.services.length" class="text-xs text-fg-3">—</span>
            </div>
            <div class="flex items-center gap-1.5 text-xs text-fg-2">
              <span class="size-[7px] shrink-0 rounded-full" :class="d.online ? 'bg-success' : 'bg-line-strong'" />{{ d.online ? relTime(d.lastSeen) : relTime(d.lastSeen) }}
            </div>
            <ChevronRight class="size-4 text-fg-3" />
          </button>
          <p v-if="!visible.length" class="m-0 px-4 py-10 text-center text-[13px] text-fg-3">没有符合条件的设备</p>
        </div>
      </div>
      <div class="flex h-11 items-center border-t border-line px-4 text-xs text-fg-3">
        <span class="grow">显示 {{ visible.length }} / {{ disc.devices.length }} 台设备</span>
        <span class="hidden sm:inline">{{ disc.status?.privilege ? '主机发现：' + disc.status.privilege : '' }}</span>
      </div>
    </div>

    <!-- 卡片视图 -->
    <div v-else class="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-3">
      <button
        v-for="d in visible"
        :key="d.key"
        type="button"
        class="lp-focus flex flex-col gap-3 rounded-lg border border-line bg-surface p-4 text-left transition-colors hover:bg-surface-hover"
        :class="d.online ? '' : 'opacity-60'"
        @click="selectedKey = d.key"
      >
        <div class="flex items-center gap-3">
          <span class="flex size-10 shrink-0 items-center justify-center rounded-[10px]" :style="{ background: tintBg(DEVICE_TYPES[deviceType(d)].tint, tone) }">
            <component :is="DEVICE_TYPES[deviceType(d)].icon" class="size-5" :stroke-width="1.8" :color="tintColor(DEVICE_TYPES[deviceType(d)].tint, tone)" />
          </span>
          <div class="flex min-w-0 grow flex-col">
            <span class="truncate text-sm font-medium">{{ deviceName(d) }}</span>
            <span class="truncate text-xs text-fg-3">{{ deviceSubtitle(d) }}</span>
          </div>
          <span class="size-2 shrink-0 rounded-full" :class="d.online ? 'bg-success' : 'bg-line-strong'" />
        </div>
        <div class="flex items-center justify-between font-mono text-xs text-fg-2">
          <span>{{ d.ip }}</span><span class="text-fg-3">{{ d.mac?.toUpperCase() }}</span>
        </div>
        <div class="flex flex-wrap gap-1.5">
          <span v-for="s in d.services.filter((x) => x.scheme !== 'tcp').slice(0, 3)" :key="s.port" class="rounded-xs bg-accent-soft px-2 text-xs leading-6 text-accent-soft-fg">{{ s.name }}</span>
          <span v-if="!d.services.some((x) => x.scheme !== 'tcp')" class="text-xs text-fg-3">{{ d.services.length ? `${d.services.length} 个非网页服务` : '未发现服务' }}</span>
        </div>
      </button>
    </div>

    <DeviceDrawer v-model:open="drawerOpen" :device-key="selectedKey" />
  </div>
</template>
