<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { DialogClose, DialogContent, DialogDescription, DialogOverlay, DialogPortal, DialogRoot, DialogTitle } from 'reka-ui'
import { Check, Copy, ExternalLink, Link2, Pencil, Plus, Trash2, X } from 'lucide-vue-next'
import { api } from '@/lib/api'
import type { Device, DeviceType, Item, Service } from '@/lib/types'
import { DEVICE_TYPES, deviceName, deviceType, fmtDate, fmtShort, relTime } from '@/lib/devices'
import { tintBg, tintColor } from '@/lib/tints'
import { ICONS } from '@/lib/icons'
import { useApp } from '@/stores/app'
import { usePanel } from '@/stores/panel'
import { useDiscovery } from '@/stores/discovery'
import { confirm } from '@/lib/confirm'
import { toast, toastError } from '@/lib/toast'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import ItemEditor from '@/components/panel/ItemEditor.vue'

const open = defineModel<boolean>('open', { default: false })
const props = defineProps<{ deviceKey: string | null }>()

const app = useApp()
const panel = usePanel()
const disc = useDiscovery()
const tone = computed(() => app.uiTheme)
const device = ref<Device | null>(null)
// 需在下方立即执行的 watch 之前声明
const editing = ref(false)
const draftName = ref('')
const draftType = ref<DeviceType>('')

watch(
  () => props.deviceKey,
  async (key) => {
    editing.value = false
    if (!key) return
    device.value = disc.devices.find((d) => d.key === key) ?? null
    try {
      device.value = await api.get<Device>(`/api/discovery/devices/${encodeURIComponent(key)}`)
      if (!device.value.acked) await disc.update(key, { acked: true })
    } catch (e) {
      toastError(e)
    }
    if (!panel.loaded) panel.load().catch(() => {})
  },
  { immediate: true },
)

const type = computed(() => (device.value ? DEVICE_TYPES[deviceType(device.value)] : DEVICE_TYPES['']))
const webServices = computed(() => device.value?.services.filter((s) => s.url && s.scheme !== 'tcp') ?? [])

const kv = computed(() => {
  const d = device.value
  if (!d) return []
  return [
    { k: 'IP 地址', v: d.ip, mono: true, copy: true },
    { k: 'MAC 地址', v: d.mac ? d.mac.toUpperCase() : '未知（跨网段）', mono: !!d.mac, copy: !!d.mac },
    { k: '厂商', v: d.randomized ? '随机 MAC（隐私地址）' : d.vendor || '未知' },
    { k: '系统 / 型号', v: [d.os, d.model].filter(Boolean).join(' · ') || '未识别' },
    { k: '主机名', v: d.hostname || '—', mono: !!d.hostname, copy: !!d.hostname },
    { k: '发现来源', v: d.sources.join(' · ') || '—' },
    { k: '首次发现', v: fmtDate(d.firstSeen), mono: true },
    { k: '最近在线', v: d.online ? `在线 · ${relTime(d.lastSeen)}` : `离线 · ${relTime(d.lastSeen)}` },
  ]
})

// ---- 编辑名称与类型 ----
function startEdit() {
  if (!device.value) return
  draftName.value = device.value.name ?? ''
  draftType.value = (device.value.userType ?? '') as DeviceType
  editing.value = true
}
async function saveEdit() {
  if (!device.value) return
  try {
    const d = await disc.update(device.value.key, { name: draftName.value.trim(), userType: draftType.value })
    device.value = { ...device.value, ...d }
    editing.value = false
  } catch (e) {
    toastError(e)
  }
}

async function forget() {
  if (!device.value) return
  const ok = await confirm({ title: `移除「${deviceName(device.value)}」？`, message: '设备记录与 IP 历史会被删除；下次扫描时如果它仍在线，会作为新设备重新出现。', confirmText: '移除', danger: true })
  if (!ok) return
  try {
    await disc.remove(device.value.key)
    open.value = false
  } catch (e) {
    toastError(e)
  }
}

// ---- 加入面板 ----
function panelItemFor(s: Service): Item | undefined {
  const mac = device.value?.mac
  for (const g of panel.groups)
    for (const it of g.items) {
      if (mac && it.deviceMac === mac && it.devicePort === s.port) return it
      if (s.url && it.urlLan === s.url) return it
    }
}
const boundCount = computed(() => {
  const mac = device.value?.mac
  if (!mac) return 0
  return panel.groups.reduce((n, g) => n + g.items.filter((it) => it.deviceMac === mac).length, 0)
})

const editorOpen = ref(false)
const prefill = ref<Partial<Item>>()
function addToPanel(s: Service) {
  const d = device.value!
  const title = s.ruleId && s.ruleId !== 'web-generic' ? s.name : s.title || `${deviceName(d)} :${s.port}`
  prefill.value = {
    title: title.slice(0, 48),
    urlLan: s.url,
    desc: `${deviceName(d)} · ${d.ip}:${s.port}`.slice(0, 120),
    icon: s.icon && ICONS[s.icon] ? { type: 'lucide', value: s.icon, color: s.color || '' } : { type: 'auto', value: '', color: '' },
    deviceMac: d.mac,
    devicePort: s.port,
  }
  editorOpen.value = true
}

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast('已复制 ' + text)
  } catch {
    toast(text, 'info')
  }
}

const typeOptions = Object.entries(DEVICE_TYPES).map(([k, v]) => ({ value: k as DeviceType, label: k ? v.label : '自动识别' }))
</script>

<template>
  <DialogRoot v-model:open="open">
    <DialogPortal>
      <DialogOverlay class="lp-overlay fixed inset-0 z-40 bg-[var(--overlay)]" />
      <DialogContent
        class="lp-sheet fixed inset-y-0 right-0 z-50 flex w-full max-w-[560px] flex-col border-l border-line bg-surface text-fg shadow-pop outline-none"
        :aria-describedby="undefined"
      >
        <template v-if="device">
          <div class="flex flex-col gap-4 border-b border-line px-6 pb-[18px] pt-5">
            <div class="flex items-start gap-3.5">
              <span class="flex size-12 shrink-0 items-center justify-center rounded-lg" :style="{ background: tintBg(type.tint, tone) }">
                <component :is="type.icon" class="size-6" :stroke-width="1.8" :color="tintColor(type.tint, tone)" />
              </span>
              <div class="flex min-w-0 grow flex-col gap-1.5">
                <DialogTitle class="m-0 truncate text-xl font-semibold tracking-[-0.01em]">{{ deviceName(device) }}</DialogTitle>
                <DialogDescription class="m-0 flex flex-wrap items-center gap-1.5">
                  <span
                    class="inline-flex h-[22px] items-center gap-1.5 rounded-xs px-2 text-xs font-medium"
                    :class="device.online ? 'bg-success-soft text-success-fg' : 'bg-surface-2 text-fg-2'"
                  >
                    <span class="size-1.5 rounded-full" :class="device.online ? 'bg-success' : 'bg-line-strong'" />{{ device.online ? '在线' : '离线' }}
                  </span>
                  <span v-if="device.ipHistory.length > 1" class="inline-flex h-[22px] items-center rounded-xs bg-warning-soft px-2 text-xs font-medium text-warning-fg">IP 变化过 {{ device.ipHistory.length - 1 }} 次</span>
                  <span class="text-xs text-fg-3">{{ type.label }}{{ device.userType ? '（手动指定）' : '' }}</span>
                </DialogDescription>
              </div>
              <DialogClose aria-label="关闭" class="lp-focus flex size-8 items-center justify-center rounded-sm text-fg-3 hover:bg-surface-2 hover:text-fg">
                <X class="size-4" />
              </DialogClose>
            </div>

            <form v-if="editing" class="flex flex-wrap items-end gap-2" @submit.prevent="saveEdit">
              <label class="flex min-w-40 grow flex-col gap-1 text-xs text-fg-2">名称
                <Input v-model="draftName" :placeholder="device.hostname || device.ip" />
              </label>
              <label class="flex flex-col gap-1 text-xs text-fg-2">类型
                <select v-model="draftType" class="h-9 rounded-sm border border-line bg-surface px-2 text-[13px] text-fg outline-none focus:border-accent">
                  <option v-for="o in typeOptions" :key="o.value" :value="o.value">{{ o.label }}</option>
                </select>
              </label>
              <Button type="submit" variant="primary"><Check class="size-3.5" />保存</Button>
              <Button @click="editing = false">取消</Button>
            </form>
            <div v-else class="flex flex-wrap gap-2">
              <a
                v-if="webServices[0]"
                :href="webServices[0].url"
                target="_blank"
                rel="noopener noreferrer"
                class="lp-focus inline-flex h-[34px] items-center gap-1.5 rounded-sm bg-accent px-3.5 text-[13px] font-medium text-accent-fg hover:bg-accent-hover"
              >
                <ExternalLink class="size-3.5" />打开 {{ webServices[0].name.length > 12 ? '管理页' : webServices[0].name }}
              </a>
              <Button @click="startEdit"><Pencil class="size-3.5" />编辑名称与类型</Button>
              <Button variant="ghost" icon-only aria-label="移除设备" @click="forget"><Trash2 class="size-4" /></Button>
            </div>
          </div>

          <div class="flex min-h-0 grow flex-col gap-6 overflow-y-auto px-6 py-5">
            <dl class="m-0 grid grid-cols-2 gap-x-5 gap-y-3.5">
              <div v-for="x in kv" :key="x.k" class="flex min-w-0 flex-col gap-0.5">
                <dt class="text-xs text-fg-3">{{ x.k }}</dt>
                <dd class="m-0 flex min-w-0 items-center gap-1 text-[13px]" :class="x.mono ? 'font-mono' : ''">
                  <span class="truncate" :title="x.v">{{ x.v }}</span>
                  <button v-if="x.copy" type="button" class="lp-focus shrink-0 rounded-xs p-0.5 text-fg-3 hover:text-fg" :aria-label="'复制' + x.k" @click="copy(x.v)">
                    <Copy class="size-3" />
                  </button>
                </dd>
              </div>
            </dl>

            <section class="flex flex-col gap-2">
              <div class="flex items-center">
                <h3 class="m-0 grow text-[13px] font-semibold">服务 <span class="font-normal text-fg-3">{{ device.services.length }}</span></h3>
                <span v-if="device.services.length" class="text-xs text-fg-3">{{ device.services.filter((s) => panelItemFor(s)).length }} 项已在面板</span>
              </div>
              <div v-if="device.services.length" class="overflow-hidden rounded-md border border-line">
                <div v-for="s in device.services" :key="s.port" class="flex min-h-12 items-center gap-2.5 border-b border-line/70 px-3 py-1.5 last:border-b-0">
                  <div class="flex w-[150px] shrink-0 flex-col">
                    <span class="truncate text-[13px] font-medium" :title="s.name">{{ s.name }}</span>
                    <span class="truncate text-[11px] text-fg-3">{{ s.ruleId && s.ruleId !== 'web-generic' ? '规则 ' + s.ruleId : { port: '端口扫描', mdns: 'mDNS 广播', ssdp: 'UPnP 声明', wsd: 'WS-Discovery' }[s.source] }}</span>
                  </div>
                  <a
                    v-if="s.url && s.scheme !== 'tcp'"
                    :href="s.url"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="min-w-0 grow truncate font-mono text-xs text-fg-2 hover:text-accent-soft-fg hover:underline"
                  >{{ s.url }}</a>
                  <span v-else class="min-w-0 grow truncate font-mono text-xs text-fg-2" :title="s.banner">{{ s.url || `${s.proto.toUpperCase()} ${s.port}` }}{{ s.banner ? '  ' + s.banner : '' }}</span>
                  <span v-if="panelItemFor(s)" class="flex shrink-0 items-center gap-1 text-xs text-success-fg">
                    <Check class="size-3.5" />已在面板
                  </span>
                  <Button v-else-if="s.url && s.scheme !== 'tcp'" size="sm" class="!h-[26px] !border-accent-soft !bg-accent-soft !px-2.5 !text-xs !text-accent-soft-fg" @click="addToPanel(s)">
                    <Plus class="size-3" />加入面板
                  </Button>
                </div>
              </div>
              <p v-else class="m-0 rounded-md border border-dashed border-line px-3 py-4 text-center text-xs text-fg-3">未发现开放的服务（完整扫描才会扫描端口）</p>
            </section>

            <section v-if="device.mac" class="flex items-center gap-3 rounded-md border border-accent-soft bg-accent-soft/50 p-3.5">
              <Link2 class="size-[18px] shrink-0 text-accent" />
              <div class="flex grow flex-col gap-0.5">
                <span class="text-[13px] font-medium">IP 自动跟随</span>
                <span class="text-xs text-fg-2">
                  {{ boundCount ? `${boundCount} 张面板卡片按 MAC 绑定此设备，IP 变化后自动改写地址` : '从上方「加入面板」添加的卡片会按 MAC 绑定此设备，IP 变化后自动改写地址' }}
                </span>
              </div>
            </section>

            <section class="flex flex-col gap-2.5">
              <h3 class="m-0 text-[13px] font-semibold">IP 历史</h3>
              <ol class="m-0 flex list-none flex-col p-0">
                <li v-for="(h, i) in [...device.ipHistory].reverse()" :key="h.from" class="flex gap-3">
                  <div class="flex w-3 flex-col items-center">
                    <span class="mt-1 size-[9px] rounded-full border-2" :class="i === 0 ? 'border-accent bg-accent' : 'border-line-strong bg-surface'" />
                    <span v-if="i < device.ipHistory.length - 1" class="w-px grow bg-line" />
                  </div>
                  <div class="flex grow items-baseline gap-2.5 pb-3.5">
                    <span class="font-mono text-[13px]" :class="i === 0 ? 'font-semibold' : ''">{{ h.ip }}</span>
                    <span class="text-xs text-fg-3">{{ h.to ? `${fmtShort(h.from)} – ${fmtShort(h.to)}` : `当前 · 自 ${fmtShort(h.from)}` }}</span>
                  </div>
                </li>
              </ol>
            </section>

            <section v-if="device.raw?.length" class="flex flex-col gap-2">
              <h3 class="m-0 text-[13px] font-semibold">原始发现数据</h3>
              <pre class="m-0 overflow-x-auto rounded-md bg-[#111114] px-3.5 py-3 font-mono text-[11.5px] leading-[1.7] text-[#D4D4D8]"><template v-for="(r, i) in device.raw" :key="i"><span class="text-[#8B84FF]">{{ r.source.padEnd(7, ' ') }}</span>{{ r.text }}
</template></pre>
            </section>
          </div>
        </template>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
  <ItemEditor v-model:open="editorOpen" :groups="panel.groups" :prefill="prefill" />
</template>

<style>
.lp-sheet[data-state='open'] { animation: lp-slide-in 220ms cubic-bezier(0.16, 1, 0.3, 1); }
@keyframes lp-slide-in { from { transform: translateX(24px); opacity: 0; } }
</style>
