<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { Sparkles } from 'lucide-vue-next'
import type { Group, Icon, Item } from '@/lib/types'
import { api } from '@/lib/api'
import { toast, toastError } from '@/lib/toast'
import { usePanel } from '@/stores/panel'
import { useApp } from '@/stores/app'
import Dialog from '@/components/ui/Dialog.vue'
import Field from '@/components/ui/Field.vue'
import Input from '@/components/ui/Input.vue'
import Button from '@/components/ui/Button.vue'
import Segmented from '@/components/ui/Segmented.vue'
import IconPicker from './IconPicker.vue'

const open = defineModel<boolean>('open', { default: false })
const props = defineProps<{ item?: Item | null; groupId?: string; groups: Group[] }>()

const panel = usePanel()
const app = useApp()

const form = reactive({
  title: '', urlLan: '', urlWan: '', desc: '', groupId: '',
  openMode: 'new' as 'new' | 'self',
  icon: { type: 'auto', value: '', color: '' } as Icon,
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
  })
})

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
    const input = { ...form, icon: { ...form.icon } }
    if (props.item) await panel.updateItem(props.item.id, input)
    else await panel.addItem(input)
    toast(props.item ? '已保存' : '已添加')
    open.value = false
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
