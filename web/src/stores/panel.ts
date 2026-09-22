import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '@/lib/api'
import type { Group, Item } from '@/lib/types'

export type ItemInput = Omit<Item, 'id' | 'order'>

export const usePanel = defineStore('panel', () => {
  const groups = ref<Group[]>([])
  const loaded = ref(false)
  const editing = ref(false)

  async function load() {
    const r = await api.get<{ groups: Group[] }>('/api/panel')
    groups.value = r.groups
    loaded.value = true
  }

  async function addGroup(name: string) {
    const g = await api.post<Group>('/api/groups', { name })
    groups.value.push(g)
    return g
  }

  async function renameGroup(id: string, name: string) {
    await api.put(`/api/groups/${id}`, { name })
    const g = groups.value.find((x) => x.id === id)
    if (g) g.name = name
  }

  async function removeGroup(id: string) {
    await api.del(`/api/groups/${id}`)
    groups.value = groups.value.filter((g) => g.id !== id)
  }

  async function saveGroupOrder() {
    await api.put('/api/groups/order', { ids: groups.value.map((g) => g.id) })
  }

  async function addItem(input: ItemInput) {
    const it = await api.post<Item>('/api/items', input)
    groups.value.find((g) => g.id === it.groupId)?.items.push(it)
    return it
  }

  async function updateItem(id: string, input: ItemInput) {
    const it = await api.put<Item>(`/api/items/${id}`, input)
    // 可能换了分组：先移除再放入目标分组
    for (const g of groups.value) {
      const i = g.items.findIndex((x) => x.id === id)
      if (i < 0) continue
      if (g.id === it.groupId) {
        g.items[i] = it
        return it
      }
      g.items.splice(i, 1)
    }
    groups.value.find((g) => g.id === it.groupId)?.items.push(it)
    return it
  }

  async function removeItem(id: string) {
    await api.del(`/api/items/${id}`)
    for (const g of groups.value) g.items = g.items.filter((x) => x.id !== id)
  }

  /** 拖拽结束后提交完整布局（支持跨分组）。 */
  async function saveLayout() {
    await api.put('/api/items/layout', {
      groups: groups.value.map((g) => ({ id: g.id, itemIds: g.items.map((i) => i.id) })),
    })
    for (const g of groups.value) g.items.forEach((it) => (it.groupId = g.id))
  }

  return {
    groups, loaded, editing, load, addGroup, renameGroup, removeGroup, saveGroupOrder,
    addItem, updateItem, removeItem, saveLayout,
  }
})
