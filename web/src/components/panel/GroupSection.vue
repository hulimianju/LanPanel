<script setup lang="ts">
import { nextTick, ref } from 'vue'
import { VueDraggable } from 'vue-draggable-plus'
import { GripVertical, Pencil, Plus, Trash2 } from 'lucide-vue-next'
import type { Group, Item, ItemStatus } from '@/lib/types'
import AppTile from './AppTile.vue'

const props = defineProps<{
  group: Group
  items: Item[] // 搜索过滤后的卡片；编辑模式下即 group.items
  tone: 'light' | 'dark'
  mode: 'lan' | 'wan'
  editing: boolean
  compact: boolean
  status?: Record<string, ItemStatus>
}>()
const emit = defineEmits<{
  layout: []
  addItem: [Group]
  editItem: [Item]
  removeItem: [Item]
  rename: [Group, string]
  remove: [Group]
}>()

const renaming = ref(false)
const draft = ref('')
const input = ref<HTMLInputElement>()

async function startRename() {
  draft.value = props.group.name
  renaming.value = true
  await nextTick()
  input.value?.select()
}

function commitRename() {
  if (!renaming.value) return
  renaming.value = false
  const name = draft.value.trim()
  if (name && name !== props.group.name) emit('rename', props.group, name)
}

const gridClass =
  'grid grid-cols-4 gap-x-2 gap-y-4 sm:gap-3 ' +
  'sm:grid-cols-[repeat(auto-fill,minmax(var(--tile-min),1fr))]'
</script>

<template>
  <section class="flex flex-col gap-3" :aria-label="group.name">
    <div class="flex h-7 items-center gap-2.5">
      <button
        v-if="editing"
        type="button"
        class="lp-group-handle lp-focus -ml-1 flex size-6 cursor-grab items-center justify-center rounded-xs text-s-fg-3 hover:text-s-fg"
        aria-label="拖动调整分组顺序"
      >
        <GripVertical class="size-4" />
      </button>
      <input
        v-if="renaming"
        ref="input"
        v-model="draft"
        maxlength="32"
        class="h-7 w-48 rounded-xs border border-s-border bg-s-glass-strong px-2 text-[13px] font-semibold text-s-fg outline-none"
        @keydown.enter="commitRename"
        @keydown.esc="renaming = false"
        @blur="commitRename"
      />
      <h2 v-else class="lp-text-shadow m-0 text-[13px] font-semibold text-s-fg" @dblclick="editing && startRename()">{{ group.name }}</h2>
      <span class="lp-text-shadow text-xs tabular-nums text-s-fg-3">{{ items.length }}</span>
      <div class="h-px grow bg-s-divider" />
      <template v-if="editing">
        <button type="button" class="lp-focus flex size-7 items-center justify-center rounded-xs text-s-fg-3 hover:bg-s-glass hover:text-s-fg" aria-label="重命名分组" @click="startRename">
          <Pencil class="size-3.5" />
        </button>
        <button type="button" class="lp-focus flex size-7 items-center justify-center rounded-xs text-s-fg-3 hover:bg-s-glass hover:text-danger" aria-label="删除分组" @click="emit('remove', group)">
          <Trash2 class="size-3.5" />
        </button>
        <button type="button" class="lp-focus flex size-7 items-center justify-center rounded-xs text-s-fg-3 hover:bg-s-glass hover:text-s-fg" aria-label="在此分组添加应用" @click="emit('addItem', group)">
          <Plus class="size-4" />
        </button>
      </template>
    </div>

    <VueDraggable
      v-if="editing"
      v-model="group.items"
      group="lp-items"
      :animation="160"
      ghost-class="sortable-ghost"
      :class="[gridClass, 'min-h-[68px]']"
      @end="emit('layout')"
    >
      <AppTile
        v-for="it in group.items"
        :key="it.id"
        :item="it"
        :tone="tone"
        :mode="mode"
        :editing="true"
        :compact="compact"
        @edit="emit('editItem', $event)"
        @remove="emit('removeItem', $event)"
      />
      <button
        v-if="group.items.length === 0"
        type="button"
        class="lp-focus col-span-full flex h-[68px] items-center justify-center gap-2 rounded-lg border border-dashed border-s-border text-[13px] text-s-fg-3 hover:text-s-fg"
        @click="emit('addItem', group)"
      >
        <Plus class="size-4" />拖动卡片到这里，或点击添加
      </button>
    </VueDraggable>
    <div v-else :class="gridClass">
      <AppTile
        v-for="it in items"
        :key="it.id"
        :item="it"
        :tone="tone"
        :mode="mode"
        :editing="false"
        :compact="compact"
        :status="status?.[it.id]"
      />
    </div>
  </section>
</template>
