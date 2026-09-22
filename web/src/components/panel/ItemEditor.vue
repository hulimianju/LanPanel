<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { Link2, Sparkles } from 'lucide-vue-next'
import type { Device, Group, Icon, Item } from '@/lib/types'
import { api } from '@/lib/api'
import { toast, toastError } from '@/lib/toast'
import { usePanel } from '@/stores/panel'
import { useApp } from '@/stores/app'
import { useDiscovery } from '@/stores/discovery'
import { hostIP, urlPort } from '@/lib/url'
import { deviceName } from '@/lib/devices'
import Dialog from '@/components/ui/Dialog.vue'
import Field from '@/components/ui/Field.vue'
import Input from '@/components/ui/Input.vue'
import Button from '@/components/ui/Button.vue'
import Segmented from '@/components/ui/Segmented.vue'
import IconPicker from './IconPicker.vue'

const open = defineModel<boolean>('open', { default: false })
const props = defineProps<{ item?: Item | null; groupId?: string; groups: Group[]; prefill?: Partial<Item> }>()

const panel = usePanel()
const app = useApp()
const disc = useDiscovery()

const form = reactive({
  title: '', urlLan: '', urlWan: '', desc: '', groupId: '',
  openMode: 'new' as 'new' | 'self',
  icon: { type: 'auto', value: '', color: '' } as Icon,
  deviceMac: '' as string | undefined,
  devicePort: 0 as number | undefined,
})
const saving = ref(false)
const fetching = ref(false)
const error = ref('')

watch(open, (v) => {
  if (!v) return
  const it = props.item
  error.value = ''
  Object.assign(form, {
    title: it?.title ?? '', urlLan: it?.urlLan ?? '', urlWan: it?.urlWan ?? '', desc: it?.desc ?? '',
    groupId: it?.groupId ?? props.groupId ?? props.groups[0]?.id ?? '',
    openMode: it?.openMode ?? 'new',
    icon: it ? { ...it.icon } : { type: 'auto', value: '', color: '' },
    deviceMac: it?.deviceMac ?? '',
    devicePort: it?.devicePort ?? 0,
  })
  // 从设备发现页「加入面板」时预填
  if (!it && props.prefill) {
    const p = props.prefill
    Object.assign(form, {
      title: p.title ?? '', urlLan: p.urlLan ?? '', urlWan: p.urlWan ?? '', desc: p.desc ?? '',
      icon: p.icon ? { ...p.icon } : form.icon, deviceMac: p.deviceMac ?? '', devicePort: p.devicePort ?? 0,
    })
    if (form.icon.type === 'auto' && !form.icon.value && fetchUrl.value) autoFetch()
  }
})

// ---- 绑定设备 ----
watch(open, (v) => {
  if (v && !disc.devices.length) disc.loadDevices().catch(() => {})
})
const boundDevice = computed(() => (form.deviceMac ? disc.devices.find((d) => d.mac === form.deviceMac) : undefined))
/** 地址中的 IP 对应一台已发现的设备时，建议绑定 */
const suggestion = computed(() => {
  if (form.deviceMac) return undefined
  const ip = hostIP(normalizeUrl(form.urlLan))
  return ip ? disc.devices.find((d) => d.ip === ip && d.mac) : undefined
})
const bindable = computed(() =>
  disc.devices.filter((d) => d.mac && !d.self).sort((a, b) => Number(b.online) - Number(a.online) || deviceName(a).localeCompare(deviceName(b))),
)
function bind(d: Device) {
  form.deviceMac = d.mac
  form.devicePort = urlPort(normalizeUrl(form.urlLan)) || 0
}
function bindByKey(e: Event) {
  const d = disc.devices.find((x) => x.key === (e.target as HTMLSelectElement).value)
  if (d) bind(d)
}
function unbind() {
  form.deviceMac = ''
  form.devicePort = 0
}

const fetchUrl = computed(() => (/^https?:\/\//i.test(form.urlLan) ? form.urlLan : /^https?:\/\//i.test(form.urlWan) ? form.urlWan : ''))

/** 抓取网站标题与图标；标题为空时才自动填充，不覆盖用户输入。 */
async function autoFetch() {
  if (!fetchUrl.value) return
  fetching.value = true
  try {
    const r = await api.post<{ title: string; icon: string | null }>('/api/icon/fetch', { url: fetchUrl.value })
    if (!form.title && r.title) form.title = r.title.slice(0, 48)
    if (r.icon) form.icon = { type: 'auto', value: r.icon, color: form.icon.color }
    else toast('未找到网站图标，将显示标题首字', 'info')
  } catch (e) {
    toastError(e)
  } finally {
    fetching.value = false
  }
}

function onUrlBlur() {
  // 新建且标题为空时，离开地址输入框自动抓取一次
  if (!props.item && !form.title && fetchUrl.value && !fetching.value) autoFetch()
}

function normalizeUrl(u: string) {
  const t = u.trim()
  if (!t) return ''
  // 只填了 IP/域名时补上 http://
  return /^[a-z][a-z0-9+.-]*:/i.test(t) ? t : `http://${t}`
}

async function save() {
  error.value = ''
  form.urlLan = normalizeUrl(form.urlLan)
  form.urlWan = normalizeUrl(form.urlWan)
  if (!form.title.trim()) return (error.value = '请填写标题')
  if (!form.urlLan && !form.urlWan) return (error.value = '内网地址和外网地址至少填写一个')
  saving.value = true
  try {
    // 面板还没有分组时自动创建一个
    if (!form.groupId || !panel.groups.some((g) => g.id === form.groupId)) {
      form.groupId = panel.groups[0]?.id ?? (await panel.addGroup('局域网服务')).id
    }
    const input = { ...form, icon: { ...form.icon } }
    if (props.item) await panel.updateItem(props.item.id, input)
    else await panel.addItem(input)
    toast(props.item ? '已保存' : '已添加')
    open.value = false
    panel.load().catch(() => {}) // 刷新卡片状态与可能被同步改写的地址
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <Dialog v-model:open="open" :title="item ? '编辑应用' : '添加应用'" width="560px">
    <form id="lp-item-form" class="flex flex-col gap-4" @submit.prevent="save">
      <Field label="内网地址" for="lp-url-lan" hint="局域网访问时使用；只填 IP:端口 会自动补全 http://">
        <div class="flex gap-2">
          <Input id="lp-url-lan" v-model="form.urlLan" mono placeholder="http://192.168.1.23:5666" @blur="onUrlBlur" />
          <Button :loading="fetching" :disabled="!fetchUrl" @click="autoFetch">
            <Sparkles v-if="!fetching" class="size-3.5" />自动获取
          </Button>
        </div>
      </Field>
      <Field label="外网地址（可选）" for="lp-url-wan" hint="通过公网域名或内网穿透访问时使用">
        <Input id="lp-url-wan" v-model="form.urlWan" mono placeholder="https://nas.example.com" />
      </Field>
      <!-- 绑定设备：IP 变化时自动更新地址 -->
      <div v-if="form.deviceMac" class="flex items-center gap-3 rounded-md border border-accent-soft bg-accent-soft/50 px-3 py-2.5">
        <Link2 class="size-4 shrink-0 text-accent" />
        <div class="flex min-w-0 grow flex-col">
          <span class="truncate text-[13px] font-medium">已绑定：{{ boundDevice ? deviceName(boundDevice) : form.deviceMac.toUpperCase() }}</span>
          <span class="truncate text-xs text-fg-2">
            {{ boundDevice ? `${boundDevice.ip} · ${boundDevice.mac?.toUpperCase()}` : '设备暂未出现在设备库中' }}{{ form.devicePort ? ` · 端口 ${form.devicePort}` : '' }} · 设备 IP 变化时自动更新地址
          </span>
        </div>
        <Button size="sm" variant="ghost" @click="unbind">解除</Button>
      </div>
      <div v-else-if="suggestion" class="flex items-center gap-3 rounded-md border border-accent-soft bg-accent-soft/50 px-3 py-2.5">
        <Link2 class="size-4 shrink-0 text-accent" />
        <span class="grow text-[13px] text-fg-2">
          地址中的 <span class="font-mono text-fg">{{ suggestion.ip }}</span> 是「<span class="text-fg">{{ deviceName(suggestion) }}</span>」，绑定后设备 IP 变化时会自动更新卡片地址
        </span>
        <Button size="sm" variant="primary" @click="bind(suggestion)">绑定</Button>
      </div>
      <details v-else-if="bindable.length" class="-mt-1 text-xs">
        <summary class="cursor-pointer text-fg-3 hover:text-fg-2">绑定到已发现的设备（可选，IP 变化时自动更新地址）</summary>
        <select class="mt-2 h-9 w-full rounded-sm border border-line bg-surface px-2 text-[13px] text-fg outline-none focus:border-accent" @change="bindByKey">
          <option value="">选择设备…</option>
          <option v-for="d in bindable" :key="d.key" :value="d.key">{{ deviceName(d) }} · {{ d.ip }}{{ d.online ? '' : '（离线）' }}</option>
        </select>
      </details>

      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Field label="标题" for="lp-title">
          <Input id="lp-title" v-model="form.title" placeholder="飞牛 fnOS" />
        </Field>
        <Field label="分组" for="lp-group">
          <select
            id="lp-group"
            v-model="form.groupId"
            class="h-9 rounded-sm border border-line bg-surface px-2.5 text-[13px] text-fg outline-none focus:border-accent"
          >
            <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option>
          </select>
        </Field>
      </div>
      <Field label="描述（可选）" for="lp-desc">
        <Input id="lp-desc" v-model="form.desc" placeholder="鼠标悬停时显示" />
      </Field>
      <Field label="图标">
        <IconPicker v-model="form.icon" :title="form.title" :tone="app.uiTheme" :fetching="fetching" :can-fetch="!!fetchUrl" @fetch="autoFetch" />
      </Field>
      <Field label="打开方式">
        <Segmented
          v-model="form.openMode"
          :options="[{ value: 'new', label: '新标签页' }, { value: 'self', label: '当前页' }]"
          label="打开方式"
        />
      </Field>
      <p v-if="error" class="m-0 rounded-sm bg-danger-soft px-3 py-2 text-[13px] text-danger-fg" role="alert">{{ error }}</p>
    </form>
    <template #footer>
      <Button @click="open = false">取消</Button>
      <Button variant="primary" type="submit" form="lp-item-form" :loading="saving">{{ item ? '保存' : '添加' }}</Button>
    </template>
  </Dialog>
</template>
