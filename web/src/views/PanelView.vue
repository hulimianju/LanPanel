<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { VueDraggable } from 'vue-draggable-plus'
import { Check, LayoutGrid, LogIn, Pencil, Plus, Settings2 } from 'lucide-vue-next'
import { useApp } from '@/stores/app'
import { usePanel } from '@/stores/panel'
import type { Group, Item } from '@/lib/types'
import { itemUrl } from '@/lib/address'
import { confirm } from '@/lib/confirm'
import { toast, toastError } from '@/lib/toast'
import Wallpaper from '@/components/panel/Wallpaper.vue'
import Clock from '@/components/panel/Clock.vue'
import SearchBar from '@/components/panel/SearchBar.vue'
import Widgets from '@/components/panel/Widgets.vue'
import GroupSection from '@/components/panel/GroupSection.vue'
import ItemEditor from '@/components/panel/ItemEditor.vue'
import Dialog from '@/components/ui/Dialog.vue'
import Input from '@/components/ui/Input.vue'
import Button from '@/components/ui/Button.vue'

const app = useApp()
const panel = usePanel()
const settings = computed(() => app.settings!)
const tone = computed(() => app.surfaceTone)
const isAdmin = computed(() => !!app.user)

onMounted(() => panel.load().catch(toastError))

// ---- 搜索 ----
const query = ref('')
const searchBar = ref<InstanceType<typeof SearchBar>>()
function matches(it: Item, q: string) {
  return [it.title, it.desc ?? '', it.urlLan, it.urlWan ?? ''].some((s) => s.toLowerCase().includes(q))
}
const visibleGroups = computed(() => {
  const q = query.value.trim().toLowerCase()
  return panel.groups
    .map((g) => ({ group: g, items: q && !panel.editing ? g.items.filter((it) => matches(it, q)) : g.items }))
    .filter((v) => !q || panel.editing || v.items.length > 0)
})
const matchCount = computed(() => visibleGroups.value.reduce((n, v) => n + v.items.length, 0))

/** 回车：只匹配到一个应用时直接打开，否则用搜索引擎搜索。 */
function onSubmit() {
  const q = query.value.trim()
  if (!q) return
  const all = visibleGroups.value.flatMap((v) => v.items)
  if (all.length === 1) {
    const it = all[0]
    window.open(itemUrl(it, app.addressMode), it.openMode === 'new' ? '_blank' : '_self', 'noopener')
  } else {
    searchBar.value?.webSearch()
  }
}

// ---- 编辑 ----
const editorOpen = ref(false)
const editingItem = ref<Item | null>(null)
const editorGroupId = ref<string>()

function openAdd(g?: Group) {
  if (!panel.groups.length) {
    groupDialog.value = true
    return
  }
  editingItem.value = null
  editorGroupId.value = g?.id
  editorOpen.value = true
}
function openEdit(it: Item) {
  editingItem.value = it
  editorOpen.value = true
}
async function removeItem(it: Item) {
  if (!(await confirm({ title: `删除「${it.title}」？`, confirmText: '删除', danger: true }))) return
  try {
    await panel.removeItem(it.id)
  } catch (e) {
    toastError(e)
  }
}
async function removeGroup(g: Group) {
  const ok = await confirm({
    title: `删除分组「${g.name}」？`,
    message: g.items.length ? `分组内的 ${g.items.length} 个应用将一并删除，此操作无法撤销。` : undefined,
    confirmText: '删除',
    danger: true,
  })
  if (!ok) return
  try {
    await panel.removeGroup(g.id)
  } catch (e) {
    toastError(e)
  }
}
async function renameGroup(g: Group, name: string) {
  try {
    await panel.renameGroup(g.id, name)
  } catch (e) {
    toastError(e)
  }
}
async function saveLayout() {
  try {
    await panel.saveLayout()
  } catch (e) {
    toastError(e)
    panel.load()
  }
}
async function saveGroupOrder() {
  try {
    await panel.saveGroupOrder()
  } catch (e) {
    toastError(e)
    panel.load()
  }
}

const groupDialog = ref(false)
const groupName = ref('')
const groupSaving = ref(false)
async function createGroup() {
  const name = groupName.value.trim()
  if (!name) return
  groupSaving.value = true
  try {
    await panel.addGroup(name)
    groupDialog.value = false
    groupName.value = ''
    toast('分组已创建')
  } catch (e) {
    toastError(e)
  } finally {
    groupSaving.value = false
  }
}

const tileMin = computed(() => (settings.value.cardSize === 'compact' ? '190px' : '240px'))
</script>

<template>
  <div
    class="lp-surface relative isolate flex min-h-dvh flex-col"
    :data-tone="tone"
    :data-wp="settings.wallpaper.type === 'image' ? 'image' : 'plain'"
    :style="{ '--tile-min': tileMin }"
  >
    <Wallpaper :wallpaper="settings.wallpaper" :tone="tone" />

    <header class="flex h-16 shrink-0 items-center gap-3 px-4 sm:px-8">
      <div class="flex min-w-0 items-center gap-2.5">
        <div class="flex size-7 shrink-0 items-center justify-center rounded-sm bg-accent">
          <LayoutGrid class="size-4 text-white" />
        </div>
        <span class="lp-text-shadow truncate text-[15px] font-semibold tracking-[-0.01em]">{{ settings.siteTitle }}</span>
      </div>
      <div class="grow" />
      <div role="radiogroup" aria-label="地址模式" class="lp-glass-strong flex rounded-[9px] p-[3px]">
        <button
          v-for="m in (['lan', 'wan'] as const)"
          :key="m"
          type="button"
          role="radio"
          :aria-checked="app.addressMode === m"
          class="lp-focus h-[26px] rounded-xs px-3 text-xs transition-colors"
          :class="app.addressMode === m ? 'bg-s-glass-hover font-medium text-s-fg' : 'text-s-fg-3 hover:text-s-fg'"
          :title="m === 'lan' ? '使用内网地址打开应用' : '使用外网地址打开应用'"
          @click="app.setAddressMode(m)"
        >
          {{ m === 'lan' ? '内网' : '外网' }}
        </button>
      </div>
      <template v-if="isAdmin">
        <button
          type="button"
          class="lp-glass-strong lp-focus flex h-8 items-center gap-1.5 rounded-sm px-3 text-[13px] text-s-fg-2 hover:text-s-fg"
          :aria-pressed="panel.editing"
          @click="panel.editing = !panel.editing"
        >
          <Pencil v-if="!panel.editing" class="size-3.5" /><Check v-else class="size-3.5" />
          <span class="hidden sm:inline">{{ panel.editing ? '完成' : '编辑' }}</span>
        </button>
        <RouterLink to="/admin/settings" aria-label="管理后台" class="lp-glass-strong lp-focus flex size-8 items-center justify-center rounded-sm text-s-fg-2 hover:text-s-fg">
          <Settings2 class="size-4" />
        </RouterLink>
      </template>
      <RouterLink v-else to="/login" class="lp-glass-strong lp-focus flex h-8 items-center gap-1.5 rounded-sm px-3 text-[13px] text-s-fg-2 hover:text-s-fg">
        <LogIn class="size-3.5" />登录
      </RouterLink>
    </header>

    <main class="mx-auto flex w-full max-w-[1200px] grow flex-col gap-7 px-4 pb-28 pt-8 sm:px-6 sm:pt-12">
      <Clock v-if="settings.showClock" :seconds="settings.showSeconds" />
      <div class="flex justify-center">
        <SearchBar ref="searchBar" v-model="query" :engine="settings.searchEngine" :custom-url="settings.customSearchUrl" @submit="onSubmit" />
      </div>
      <Widgets v-if="settings.showWidgets && isAdmin" :tone="tone" />

      <p v-if="query && !panel.editing" class="lp-text-shadow -mb-2 text-center text-xs text-s-fg-3">
        {{ matchCount ? `匹配 ${matchCount} 个应用${matchCount === 1 ? '，回车直接打开' : '，回车在网页中搜索'}` : '没有匹配的应用，回车在网页中搜索' }}
      </p>

      <VueDraggable
        v-if="panel.editing"
        v-model="panel.groups"
        handle=".lp-group-handle"
        :animation="180"
        class="flex flex-col gap-7"
        @end="saveGroupOrder"
      >
        <GroupSection
          v-for="g in panel.groups"
          :key="g.id"
          :group="g"
          :items="g.items"
          :tone="tone"
          :mode="app.addressMode"
          :editing="true"
          :compact="settings.cardSize === 'compact'"
          @layout="saveLayout"
          @add-item="openAdd"
          @edit-item="openEdit"
          @remove-item="removeItem"
          @rename="renameGroup"
          @remove="removeGroup"
        />
      </VueDraggable>
      <template v-else>
        <GroupSection
          v-for="v in visibleGroups"
          :key="v.group.id"
          :group="v.group"
          :items="v.items"
          :tone="tone"
          :mode="app.addressMode"
          :editing="false"
          :compact="settings.cardSize === 'compact'"
        />
      </template>

      <div v-if="panel.loaded && !panel.groups.length" class="lp-glass mx-auto flex max-w-md flex-col items-center gap-3 rounded-xl px-8 py-10 text-center">
        <LayoutGrid class="size-8 text-s-fg-3" />
        <p class="m-0 text-[15px] font-medium">面板还是空的</p>
        <p class="m-0 text-[13px] text-s-fg-2">{{ isAdmin ? '先创建一个分组，再把常用的网页服务添加进来。' : '管理员还没有添加任何应用。' }}</p>
        <Button v-if="isAdmin" variant="primary" @click="groupDialog = true"><Plus class="size-4" />创建分组</Button>
      </div>
    </main>

    <Transition name="lp-bar">
      <div v-if="panel.editing" class="fixed inset-x-0 bottom-5 z-30 flex justify-center px-4">
        <div class="flex items-center gap-2 rounded-xl border border-line bg-surface py-2 pl-4 pr-2 text-fg shadow-pop">
          <span class="hidden text-[13px] text-fg-2 md:inline">拖动卡片调整顺序，可跨分组；拖动分组左侧手柄调整分组顺序</span>
          <Button size="sm" @click="groupDialog = true"><Plus class="size-3.5" />分组</Button>
          <Button size="sm" @click="openAdd()"><Plus class="size-3.5" />应用</Button>
          <Button size="sm" variant="primary" @click="panel.editing = false"><Check class="size-3.5" />完成</Button>
        </div>
      </div>
    </Transition>

    <ItemEditor v-model:open="editorOpen" :item="editingItem" :group-id="editorGroupId" :groups="panel.groups" />

    <Dialog v-model:open="groupDialog" title="新建分组" width="400px">
      <form id="lp-group-form" @submit.prevent="createGroup">
        <Input v-model="groupName" placeholder="例如：存储与虚拟化" autofocus />
      </form>
      <template #footer>
        <Button @click="groupDialog = false">取消</Button>
        <Button variant="primary" type="submit" form="lp-group-form" :loading="groupSaving" :disabled="!groupName.trim()">创建</Button>
      </template>
    </Dialog>
  </div>
</template>

