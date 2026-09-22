<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Download, FlaskConical, Plus, RotateCcw, Search, Trash2, Upload, X } from 'lucide-vue-next'
import { api } from '@/lib/api'
import type { Cond, DeviceType, Rule, TestResult, Tint } from '@/lib/types'
import { CATEGORY_LABELS, DEVICE_TYPES } from '@/lib/devices'
import { ICONS, ICON_NAMES } from '@/lib/icons'
import { TINT_NAMES, tintBg, tintColor } from '@/lib/tints'
import { useApp } from '@/stores/app'
import { confirm } from '@/lib/confirm'
import { toast, toastError } from '@/lib/toast'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Switch from '@/components/ui/Switch.vue'
import Segmented from '@/components/ui/Segmented.vue'

const app = useApp()
const tone = computed(() => app.uiTheme)
const rules = ref<Rule[]>([])
const portCount = ref(0)
const category = ref('all')
const query = ref('')

async function load() {
  const r = await api.get<{ rules: Rule[]; ports: number }>('/api/rules')
  rules.value = r.rules
  portCount.value = r.ports
}
onMounted(() => load().catch(toastError))

const cats = computed(() => [
  { key: 'all', label: '全部', count: rules.value.length },
  ...Object.entries(CATEGORY_LABELS)
    .map(([k, label]) => ({ key: k, label, count: rules.value.filter((r) => r.category === k).length }))
    .filter((c) => c.count > 0),
  { key: 'modified', label: '已修改 / 自定义', count: rules.value.filter((r) => r.modified || !r.builtin).length },
])
const visible = computed(() => {
  const q = query.value.trim().toLowerCase()
  return rules.value.filter((r) => {
    if (category.value === 'modified' && !(r.modified || !r.builtin)) return false
    if (category.value !== 'all' && category.value !== 'modified' && r.category !== category.value) return false
    return !q || r.name.toLowerCase().includes(q) || r.id.includes(q) || r.ports.some((p) => String(p).includes(q))
  })
})
const summary = computed(() => {
  const on = rules.value.filter((r) => !r.disabled).length
  const custom = rules.value.filter((r) => !r.builtin).length
  return `${rules.value.length} 条规则 · ${on} 条启用 · 自定义 ${custom} 条 · 扫描 ${portCount.value} 个端口`
})

function matchText(r: Rule) {
  if (!r.match?.length) return '端口开放即可'
  const opText: Record<string, string> = { contains: '~', equals: '=', prefix: '^', regex: '≈', exists: '存在' }
  return r.match.map((c) => (c.op === 'exists' ? `${c.field} 存在` : `${c.field} ${opText[c.op ?? 'contains']} "${c.value}"`)).join(r.all ? ' 且 ' : ' | ')
}

async function toggle(r: Rule, on: boolean) {
  try {
    const saved = await api.put<Rule>(`/api/rules/${r.id}`, { ...r, disabled: !on })
    Object.assign(r, saved)
    await load()
  } catch (e) {
    toastError(e)
  }
}

// ---- 编辑器 ----
type Draft = Omit<Rule, 'builtin' | 'modified'> & { portsText: string }
const selectedId = ref<string | null>(null)
const creating = ref(false)
const draft = ref<Draft | null>(null)
const saving = ref(false)
const current = computed(() => rules.value.find((r) => r.id === selectedId.value))

function toDraft(r: Rule): Draft {
  return JSON.parse(JSON.stringify({ ...r, match: r.match ?? [], portsText: r.ports.join(', ') }))
}
function select(r: Rule) {
  creating.value = false
  selectedId.value = r.id
  draft.value = toDraft(r)
  testResults.value = []
}
function newRule() {
  creating.value = true
  selectedId.value = null
  draft.value = {
    id: '', name: '', category: 'custom', ports: [], portsText: '', proto: 'web', match: [{ field: 'title', op: 'contains', value: '' }],
    icon: 'globe', color: 'blue', device: '', url: '',
  }
  testResults.value = []
}
function closeEditor() {
  draft.value = null
  selectedId.value = null
  creating.value = false
}

const FIELDS = [
  { value: 'title', label: '页面标题' }, { value: 'body', label: '页面内容' }, { value: 'server', label: '响应头 Server' },
  { value: 'header.', label: '其他响应头' }, { value: 'favicon', label: '图标哈希' }, { value: 'status', label: '状态码' },
  { value: 'banner', label: '欢迎信息 banner' },
]
const OPS = [
  { value: 'contains', label: '包含' }, { value: 'equals', label: '等于' }, { value: 'prefix', label: '开头是' },
  { value: 'regex', label: '正则' }, { value: 'exists', label: '存在' },
]
function fieldKind(c: Cond) {
  return c.field.startsWith('header.') ? 'header.' : c.field
}
function setField(c: Cond, v: string) {
  c.field = v === 'header.' ? 'header.X-Powered-By' : v
}
function addCond() {
  draft.value?.match?.push({ field: draft.value.proto === 'tcp' ? 'banner' : 'title', op: 'contains', value: '' })
}

function buildRule(d: Draft): Omit<Rule, 'builtin' | 'modified'> {
  const ports = d.portsText.split(/[,，\s]+/).map((x) => parseInt(x, 10)).filter((n) => Number.isFinite(n))
  const { portsText: _p, ...rest } = d
  return {
    ...rest,
    ports,
    match: (d.match ?? []).filter((c) => c.op === 'exists' || c.value),
    probe: d.proto === 'tcp' ? d.probe : undefined,
  }
}

async function save() {
  if (!draft.value) return
  saving.value = true
  try {
    const body = buildRule(draft.value)
    const r = creating.value ? await api.post<Rule>('/api/rules', body) : await api.put<Rule>(`/api/rules/${body.id}`, body)
    await load()
    toast(creating.value ? '规则已创建，下次扫描生效' : '规则已保存，下次扫描生效')
    select(rules.value.find((x) => x.id === r.id) ?? r)
  } catch (e) {
    toastError(e)
  } finally {
    saving.value = false
  }
}

async function removeOrReset() {
  const r = current.value
  if (!r) return
  const isCustom = !r.builtin
  const ok = await confirm({
    title: isCustom ? `删除规则「${r.name}」？` : `恢复「${r.name}」为默认？`,
    message: isCustom ? '删除后无法恢复。' : '你对这条内置规则的修改（包括启用状态）会被丢弃。',
    confirmText: isCustom ? '删除' : '恢复默认',
    danger: isCustom,
  })
  if (!ok) return
  try {
    await api.del(`/api/rules/${r.id}`)
    await load()
    if (isCustom) closeEditor()
    else select(rules.value.find((x) => x.id === r.id)!)
  } catch (e) {
    toastError(e)
  }
}

// ---- YAML 预览 ----
function yamlStr(v: string) {
  return /^[\w一-龥 .+/-]+$/.test(v) && !/^\d+$/.test(v) ? v : JSON.stringify(v)
}
const yamlPreview = computed(() => {
  if (!draft.value) return ''
  const r = buildRule(draft.value)
  const lines = [`- id: ${r.id || '<id>'}`, `  name: ${yamlStr(r.name || '<名称>')}`, `  category: ${r.category}`, `  ports: [${r.ports.join(', ')}]`, `  proto: ${r.proto}`]
  if (r.match?.length) {
    lines.push('  match:')
    for (const c of r.match) lines.push(`    - {field: ${c.field}${c.op && c.op !== 'contains' ? `, op: ${c.op}` : ''}${c.op === 'exists' ? '' : `, value: ${yamlStr(c.value ?? '')}`}}`)
  }
  if (r.all) lines.push('  all: true')
  if (r.probe) lines.push(`  probe: ${JSON.stringify(r.probe)}`)
  if (r.device) lines.push(`  device: ${r.device}`)
  if (r.system) lines.push('  system: true')
  if (r.icon) lines.push(`  icon: ${r.icon}`)
  if (r.color) lines.push(`  color: ${r.color}`)
  if (r.url) lines.push(`  url: ${yamlStr(r.url)}`)
  if (r.disabled) lines.push('  disabled: true')
  return lines.join('\n')
})

// ---- 测试 ----
const testIP = ref('')
const testing = ref(false)
const testResults = ref<TestResult[]>([])
async function runTest() {
  if (!draft.value) return
  testing.value = true
  testResults.value = []
  try {
    const rule = buildRule(draft.value)
    if (!rule.id) rule.id = 'draft-test'
    const r = await api.post<{ results: TestResult[] }>('/api/rules/test', { ip: testIP.value.trim(), rule })
    testResults.value = r.results
  } catch (e) {
    toastError(e)
  } finally {
    testing.value = false
  }
}

// ---- 导入 ----
async function onImport(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  ;(e.target as HTMLInputElement).value = ''
  if (!f) return
  try {
    const r = await api.post<{ total: number; changed: number }>('/api/rules/import', { yaml: await f.text() })
    await load()
    toast(`已导入 ${r.total} 条规则，其中 ${r.changed} 条有变化`)
  } catch (err) {
    toastError(err)
  }
}

const iconQuery = ref('')
const iconOpen = ref(false)
const iconList = computed(() => ICON_NAMES.filter((n) => n.includes(iconQuery.value.toLowerCase())))
const deviceOptions = Object.entries(DEVICE_TYPES).map(([k, v]) => ({ value: k as DeviceType, label: k ? v.label : '不推断' }))
</script>

<template>
  <div class="flex flex-col gap-5">
    <div class="flex flex-wrap items-end gap-2.5">
      <div class="flex min-w-0 grow flex-col gap-1">
        <h1 class="m-0 text-[22px] font-semibold tracking-[-0.01em]">端口规则</h1>
        <p class="m-0 text-[13px] text-fg-3">{{ summary }}</p>
      </div>
      <label class="lp-focus inline-flex h-9 cursor-pointer items-center gap-1.5 rounded-sm border border-line bg-surface px-3.5 text-[13px] font-medium hover:bg-surface-hover">
        <Upload class="size-3.5" />导入 YAML
        <input type="file" accept=".yaml,.yml,text/yaml" class="sr-only" @change="onImport" />
      </label>
      <a href="/api/rules/export" download class="lp-focus inline-flex h-9 items-center gap-1.5 rounded-sm border border-line bg-surface px-3.5 text-[13px] font-medium text-fg hover:bg-surface-hover">
        <Download class="size-3.5" />导出
      </a>
      <Button variant="primary" @click="newRule"><Plus class="size-3.5" />新建规则</Button>
    </div>

    <div class="grid gap-5" :class="draft ? 'xl:grid-cols-[180px_minmax(0,1fr)_380px]' : 'lg:grid-cols-[180px_minmax(0,1fr)]'">
      <nav aria-label="规则分类" class="flex gap-0.5 overflow-x-auto lg:flex-col">
        <button
          v-for="c in cats"
          :key="c.key"
          type="button"
          class="lp-focus flex h-[34px] shrink-0 items-center gap-3 rounded-sm px-2.5 text-left text-[13px]"
          :class="category === c.key ? 'bg-surface-2 font-medium text-fg' : 'text-fg-2 hover:bg-surface-hover'"
          @click="category = c.key"
        >
          <span class="grow whitespace-nowrap">{{ c.label }}</span><span class="text-xs tabular-nums text-fg-3">{{ c.count }}</span>
        </button>
      </nav>

      <div class="flex min-w-0 flex-col overflow-hidden rounded-lg border border-line bg-surface">
        <label class="flex h-11 items-center gap-2 border-b border-line px-4">
          <Search class="size-[15px] text-fg-3" />
          <input v-model="query" placeholder="按名称、ID 或端口筛选" class="min-w-0 grow border-0 bg-transparent text-[13px] text-fg outline-none placeholder:text-fg-3" />
        </label>
        <div class="overflow-x-auto">
          <div class="min-w-[640px]">
            <div class="grid h-[38px] grid-cols-[minmax(160px,1.2fr)_100px_64px_minmax(160px,1.6fr)_56px_40px] items-center gap-3 border-b border-line bg-bg-subtle px-4 text-xs font-medium text-fg-3">
              <span>名称</span><span>端口</span><span>协议</span><span>匹配条件</span><span>来源</span><span>启用</span>
            </div>
            <div
              v-for="r in visible"
              :key="r.id"
              class="grid min-h-[50px] cursor-pointer grid-cols-[minmax(160px,1.2fr)_100px_64px_minmax(160px,1.6fr)_56px_40px] items-center gap-3 border-b border-line/60 px-4 py-1.5 last:border-b-0 hover:bg-surface-hover"
              :class="[selectedId === r.id ? 'bg-accent-soft/50' : '', r.disabled ? 'opacity-55' : '']"
              @click="select(r)"
            >
              <div class="flex min-w-0 items-center gap-2.5">
                <span class="flex size-7 shrink-0 items-center justify-center rounded-[7px]" :style="{ background: tintBg((r.color || 'slate') as Tint, tone) }">
                  <component :is="ICONS[r.icon ?? ''] ?? ICONS.globe" :size="15" :stroke-width="1.8" :color="tintColor((r.color || 'slate') as Tint, tone)" />
                </span>
                <button type="button" class="lp-focus min-w-0 truncate text-left text-[13px] font-medium" @click.stop="select(r)">{{ r.name }}</button>
              </div>
              <span class="truncate font-mono text-xs">{{ r.ports.join(', ') || '任意' }}</span>
              <span class="text-xs uppercase text-fg-2">{{ r.proto }}</span>
              <span class="truncate font-mono text-[11.5px] text-fg-2" :title="matchText(r)">{{ matchText(r) }}</span>
              <span class="text-xs" :class="!r.builtin ? 'text-accent-soft-fg' : r.modified ? 'text-warning-fg' : 'text-fg-3'">{{ !r.builtin ? '自定义' : r.modified ? '已修改' : '内置' }}</span>
              <span @click.stop>
                <Switch :model-value="!r.disabled" :label="`启用 ${r.name}`" @update:model-value="toggle(r, $event)" />
              </span>
            </div>
            <p v-if="!visible.length" class="m-0 py-10 text-center text-[13px] text-fg-3">没有匹配的规则</p>
          </div>
        </div>
      </div>

      <!-- 编辑面板 -->
      <section v-if="draft" aria-label="编辑规则" class="flex h-fit flex-col gap-3.5 rounded-lg border border-line bg-surface p-[18px] xl:sticky xl:top-6">
        <div class="flex items-center gap-2">
          <h2 class="m-0 min-w-0 grow truncate text-sm font-semibold">{{ creating ? '新建规则' : `编辑规则 · ${current?.name}` }}</h2>
          <span v-if="!creating" class="rounded-[5px] bg-surface-2 px-1.5 text-[11px] leading-5 text-fg-2">{{ current?.builtin ? (current?.modified ? '内置 · 已修改' : '内置') : '自定义' }}</span>
          <button type="button" class="lp-focus flex size-7 items-center justify-center rounded-xs text-fg-3 hover:text-fg" aria-label="关闭编辑" @click="closeEditor"><X class="size-4" /></button>
        </div>

        <div class="grid grid-cols-2 gap-2.5">
          <label class="col-span-2 flex flex-col gap-1.5 text-xs font-medium text-fg-2">名称<Input v-model="draft.name" placeholder="例如：我的应用" /></label>
          <label v-if="creating" class="flex flex-col gap-1.5 text-xs font-medium text-fg-2">ID<Input v-model="draft.id" mono placeholder="my-app" /></label>
          <label class="flex flex-col gap-1.5 text-xs font-medium text-fg-2" :class="creating ? '' : 'col-span-1'">分类
            <select v-model="draft.category" class="h-9 rounded-sm border border-line bg-surface px-2 text-[13px] text-fg outline-none focus:border-accent">
              <option v-for="(label, k) in CATEGORY_LABELS" :key="k" :value="k">{{ label }}</option>
            </select>
          </label>
          <label class="flex flex-col gap-1.5 text-xs font-medium text-fg-2" :class="creating ? 'col-span-2' : ''">端口<Input v-model="draft.portsText" mono placeholder="8080, 8443" /></label>
        </div>

        <div class="flex flex-col gap-1.5">
          <span class="text-xs font-medium text-fg-2">协议</span>
          <Segmented v-model="draft.proto" block :options="[{ value: 'web', label: '自动' }, { value: 'http', label: 'HTTP' }, { value: 'https', label: 'HTTPS' }, { value: 'tcp', label: 'TCP' }]" label="协议" />
        </div>

        <div class="flex flex-col gap-1.5">
          <div class="flex items-center">
            <span class="grow text-xs font-medium text-fg-2">匹配条件（{{ draft.all ? '全部满足' : '任一满足' }}）</span>
            <label class="flex items-center gap-1.5 text-xs text-fg-3"><input v-model="draft.all" type="checkbox" class="accent-[var(--accent)]" />全部满足</label>
          </div>
          <div v-for="(c, i) in draft.match" :key="i" class="flex items-center gap-1.5">
            <select :value="fieldKind(c)" class="h-8 w-[104px] shrink-0 rounded-sm border border-line bg-surface px-1.5 text-xs text-fg outline-none" @change="setField(c, ($event.target as HTMLSelectElement).value)">
              <option v-for="f in FIELDS.filter((x) => (draft!.proto === 'tcp') === (x.value === 'banner'))" :key="f.value" :value="f.value">{{ f.label }}</option>
            </select>
            <input v-if="c.field.startsWith('header.')" v-model="c.field" class="h-8 w-28 min-w-0 rounded-sm border border-line bg-surface px-2 font-mono text-xs text-fg outline-none" />
            <select v-model="c.op" class="h-8 w-[70px] shrink-0 rounded-sm border border-line bg-surface px-1 text-xs text-fg outline-none">
              <option v-for="o in OPS" :key="o.value" :value="o.value">{{ o.label }}</option>
            </select>
            <input v-if="c.op !== 'exists'" v-model="c.value" class="h-8 min-w-0 grow rounded-sm border border-line bg-surface px-2 font-mono text-xs text-fg outline-none focus:border-accent" placeholder="值" />
            <span v-else class="grow" />
            <button type="button" class="lp-focus flex size-7 shrink-0 items-center justify-center rounded-xs text-fg-3 hover:text-danger" aria-label="删除条件" @click="draft.match!.splice(i, 1)"><X class="size-3.5" /></button>
          </div>
          <button type="button" class="self-start text-xs text-accent-soft-fg hover:underline" @click="addCond">+ 添加条件</button>
          <p v-if="!draft.match?.length" class="m-0 text-[11px] text-fg-3">没有条件时，端口开放即视为该服务（仅在本规则的端口上生效）</p>
        </div>

        <label v-if="draft.proto === 'tcp'" class="flex flex-col gap-1.5 text-xs font-medium text-fg-2">探测内容（可选）
          <Input v-model="draft.probe" mono placeholder="连接后发送，如 PING\r\n" />
        </label>

        <div class="grid grid-cols-2 gap-2.5">
          <label class="flex flex-col gap-1.5 text-xs font-medium text-fg-2">推断设备类型
            <select v-model="draft.device" class="h-9 rounded-sm border border-line bg-surface px-2 text-[13px] text-fg outline-none focus:border-accent">
              <option v-for="o in deviceOptions" :key="o.value" :value="o.value">{{ o.label }}</option>
            </select>
          </label>
          <label class="flex items-end gap-2 pb-2 text-xs text-fg-2">
            <input v-model="draft.system" type="checkbox" class="size-4 accent-[var(--accent)]" />作为设备系统显示
          </label>
        </div>

        <div class="flex flex-col gap-1.5">
          <span class="text-xs font-medium text-fg-2">图标与色调</span>
          <div class="flex items-center gap-2">
            <button type="button" class="lp-focus flex size-9 items-center justify-center rounded-md border border-line" :style="{ background: tintBg((draft.color || 'slate') as Tint, tone) }" aria-label="选择图标" @click="iconOpen = !iconOpen">
              <component :is="ICONS[draft.icon ?? ''] ?? ICONS.globe" :size="17" :stroke-width="1.8" :color="tintColor((draft.color || 'slate') as Tint, tone)" />
            </button>
            <button
              v-for="c in TINT_NAMES"
              :key="c"
              type="button"
              :aria-label="c"
              class="lp-focus size-5 rounded-full"
              :class="draft.color === c ? 'ring-2 ring-offset-2 ring-offset-surface' : ''"
              :style="{ background: tintColor(c, tone), '--tw-ring-color': tintColor(c, tone) }"
              @click="draft.color = c"
            />
          </div>
          <div v-if="iconOpen" class="flex flex-col gap-1.5 rounded-md border border-line p-2">
            <input v-model="iconQuery" placeholder="搜索图标" class="h-7 rounded-xs border border-line bg-surface px-2 text-xs outline-none" />
            <div class="grid max-h-32 grid-cols-9 gap-1 overflow-y-auto">
              <button v-for="n in iconList" :key="n" type="button" :title="n" :aria-label="n" class="lp-focus flex aspect-square items-center justify-center rounded-xs hover:bg-surface-2" :class="draft.icon === n ? 'bg-accent-soft' : ''" @click="draft.icon = n; iconOpen = false">
                <component :is="ICONS[n]" :size="15" :stroke-width="1.8" class="text-fg-2" />
              </button>
            </div>
          </div>
        </div>

        <label class="flex flex-col gap-1.5 text-xs font-medium text-fg-2">面板地址模板（可选）
          <Input v-model="draft.url" mono placeholder="{scheme}://{ip}:{port}" />
        </label>

        <pre class="m-0 overflow-x-auto rounded-sm bg-[#111114] px-3 py-2.5 font-mono text-[11px] leading-relaxed text-[#D4D4D8]">{{ yamlPreview }}</pre>

        <div class="flex flex-col gap-2">
          <form class="flex gap-2" @submit.prevent="runTest">
            <Input v-model="testIP" mono placeholder="输入 IP 测试，如 192.168.1.23" />
            <Button type="submit" :loading="testing" :disabled="!testIP"><FlaskConical v-if="!testing" class="size-3.5" />测试</Button>
          </form>
          <div v-if="testResults.length" class="flex flex-col gap-1 rounded-sm border border-line p-2 text-xs">
            <div v-for="t in testResults" :key="t.port" class="flex flex-col gap-0.5">
              <div class="flex items-center gap-2">
                <span class="font-mono">:{{ t.port }}</span>
                <span v-if="!t.open" class="text-fg-3">未开放</span>
                <span v-else-if="t.matched" class="font-medium text-success-fg">命中 {{ t.ruleName }}</span>
                <span v-else class="text-warning-fg">开放但未命中</span>
                <span v-if="t.open" class="ml-auto text-fg-3">{{ t.scheme?.toUpperCase() }} {{ t.status || '' }}</span>
              </div>
              <span v-if="t.open && (t.title || t.server || t.banner)" class="break-all font-mono text-[11px] text-fg-3">
                {{ t.title ? `title="${t.title}" ` : '' }}{{ t.server ? `server="${t.server}" ` : '' }}{{ t.banner ? `banner="${t.banner}" ` : '' }}{{ t.favicon ? `favicon=${t.favicon}` : '' }}
              </span>
            </div>
          </div>
        </div>

        <div class="flex gap-2 border-t border-line pt-3.5">
          <Button v-if="!creating && (current?.modified || !current?.builtin)" :variant="current?.builtin ? 'secondary' : 'danger'" @click="removeOrReset">
            <RotateCcw v-if="current?.builtin" class="size-3.5" /><Trash2 v-else class="size-3.5" />{{ current?.builtin ? '恢复默认' : '删除' }}
          </Button>
          <div class="grow" />
          <Button @click="closeEditor">取消</Button>
          <Button variant="primary" :loading="saving" @click="save">保存</Button>
        </div>
      </section>
    </div>
  </div>
</template>
