<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Check, Info, Plus, RefreshCw, X, Zap } from 'lucide-vue-next'
import { api } from '@/lib/api'
import type { DiscoveryConfig, LogLine, ScanSummary } from '@/lib/types'
import { useDiscovery } from '@/stores/discovery'
import { fmtShort, relTime } from '@/lib/devices'
import { toast, toastError } from '@/lib/toast'
import Button from '@/components/ui/Button.vue'
import Switch from '@/components/ui/Switch.vue'
import Input from '@/components/ui/Input.vue'

const disc = useDiscovery()
const status = computed(() => disc.status)
const running = computed(() => !!status.value?.running)

// ---- 日志与进度轮询 ----
const logs = ref<LogLine[]>([])
const lastSeq = ref(0)
const logBox = ref<HTMLElement>()
const scans = ref<ScanSummary[]>([])
const follow = ref(true)

async function pollLogs() {
  const r = await api.get<{ logs: LogLine[] }>(`/api/discovery/logs?since=${lastSeq.value}`)
  if (!r.logs.length) return
  lastSeq.value = r.logs[r.logs.length - 1].seq
  // 过滤掉逐台 ARP 记录，保持日志可读
  logs.value = [...logs.value, ...r.logs.filter((l) => l.kind !== 'ARP')].slice(-300)
  if (follow.value) {
    await nextTick()
    logBox.value?.scrollTo({ top: logBox.value.scrollHeight })
  }
}
async function loadScans() {
  scans.value = (await api.get<{ scans: ScanSummary[] }>('/api/discovery/scans')).scans
}

let timer: number | undefined
async function tick() {
  const was = running.value
  try {
    await Promise.all([disc.loadStatus(), pollLogs()])
    if (was && !running.value) {
      await loadScans()
      disc.loadDevices().catch(() => {})
    }
  } catch {
    /* 下次重试 */
  }
  timer = window.setTimeout(tick, running.value ? 1000 : 5000)
}
onMounted(() => {
  tick()
  loadScans().catch(toastError)
  loadConfig()
})
onBeforeUnmount(() => clearTimeout(timer))

function onLogScroll() {
  const el = logBox.value
  if (el) follow.value = el.scrollHeight - el.scrollTop - el.clientHeight < 40
}

async function start(mode: 'full' | 'quick') {
  try {
    logs.value = []
    await disc.scan(mode)
    clearTimeout(timer)
    tick()
  } catch (e) {
    toastError(e)
  }
}
async function cancel() {
  await api.post('/api/discovery/cancel').catch(toastError)
}

const elapsed = ref('00:00')
let clock: number | undefined
watch(
  () => status.value?.running,
  (r) => {
    clearInterval(clock)
    if (!r) return
    const upd = () => {
      const s = Math.max(0, Math.floor((Date.now() - new Date(status.value!.start!).getTime()) / 1000))
      elapsed.value = `${String(Math.floor(s / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`
    }
    upd()
    clock = window.setInterval(upd, 1000)
  },
  { immediate: true },
)
onBeforeUnmount(() => clearInterval(clock))

const LOG_COLORS: Record<string, string> = {
  ARP: '#8FA8FF', mDNS: '#B4A1FF', SSDP: '#5ED3BA', PORT: '#F2BD62', HTTP: '#7BD88F', TCP: '#F2BD62',
  NBNS: '#F79AAB', WSD: '#5ED3BA', SNMP: '#F79AAB', DHCP: '#8FA8FF', ICMP: '#8FA8FF', INFO: '#D4D4D8', WARN: '#F5B83D',
}
const logTime = (iso: string) => new Date(iso).toTimeString().slice(0, 8)

// ---- 扫描设置 ----
const cfg = ref<DiscoveryConfig | null>(null)
const saved = ref('')
const meta = ref({ ruleCount: 0, ports: 0, lowResource: false })
const newSubnet = ref('')
const extraPorts = ref('')
const savingCfg = ref(false)

async function loadConfig() {
  const r = await api.get<{ config: DiscoveryConfig; ruleCount: number; ports: number; lowResource: boolean }>('/api/discovery/config')
  cfg.value = r.config
  saved.value = JSON.stringify(r.config)
  extraPorts.value = r.config.extraPorts.join(', ')
  meta.value = { ruleCount: r.ruleCount, ports: r.ports, lowResource: r.lowResource }
}
const cfgDirty = computed(() => {
  if (!cfg.value) return false
  return JSON.stringify({ ...cfg.value, extraPorts: parsePorts(extraPorts.value) }) !== saved.value
})
function parsePorts(s: string) {
  return s.split(/[,，\s]+/).map((x) => parseInt(x, 10)).filter((n) => Number.isFinite(n))
}
function addSubnet() {
  const v = newSubnet.value.trim()
  if (!v || !cfg.value) return
  if (!/^\d+\.\d+\.\d+\.\d+\/\d+$/.test(v)) return toast('网段格式应为 192.168.1.0/24', 'error')
  if (!cfg.value.subnets.includes(v)) cfg.value.subnets.push(v)
  newSubnet.value = ''
}
async function saveConfig() {
  if (!cfg.value) return
  savingCfg.value = true
  try {
    const r = await api.put<DiscoveryConfig>('/api/discovery/config', { ...cfg.value, extraPorts: parsePorts(extraPorts.value) })
    cfg.value = r
    saved.value = JSON.stringify(r)
    extraPorts.value = r.extraPorts.join(', ')
    toast('扫描设置已保存')
  } catch (e) {
    toastError(e)
  } finally {
    savingCfg.value = false
  }
}

const PROTOCOLS = [
  ['icmp', 'ICMP Ping'], ['mdns', 'mDNS / Bonjour'], ['ssdp', 'SSDP / UPnP'], ['netbios', 'NetBIOS'],
  ['wsd', 'WS-Discovery'], ['snmp', 'SNMP'], ['dns', '反向 DNS'],
] as const

const nextText = computed(() => {
  const n = status.value?.nextFull
  if (!n || !cfg.value?.fullInterval) return '定时完整扫描已关闭'
  const m = Math.round((new Date(n).getTime() - Date.now()) / 60000)
  return m <= 0 ? '即将开始下一次完整扫描' : `下次完整扫描约 ${m} 分钟后`
})
</script>

<template>
  <div class="flex flex-col gap-5">
    <div class="flex flex-wrap items-end gap-3">
      <div class="flex min-w-0 grow flex-col gap-1">
        <h1 class="m-0 text-[22px] font-semibold tracking-[-0.01em]">扫描任务</h1>
        <p class="m-0 text-[13px] text-fg-3">主动探测会产生少量网络流量{{ meta.lowResource ? '；已检测到小内存设备，默认并发已降低' : '' }}</p>
      </div>
      <template v-if="running">
        <Button variant="danger" @click="cancel"><X class="size-3.5" />取消扫描</Button>
      </template>
      <template v-else>
        <Button @click="start('quick')"><Zap class="size-3.5" />快速扫描</Button>
        <Button variant="primary" @click="start('full')"><RefreshCw class="size-3.5" />完整扫描</Button>
      </template>
    </div>

    <div class="grid gap-5 xl:grid-cols-[minmax(0,1fr)_360px]">
      <div class="flex min-w-0 flex-col gap-5">
        <!-- 当前任务 -->
        <section class="flex flex-col gap-4 rounded-lg border border-line bg-surface p-5">
          <div class="flex flex-wrap items-baseline gap-3">
            <h2 class="m-0 grow text-base font-semibold">
              <template v-if="running">
                正在{{ status?.mode === 'quick' ? '快速' : '' }}扫描 <span class="font-mono font-medium">{{ status?.subnets?.join('、') }}</span>
              </template>
              <template v-else>当前没有进行中的扫描</template>
            </h2>
            <span v-if="running" class="text-[13px] tabular-nums text-fg-3">已用时 {{ elapsed }} · 发现 {{ status?.found }} 台</span>
            <span v-else class="text-[13px] text-fg-3">{{ nextText }}</span>
            <span v-if="running" class="text-xl font-semibold tabular-nums">{{ Math.round(status?.percent ?? 0) }}%</span>
          </div>
          <div class="h-1.5 overflow-hidden rounded-full bg-surface-2">
            <div class="h-full rounded-full bg-accent transition-[width] duration-500" :style="{ width: (running ? status?.percent ?? 0 : 0) + '%' }" />
          </div>
          <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
            <div
              v-for="(p, i) in status?.phases ?? []"
              :key="p.key"
              class="flex flex-col gap-1.5 rounded-md border p-3"
              :class="p.status === 'run' && running ? 'border-accent/40 bg-accent-soft/40' : 'border-line'"
            >
              <div class="flex items-center gap-2">
                <span
                  class="flex size-5 items-center justify-center rounded-full text-[11px] font-semibold"
                  :class="{
                    'bg-success-soft text-success-fg': p.status === 'done',
                    'bg-accent text-accent-fg': p.status === 'run' && running,
                    'bg-surface-2 text-fg-3': p.status === 'wait' || p.status === 'skip' || (p.status === 'run' && !running),
                  }"
                >
                  <Check v-if="p.status === 'done'" class="size-3" /><template v-else>{{ i + 1 }}</template>
                </span>
                <span class="text-[13px] font-medium">{{ p.label }}</span>
              </div>
              <span class="text-xs text-fg-2">{{ p.desc }}</span>
              <span class="font-mono text-[11.5px] text-fg-3">
                {{ p.status === 'skip' ? '快速扫描跳过' : p.detail || (p.total ? `${p.done} / ${p.total}` : p.status === 'wait' ? '等待中' : '进行中') }}
              </span>
            </div>
          </div>
          <p v-if="status?.privilege" class="m-0 flex items-center gap-1.5 text-xs text-fg-3">
            <Info class="size-3.5" />主机发现方式：{{ status.privilege }}
          </p>
        </section>

        <!-- 实时日志 -->
        <section class="flex min-h-[420px] flex-col overflow-hidden rounded-lg bg-[#111114]">
          <div class="flex h-10 shrink-0 items-center gap-2.5 border-b border-[#26262B] px-4">
            <span class="size-[7px] rounded-full" :class="running ? 'animate-pulse bg-[#3DD68C]' : 'bg-[#52525B]'" />
            <span class="grow text-[13px] font-medium text-[#EDEDEF]">实时日志</span>
            <span class="text-xs text-[#8B8D98]">{{ logs.length }} 条{{ follow ? '' : ' · 已暂停自动滚动' }}</span>
          </div>
          <div ref="logBox" class="h-[420px] overflow-y-auto px-4 py-3 font-mono text-xs leading-[1.75]" @scroll="onLogScroll">
            <div v-for="l in logs" :key="l.seq" class="grid grid-cols-[62px_48px_minmax(0,170px)_minmax(0,1fr)] gap-3 text-[#D4D4D8]">
              <span class="text-[#6B6F7A]">{{ logTime(l.time) }}</span>
              <span :style="{ color: LOG_COLORS[l.kind] ?? '#A1A1AA' }">{{ l.kind }}</span>
              <span class="truncate" :title="l.target">{{ l.target }}</span>
              <span class="break-all text-[#A1A1AA]">{{ l.msg }}</span>
            </div>
            <p v-if="!logs.length" class="m-0 py-10 text-center text-[#6B6F7A]">暂无日志，开始扫描后会实时显示发现过程</p>
          </div>
        </section>
      </div>

      <aside class="flex flex-col gap-4">
        <!-- 扫描设置 -->
        <section v-if="cfg" class="flex flex-col gap-4 rounded-lg border border-line bg-surface p-[18px]">
          <h2 class="m-0 text-sm font-semibold">扫描设置</h2>
          <div class="flex flex-col gap-2">
            <span class="text-xs font-medium text-fg-2">网段</span>
            <label class="flex items-center gap-2 text-[13px]">
              <Switch v-model="cfg.autoSubnets" label="自动扫描本机所在网段" />自动扫描本机所在网段
            </label>
            <div class="flex flex-wrap gap-1.5">
              <span v-for="(n, i) in cfg.subnets" :key="n" class="flex h-7 items-center gap-1 rounded-sm bg-surface-2 pl-2.5 pr-1 font-mono text-xs">
                {{ n }}
                <button type="button" class="lp-focus flex size-5 items-center justify-center rounded-xs text-fg-3 hover:text-fg" :aria-label="'移除 ' + n" @click="cfg.subnets.splice(i, 1)"><X class="size-3" /></button>
              </span>
            </div>
            <form class="flex gap-1.5" @submit.prevent="addSubnet">
              <Input v-model="newSubnet" mono placeholder="10.0.10.0/24" />
              <Button type="submit" icon-only aria-label="添加网段"><Plus class="size-4" /></Button>
            </form>
            <span class="text-[11px] text-fg-3">额外网段不与本机直连时无法获取 MAC，只能按 IP 记录</span>
          </div>
          <div class="flex flex-col gap-1.5">
            <span class="text-xs font-medium text-fg-2">端口</span>
            <span class="text-[13px]">规则库 {{ meta.ruleCount }} 条规则 · {{ meta.ports }} 个端口 <RouterLink to="/admin/rules" class="text-accent-soft-fg hover:underline">管理</RouterLink></span>
            <Input v-model="extraPorts" mono placeholder="额外端口，如 8090, 9443" />
          </div>
          <div class="flex flex-col gap-2">
            <span class="text-xs font-medium text-fg-2">发现协议</span>
            <div class="grid grid-cols-2 gap-2">
              <label v-for="[k, label] in PROTOCOLS" :key="k" class="flex cursor-pointer items-center gap-2 text-[13px]">
                <input v-model="cfg.protocols[k]" type="checkbox" class="size-4 accent-[var(--accent)]" />{{ label }}
              </label>
            </div>
            <label v-if="cfg.protocols.snmp" class="flex items-center gap-2 text-xs text-fg-2">
              SNMP 团体名<Input v-model="cfg.snmpCommunity" mono class="!h-8 !w-32" />
            </label>
          </div>
          <div class="grid grid-cols-2 gap-3">
            <label class="flex flex-col gap-1.5 text-xs font-medium text-fg-2">并发数
              <input v-model.number="cfg.concurrency" type="number" min="8" max="1024" class="h-9 rounded-sm border border-line bg-surface px-3 font-mono text-[13px] text-fg outline-none focus:border-accent" />
            </label>
            <label class="flex flex-col gap-1.5 text-xs font-medium text-fg-2">超时（毫秒）
              <input v-model.number="cfg.timeoutMs" type="number" min="200" max="5000" step="100" class="h-9 rounded-sm border border-line bg-surface px-3 font-mono text-[13px] text-fg outline-none focus:border-accent" />
            </label>
            <label class="flex flex-col gap-1.5 text-xs font-medium text-fg-2">完整扫描间隔（分钟）
              <input v-model.number="cfg.fullInterval" type="number" min="0" class="h-9 rounded-sm border border-line bg-surface px-3 font-mono text-[13px] text-fg outline-none focus:border-accent" />
            </label>
            <label class="flex flex-col gap-1.5 text-xs font-medium text-fg-2">快速扫描间隔（分钟）
              <input v-model.number="cfg.quickInterval" type="number" min="0" class="h-9 rounded-sm border border-line bg-surface px-3 font-mono text-[13px] text-fg outline-none focus:border-accent" />
            </label>
          </div>
          <span class="-mt-2 text-[11px] text-fg-3">间隔为 0 表示关闭；快速扫描只做主机发现与协议广播，用于及时发现 IP 变化</span>
          <label class="flex items-center gap-2 text-[13px]">
            <Switch v-model="cfg.scanOnStart" label="启动时扫描" />程序启动时执行一次完整扫描
          </label>
          <Button variant="primary" :disabled="!cfgDirty" :loading="savingCfg" @click="saveConfig">保存设置</Button>
        </section>

        <!-- 最近扫描 -->
        <section class="flex flex-col gap-2.5 rounded-lg border border-line bg-surface p-[18px]">
          <h2 class="m-0 text-sm font-semibold">最近扫描</h2>
          <div v-for="s in scans.slice(0, 8)" :key="s.id" class="flex items-center gap-2.5 text-xs" :title="s.error || s.subnets.join('、')">
            <span class="size-[7px] shrink-0 rounded-full" :class="s.error ? 'bg-danger' : s.canceled ? 'bg-line-strong' : s.ipChanged ? 'bg-warning' : 'bg-success'" />
            <span class="w-[92px] shrink-0 font-mono text-fg-2">{{ fmtShort(s.end) }}</span>
            <span class="w-9 shrink-0 text-fg-3">{{ { manual: '手动', schedule: '定时', startup: '启动' }[s.trigger] }}</span>
            <span class="min-w-0 grow truncate text-fg-2">
              {{ s.error ? '失败' : s.canceled ? '已取消' : `${s.online} 台在线` }}{{ s.new ? ` · 新 ${s.new}` : '' }}{{ s.ipChanged ? ` · IP 变化 ${s.ipChanged}` : '' }}{{ s.mode === 'quick' ? ' · 快速' : '' }}
            </span>
            <span class="font-mono text-fg-3">{{ Math.max(1, Math.round((new Date(s.end).getTime() - new Date(s.start).getTime()) / 1000)) }}s</span>
          </div>
          <p v-if="!scans.length" class="m-0 text-xs text-fg-3">还没有扫描记录</p>
          <p v-else class="m-0 text-[11px] text-fg-3">最近一次：{{ relTime(scans[0].end) }}</p>
        </section>
      </aside>
    </div>
  </div>
</template>
