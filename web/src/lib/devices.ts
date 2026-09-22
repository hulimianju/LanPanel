import type { Component } from 'vue'
import {
  Camera, HardDrive, HelpCircle, Laptop, Lightbulb, Network, Printer, Router, Server, Smartphone, Tv, Wifi,
} from 'lucide-vue-next'
import type { Device, DeviceType, Tint } from './types'

export const DEVICE_TYPES: Record<DeviceType, { label: string; icon: Component; tint: Tint }> = {
  '': { label: '未知', icon: HelpCircle, tint: 'slate' },
  router: { label: '路由器', icon: Router, tint: 'teal' },
  switch: { label: '交换机', icon: Network, tint: 'teal' },
  ap: { label: '无线 AP', icon: Wifi, tint: 'teal' },
  nas: { label: 'NAS', icon: HardDrive, tint: 'blue' },
  server: { label: '服务器', icon: Server, tint: 'amber' },
  pc: { label: '电脑', icon: Laptop, tint: 'slate' },
  phone: { label: '手机/平板', icon: Smartphone, tint: 'violet' },
  tv: { label: '电视/投屏', icon: Tv, tint: 'rose' },
  printer: { label: '打印机', icon: Printer, tint: 'slate' },
  camera: { label: '摄像头', icon: Camera, tint: 'rose' },
  iot: { label: '智能家居', icon: Lightbulb, tint: 'green' },
}

/** 列表筛选分组 */
export const TYPE_FILTERS: { key: string; label: string; types: DeviceType[] }[] = [
  { key: 'net', label: '网络设备', types: ['router', 'switch', 'ap'] },
  { key: 'nas', label: 'NAS / 服务器', types: ['nas', 'server'] },
  { key: 'pc', label: '电脑', types: ['pc'] },
  { key: 'phone', label: '手机', types: ['phone'] },
  { key: 'home', label: '家居影音', types: ['tv', 'iot', 'camera', 'printer'] },
  { key: 'other', label: '未知', types: [''] },
]

export function deviceType(d: Device): DeviceType {
  return (d.userType || d.type || '') as DeviceType
}

export function deviceName(d: Device) {
  return d.name || d.hostname || d.model || d.os || d.vendor || d.ip
}

/** 名称下方的副标题：系统 / 型号 / 主机名中与名称不重复的部分。 */
export function deviceSubtitle(d: Device) {
  const name = deviceName(d)
  const parts = [d.os, d.model, d.hostname].filter((x): x is string => !!x && x !== name)
  const uniq = [...new Set(parts)]
  if (d.self) uniq.unshift('本机')
  if (d.gateway) uniq.unshift('网关')
  return uniq.slice(0, 3).join(' · ') || DEVICE_TYPES[deviceType(d)].label
}


export function relTime(iso?: string) {
  if (!iso) return '-'
  const s = Math.round((Date.now() - new Date(iso).getTime()) / 1000)
  if (s < 90) return '刚刚'
  if (s < 3600) return `${Math.round(s / 60)} 分钟前`
  if (s < 86400) return `${Math.round(s / 3600)} 小时前`
  return `${Math.round(s / 86400)} 天前`
}

export function fmtDate(iso?: string) {
  if (!iso) return '-'
  const d = new Date(iso)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

export function fmtShort(iso?: string) {
  if (!iso) return '-'
  const d = new Date(iso)
  return `${d.getMonth() + 1}月${d.getDate()}日 ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

export const CATEGORY_LABELS: Record<string, string> = {
  nas: 'NAS 与存储', virt: '虚拟化', router: '路由与网络', media: '影音', download: '下载',
  ops: '运维开发', smarthome: '智能家居', printcam: '打印与摄像', base: '基础服务', custom: '自定义',
}
