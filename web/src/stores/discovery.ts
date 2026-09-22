import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '@/lib/api'
import type { Device, DiscoverySummary, ScanStatus } from '@/lib/types'

/** 设备发现的共享状态：侧边栏角标、面板顶部提示与设备页共用。 */
export const useDiscovery = defineStore('discovery', () => {
  const devices = ref<Device[]>([])
  const summary = ref<DiscoverySummary | null>(null)
  const status = ref<ScanStatus | null>(null)

  async function loadDevices() {
    const r = await api.get<{ devices: Device[]; summary: DiscoverySummary }>('/api/discovery/devices')
    devices.value = r.devices
    summary.value = r.summary
  }

  async function loadStatus() {
    const r = await api.get<{ status: ScanStatus; summary: DiscoverySummary }>('/api/discovery/status')
    status.value = r.status
    summary.value = r.summary
    return r.status
  }

  async function scan(mode: 'full' | 'quick' = 'full') {
    await api.post('/api/discovery/scan', { mode })
    await loadStatus()
  }

  async function update(key: string, patch: Partial<Pick<Device, 'name' | 'userType' | 'acked'>>) {
    const d = await api.put<Device>(`/api/discovery/devices/${encodeURIComponent(key)}`, patch)
    const i = devices.value.findIndex((x) => x.key === key)
    if (i >= 0) devices.value[i] = { ...devices.value[i], ...d, raw: undefined }
    if (patch.acked !== undefined && summary.value) summary.value.new = devices.value.filter((x) => !x.acked && !x.self).length
    return d
  }

  async function remove(key: string) {
    await api.del(`/api/discovery/devices/${encodeURIComponent(key)}`)
    devices.value = devices.value.filter((d) => d.key !== key)
  }

  async function ackAll() {
    await api.post('/api/discovery/ack-all')
    devices.value.forEach((d) => (d.acked = true))
    if (summary.value) summary.value.new = 0
  }

  return { devices, summary, status, loadDevices, loadStatus, scan, update, remove, ackAll }
})
