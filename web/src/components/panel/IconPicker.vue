<script setup lang="ts">
import { computed, ref } from 'vue'
import { RefreshCw, Upload } from 'lucide-vue-next'
import type { Icon, Tint } from '@/lib/types'
import { ICONS, ICON_NAMES } from '@/lib/icons'
import { TINT_NAMES, tintColor, tintFor } from '@/lib/tints'
import { api } from '@/lib/api'
import { toastError } from '@/lib/toast'
import Segmented from '@/components/ui/Segmented.vue'
import Button from '@/components/ui/Button.vue'
import AppIcon from './AppIcon.vue'

const icon = defineModel<Icon>({ required: true })
const props = defineProps<{ title: string; tone: 'light' | 'dark'; fetching: boolean; canFetch: boolean }>()
const emit = defineEmits<{ fetch: [] }>()

const TABS = [
  { value: 'auto', label: '网站图标' },
  { value: 'lucide', label: '图标库' },
  { value: 'image', label: '上传' },
  { value: 'text', label: '文字' },
] as const
type TabKey = (typeof TABS)[number]['value']

const tab = computed<TabKey>({
  get: () => icon.value.type,
  set: (t) => {
    // 切换类型时保留颜色，清掉不兼容的值
    const keep = t === icon.value.type ? icon.value.value : ''
    icon.value = { type: t, value: t === 'lucide' && !keep ? 'globe' : keep, color: icon.value.color }
  },
})

const query = ref('')
const filtered = computed(() => ICON_NAMES.filter((n) => n.includes(query.value.trim().toLowerCase())))
const currentTint = computed<Tint>(() => (icon.value.color as Tint) || tintFor(props.title || '?'))
const uploading = ref(false)

async function onFile(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  ;(e.target as HTMLInputElement).value = ''
  if (!f) return
  uploading.value = true
  try {
    const r = await api.upload(f, 'icon')
    icon.value = { ...icon.value, type: 'image', value: r.url }
  } catch (err) {
    toastError(err)
  } finally {
    uploading.value = false
  }
}

function setColor(c: Tint) {
  icon.value = { ...icon.value, color: c }
}
</script>

<template>
  <div class="flex flex-col gap-3">
    <div class="flex items-center gap-3">
      <AppIcon :icon="icon" :title="title || '?'" :tone="tone" :size="48" />
      <Segmented v-model="tab" :options="[...TABS]" label="图标类型" />
    </div>

    <div v-if="tab === 'auto'" class="flex items-center gap-2 rounded-md bg-surface-2 px-3 py-2.5 text-xs text-fg-2">
      <span class="grow">{{ icon.value ? '已获取网站图标' : '尚未获取；获取失败时显示标题首字' }}</span>
      <Button size="sm" :loading="fetching" :disabled="!canFetch" @click="emit('fetch')">
        <RefreshCw v-if="!fetching" class="size-3.5" />{{ icon.value ? '重新获取' : '获取' }}
      </Button>
    </div>

    <div v-else-if="tab === 'lucide'" class="flex flex-col gap-2">
      <input
        v-model="query"
        placeholder="搜索图标（英文，如 server、film）"
        class="h-8 rounded-sm border border-line bg-surface px-2.5 text-[13px] outline-none focus:border-accent"
      />
      <div class="grid max-h-40 grid-cols-10 gap-1 overflow-y-auto rounded-md border border-line p-1.5">
        <button
          v-for="n in filtered"
          :key="n"
          type="button"
          :title="n"
          :aria-label="n"
          class="lp-focus flex aspect-square items-center justify-center rounded-xs hover:bg-surface-2"
          :class="icon.value === n ? 'bg-accent-soft ring-1 ring-accent' : ''"
          @click="icon = { ...icon, value: n }"
        >
          <component :is="ICONS[n]" :size="17" :stroke-width="1.8" :color="icon.value === n ? tintColor(currentTint, tone) : 'currentColor'" class="text-fg-2" />
        </button>
        <p v-if="!filtered.length" class="col-span-full m-0 py-4 text-center text-xs text-fg-3">没有匹配的图标</p>
      </div>
    </div>

    <label v-else-if="tab === 'image'" class="flex cursor-pointer items-center gap-2 rounded-md border border-dashed border-line-strong px-3 py-3 text-xs text-fg-2 hover:bg-surface-2">
      <Upload class="size-4" />
      <span class="grow">{{ uploading ? '上传中…' : icon.value ? '点击更换图片' : '点击上传图片（PNG / SVG / ICO / WebP，≤ 1 MB）' }}</span>
      <input type="file" accept="image/*,.ico,.svg" class="sr-only" @change="onFile" />
    </label>

    <div v-else class="flex items-center gap-2">
      <input
        :value="icon.value"
        maxlength="2"
        placeholder="留空使用标题首字"
        class="h-8 w-40 rounded-sm border border-line bg-surface px-2.5 text-[13px] outline-none focus:border-accent"
        @input="icon = { ...icon, value: ($event.target as HTMLInputElement).value }"
      />
      <span class="text-xs text-fg-3">最多 2 个字符</span>
    </div>

    <div v-if="tab === 'lucide' || tab === 'text' || (tab === 'auto' && !icon.value)" class="flex items-center gap-2">
      <span class="text-xs text-fg-3">色调</span>
      <button
        v-for="c in TINT_NAMES"
        :key="c"
        type="button"
        :aria-label="c"
        :aria-pressed="currentTint === c"
        class="lp-focus size-5 rounded-full"
        :class="currentTint === c ? 'ring-2 ring-offset-2 ring-offset-surface' : ''"
        :style="{ background: tintColor(c, tone), '--tw-ring-color': tintColor(c, tone) }"
        @click="setColor(c)"
      />
    </div>
  </div>
</template>
