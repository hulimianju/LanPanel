<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import { CircleAlert, CircleCheck, Download, ImagePlus, Upload } from 'lucide-vue-next'
import { useApp } from '@/stores/app'
import { api } from '@/lib/api'
import type { Settings } from '@/lib/types'
import { resolveTone } from '@/lib/surface'
import { assessReadability, hexLuminance, sampleImageLuminance } from '@/lib/luminance'
import { toast, toastError } from '@/lib/toast'
import { confirm } from '@/lib/confirm'
import Field from '@/components/ui/Field.vue'
import Input from '@/components/ui/Input.vue'
import Button from '@/components/ui/Button.vue'
import Switch from '@/components/ui/Switch.vue'
import Segmented from '@/components/ui/Segmented.vue'
import Slider from '@/components/ui/Slider.vue'
import { tintBg, tintColor } from '@/lib/tints'

const app = useApp()
const clone = (s: Settings): Settings => JSON.parse(JSON.stringify(s))
const draft = ref<Settings>(clone(app.settings!))
const dirty = computed(() => JSON.stringify(draft.value) !== JSON.stringify(app.settings))
const saving = ref(false)

async function save() {
  saving.value = true
  try {
    await app.saveSettings(draft.value)
    draft.value = clone(app.settings!)
    toast('设置已保存')
  } catch (e) {
    toastError(e)
  } finally {
    saving.value = false
  }
}
function reset() {
  draft.value = clone(app.settings!)
}

onBeforeRouteLeave(async () => {
  if (!dirty.value) return true
  return confirm({ title: '放弃未保存的修改？', confirmText: '放弃', danger: true })
})

// ---- 壁纸 ----
const wp = computed(() => draft.value.wallpaper)
const uploading = ref(false)
const fileInput = ref<HTMLInputElement>()

/** 当前壁纸亮度：纯色直接算，图片用已保存值（上传时会重新采样）。 */
const luminance = computed(() => {
  if (wp.value.type === 'color') return hexLuminance(wp.value.value)
  if (wp.value.type === 'image') return wp.value.luminance >= 0 ? wp.value.luminance : app.sampledLum ?? -1
  return -1
})

const previewTone = computed(() => resolveTone(draft.value, app.uiTheme, luminance.value >= 0 ? luminance.value : undefined))

/** 仅图片壁纸需要可读性评估：裸露在壁纸上的时钟、分组标题依赖遮罩提供对比度。 */
const readability = computed(() => {
  if (wp.value.type !== 'image' || luminance.value < 0) return null
  return assessReadability(luminance.value, previewTone.value, wp.value.dim)
})

watch(
  () => wp.value.type,
  (t, old) => {
    if (t === old) return
    if (t === 'color' && !/^#[0-9a-f]{6}$/i.test(wp.value.value)) wp.value.value = '#1E2130'
    if (t === 'image' && !wp.value.value.startsWith('/uploads/') && !/^https?:/.test(wp.value.value)) wp.value.value = ''
    if (t !== 'image') wp.value.luminance = -1
  },
)

async function onWallpaperFile(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  ;(e.target as HTMLInputElement).value = ''
  if (!f) return
  uploading.value = true
  try {
    const r = await api.upload(f, 'wallpaper')
    const lum = await sampleImageLuminance(r.url).catch(() => -1)
    Object.assign(draft.value.wallpaper, { type: 'image', value: r.url, luminance: lum })
    // 根据亮度给出一个合理的初始遮罩
    if (lum >= 0) {
      const tone = resolveTone(draft.value, app.uiTheme, lum)
      const a = assessReadability(lum, tone, draft.value.wallpaper.dim)
      if (!a.ok) draft.value.wallpaper.dim = a.recommended
    }
  } catch (err) {
    toastError(err)
  } finally {
    uploading.value = false
  }
}

const toneLabel = computed(() => (previewTone.value === 'dark' ? '深色调（浅色文字）' : '浅色调（深色文字）'))

const previewBg = computed(() => {
  if (wp.value.type === 'image' && wp.value.value)
    return {
      backgroundImage: `url("${wp.value.value}")`,
      filter: wp.value.blur ? `blur(${wp.value.blur / 3}px)` : undefined,
      transform: wp.value.blur ? 'scale(1.1)' : undefined,
    }
  if (wp.value.type === 'color') return { backgroundColor: wp.value.value }
  return {}
})

// ---- 账号 ----
const pw = ref({ old: '', new: '', confirm: '' })
const pwSaving = ref(false)
async function changePassword() {
  if (pw.value.new.length < 6) return toast('新密码至少 6 位', 'error')
  if (pw.value.new !== pw.value.confirm) return toast('两次输入的新密码不一致', 'error')
  pwSaving.value = true
  try {
    await api.put('/api/account/password', { old: pw.value.old, new: pw.value.new })
    pw.value = { old: '', new: '', confirm: '' }
    toast('密码已修改，其他设备需要重新登录')
  } catch (e) {
    toastError(e)
  } finally {
    pwSaving.value = false
  }
}

// ---- 备份 ----
const importMode = ref<'merge' | 'replace'>('merge')
async function onImport(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  ;(e.target as HTMLInputElement).value = ''
  if (!f) return
  try {
    const data = JSON.parse(await f.text())
    if (importMode.value === 'replace') {
      const ok = await confirm({ title: '覆盖当前面板？', message: '现有的分组、应用和外观设置将被导入文件替换。', confirmText: '覆盖导入', danger: true })
      if (!ok) return
    }
    const r = await api.post<{ groups: number; items: number }>('/api/import', { ...data, mode: importMode.value })
    toast(`已导入 ${r.groups} 个分组、${r.items} 个应用`)
    await app.load()
    draft.value = clone(app.settings!)
  } catch (err) {
    toastError(err instanceof SyntaxError ? new Error('文件不是有效的 JSON') : err)
  }
}

const themeOptions = [
  { value: 'auto', label: '跟随系统' },
  { value: 'light', label: '浅色' },
  { value: 'dark', label: '深色' },
] as const
</script>

<template>
  <div class="flex flex-col gap-6">
    <div class="flex flex-col gap-1">
      <h1 class="m-0 text-[22px] font-semibold tracking-[-0.01em]">设置</h1>
      <p class="m-0 text-[13px] text-fg-3">外观、面板行为、账号与备份</p>
    </div>

    <!-- 外观 -->
    <section class="rounded-lg border border-line bg-surface">
      <h2 class="m-0 border-b border-line px-5 py-3.5 text-sm font-semibold">外观</h2>
      <div class="grid gap-6 p-5 lg:grid-cols-[minmax(0,1fr)_380px]">
        <div class="flex flex-col gap-5">
          <Field label="界面主题" hint="作用于管理页面与弹窗；面板首页的色调由壁纸决定">
            <Segmented v-model="draft.theme" :options="[...themeOptions]" label="界面主题" />
          </Field>

          <Field label="壁纸">
            <div class="flex flex-col gap-3">
              <Segmented
                v-model="draft.wallpaper.type"
                :options="[{ value: 'none', label: '点阵（默认）' }, { value: 'color', label: '纯色' }, { value: 'image', label: '图片' }]"
                label="壁纸类型"
              />
              <div v-if="wp.type === 'color'" class="flex items-center gap-2">
                <input v-model="draft.wallpaper.value" type="color" aria-label="壁纸颜色" class="h-9 w-12 cursor-pointer rounded-sm border border-line bg-surface p-1" />
                <Input v-model="draft.wallpaper.value" mono class="!w-32" />
              </div>
              <div v-if="wp.type === 'image'" class="flex items-center gap-3">
                <div class="h-14 w-24 shrink-0 overflow-hidden rounded-sm border border-line bg-surface-2">
                  <img v-if="wp.value" :src="wp.value" alt="当前壁纸" class="size-full object-cover" />
                </div>
                <div class="flex flex-col gap-1.5">
                  <Button size="sm" :loading="uploading" @click="fileInput?.click()">
                    <ImagePlus v-if="!uploading" class="size-3.5" />{{ wp.value ? '更换图片' : '上传图片' }}
                  </Button>
                  <span class="text-xs text-fg-3">JPG / PNG / WebP，≤ 12 MB；建议 2560×1440 以上</span>
                </div>
                <input ref="fileInput" type="file" accept="image/jpeg,image/png,image/webp,image/gif" class="sr-only" @change="onWallpaperFile" />
              </div>
            </div>
          </Field>

          <Field
            v-if="wp.type !== 'none'"
            label="面板色调"
            :hint="luminance >= 0 ? `壁纸亮度 ${(luminance * 100).toFixed(0)}% · 当前为${toneLabel}` : `当前为${toneLabel}`"
          >
            <Segmented
              v-model="draft.wallpaper.tone"
              :options="[{ value: 'auto', label: '按壁纸自动' }, { value: 'dark', label: '深色调' }, { value: 'light', label: '浅色调' }]"
              label="面板色调"
            />
          </Field>

          <template v-if="wp.type === 'image'">
            <Field :label="`模糊 ${wp.blur}px`" hint="细节丰富的照片适当模糊，卡片与文字会更清晰">
              <Slider v-model="draft.wallpaper.blur" :min="0" :max="40" label="模糊程度" />
            </Field>
            <Field :label="`遮罩 ${wp.dim}%`" :hint="`在壁纸上叠加一层${previewTone === 'dark' ? '黑色' : '白色'}，提高文字对比度`">
              <Slider v-model="draft.wallpaper.dim" :min="0" :max="80" label="遮罩强度" />
            </Field>
            <div
              v-if="readability"
              class="flex items-center gap-2.5 rounded-md px-3 py-2.5 text-[13px]"
              :class="readability.ok ? 'bg-success-soft text-success-fg' : 'bg-warning-soft text-warning-fg'"
              role="status"
            >
              <CircleCheck v-if="readability.ok" class="size-4 shrink-0" />
              <CircleAlert v-else class="size-4 shrink-0" />
              <span class="grow">
                文字对比度 {{ readability.ratio.toFixed(1) }}:1
                {{ readability.ok ? '，清晰可读' : `，低于 4.5:1，时钟和分组标题可能看不清` }}
              </span>
              <Button v-if="!readability.ok" size="sm" @click="draft.wallpaper.dim = readability.recommended">调到 {{ readability.recommended }}%</Button>
            </div>
          </template>

          <Field label="卡片尺寸">
            <Segmented v-model="draft.cardSize" :options="[{ value: 'comfortable', label: '标准' }, { value: 'compact', label: '紧凑' }]" label="卡片尺寸" />
          </Field>
        </div>

        <!-- 实时预览：与首页使用同一套表面变量 -->
        <div class="flex flex-col gap-2">
          <span class="text-xs font-medium text-fg-2">预览</span>
          <div
            class="lp-surface relative isolate aspect-[16/10] overflow-hidden rounded-lg border border-line"
            :data-tone="previewTone"
            :data-wp="wp.type === 'image' ? 'image' : 'plain'"
            style="--s-blur: 10px"
          >
            <div class="absolute inset-0 -z-10 overflow-hidden">
              <div v-if="wp.type === 'none'" class="lp-dots absolute inset-0" />
              <div v-else class="absolute inset-0 bg-cover bg-center" :style="previewBg" />
              <div
                v-if="wp.type === 'image'"
                class="absolute inset-0"
                :style="{ background: previewTone === 'dark' ? `rgba(0,0,0,${wp.dim / 100})` : `rgba(255,255,255,${wp.dim / 100})` }"
              />
            </div>
            <div class="flex h-full flex-col items-center gap-2.5 px-5 pt-6">
              <span class="lp-text-shadow text-[38px] font-light leading-none tracking-[-0.04em] text-s-fg">14:32</span>
              <span class="lp-text-shadow text-[10px] text-s-fg-2">9月22日 星期二</span>
              <div class="lp-glass-strong h-7 w-3/4 rounded-md" />
              <div class="lp-text-shadow mt-1 w-full text-left text-[10px] font-semibold text-s-fg">常用</div>
              <div class="grid w-full grid-cols-2 gap-2">
                <div v-for="(t, i) in [['飞牛 fnOS', 'blue'], ['Jellyfin', 'violet']] as const" :key="i" class="lp-glass flex items-center gap-2 rounded-md px-2 py-2">
                  <span class="size-6 shrink-0 rounded-[6px]" :style="{ background: tintBg(t[1], previewTone), border: `1.5px solid ${tintColor(t[1], previewTone)}` }" />
                  <div class="flex min-w-0 flex-col">
                    <span class="truncate text-[11px] font-medium text-s-fg">{{ t[0] }}</span>
                    <span class="truncate font-mono text-[9px] text-s-fg-3">192.168.1.23</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- 面板 -->
    <section class="rounded-lg border border-line bg-surface">
      <h2 class="m-0 border-b border-line px-5 py-3.5 text-sm font-semibold">面板</h2>
      <div class="grid gap-5 p-5 md:grid-cols-2">
        <Field label="站点标题" for="lp-site-title">
          <Input id="lp-site-title" v-model="draft.siteTitle" />
        </Field>
        <Field label="默认地址模式" hint="自动：按访问者 IP 判断是否在局域网内">
          <Segmented
            v-model="draft.addressMode"
            :options="[{ value: 'auto', label: '自动' }, { value: 'lan', label: '内网' }, { value: 'wan', label: '外网' }]"
            label="默认地址模式"
          />
        </Field>
        <Field label="搜索引擎" for="lp-engine">
          <select id="lp-engine" v-model="draft.searchEngine" class="h-9 rounded-sm border border-line bg-surface px-2.5 text-[13px] text-fg outline-none focus:border-accent">
            <option value="bing">Bing</option>
            <option value="baidu">百度</option>
            <option value="google">Google</option>
            <option value="duckduckgo">DuckDuckGo</option>
            <option value="custom">自定义</option>
          </select>
        </Field>
        <Field v-if="draft.searchEngine === 'custom'" label="自定义搜索地址" for="lp-custom" hint="用 %s 表示关键词">
          <Input id="lp-custom" v-model="draft.customSearchUrl" mono placeholder="https://search.example.com/?q=%s" />
        </Field>
        <div class="flex flex-col gap-3 md:col-span-2">
          <label v-for="opt in ([
            ['showClock', '显示时钟', ''],
            ['showSeconds', '时钟显示秒', ''],
            ['showWidgets', '显示状态卡片', '本机 CPU、内存、网速（仅登录后可见）'],
            ['publicPanel', '访客模式', '未登录访客也可浏览面板（只读）'],
          ] as const)" :key="opt[0]" class="flex items-center gap-3">
            <Switch v-model="draft[opt[0]]" :label="opt[1]" />
            <span class="text-[13px]">{{ opt[1] }}</span>
            <span v-if="opt[2]" class="text-xs text-fg-3">{{ opt[2] }}</span>
          </label>
        </div>
      </div>
    </section>

    <!-- 账号 -->
    <section class="rounded-lg border border-line bg-surface">
      <h2 class="m-0 border-b border-line px-5 py-3.5 text-sm font-semibold">账号 · {{ app.user }}</h2>
      <form class="grid gap-4 p-5 md:grid-cols-3" @submit.prevent="changePassword">
        <Field label="原密码" for="lp-old"><Input id="lp-old" v-model="pw.old" type="password" autocomplete="current-password" /></Field>
        <Field label="新密码" for="lp-new"><Input id="lp-new" v-model="pw.new" type="password" autocomplete="new-password" /></Field>
        <Field label="确认新密码" for="lp-new2"><Input id="lp-new2" v-model="pw.confirm" type="password" autocomplete="new-password" /></Field>
        <div class="md:col-span-3">
          <Button type="submit" :loading="pwSaving" :disabled="!pw.old || !pw.new">修改密码</Button>
        </div>
      </form>
    </section>

    <!-- 备份 -->
    <section class="rounded-lg border border-line bg-surface">
      <h2 class="m-0 border-b border-line px-5 py-3.5 text-sm font-semibold">备份与迁移</h2>
      <div class="flex flex-col gap-4 p-5">
        <div class="flex flex-wrap items-center gap-3">
          <a href="/api/export" download class="lp-focus inline-flex h-9 items-center gap-1.5 rounded-sm border border-line bg-surface px-3.5 text-[13px] font-medium text-fg hover:bg-surface-hover">
            <Download class="size-3.5" />导出配置
          </a>
          <span class="text-xs text-fg-3">包含分组、应用与外观设置（JSON）；上传的图片不包含在内</span>
        </div>
        <div class="flex flex-wrap items-center gap-3">
          <Segmented v-model="importMode" :options="[{ value: 'merge', label: '追加' }, { value: 'replace', label: '覆盖' }]" label="导入方式" />
          <label class="lp-focus inline-flex h-9 cursor-pointer items-center gap-1.5 rounded-sm border border-line bg-surface px-3.5 text-[13px] font-medium hover:bg-surface-hover">
            <Upload class="size-3.5" />选择文件导入
            <input type="file" accept="application/json,.json" class="sr-only" @change="onImport" />
          </label>
        </div>
      </div>
    </section>

    <!-- 保存条 -->
    <Transition name="lp-bar">
      <div v-if="dirty" class="fixed inset-x-0 bottom-5 z-30 flex justify-center px-4 md:pl-[232px]">
        <div class="flex items-center gap-3 rounded-xl border border-line bg-surface py-2 pl-4 pr-2 shadow-pop">
          <span class="text-[13px] text-fg-2">有未保存的修改</span>
          <Button size="sm" @click="reset">撤销</Button>
          <Button size="sm" variant="primary" :loading="saving" @click="save">保存</Button>
        </div>
      </div>
    </Transition>
  </div>
</template>
