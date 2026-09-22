export type Tint = 'blue' | 'teal' | 'amber' | 'violet' | 'rose' | 'green' | 'slate'

export interface Icon {
  type: 'auto' | 'lucide' | 'image' | 'text'
  value: string
  color?: Tint | ''
}

export interface Item {
  id: string
  groupId: string
  title: string
  desc?: string
  urlLan: string
  urlWan?: string
  icon: Icon
  openMode: 'new' | 'self'
  order: number
  deviceMac?: string
  devicePort?: number
}

export interface Group {
  id: string
  name: string
  order: number
  items: Item[]
}

export type Tone = 'auto' | 'light' | 'dark'

export interface Wallpaper {
  type: 'none' | 'image' | 'color'
  value: string
  blur: number
  dim: number
  tone: Tone
  luminance: number
}

export interface Settings {
  siteTitle: string
  theme: 'auto' | 'light' | 'dark'
  wallpaper: Wallpaper
  searchEngine: 'bing' | 'baidu' | 'google' | 'duckduckgo' | 'custom'
  customSearchUrl?: string
  showClock: boolean
  showSeconds: boolean
  showWidgets: boolean
  addressMode: 'auto' | 'lan' | 'wan'
  publicPanel: boolean
  cardSize: 'comfortable' | 'compact'
}

export interface Bootstrap {
  needSetup: boolean
  user: string | null
  settings: Settings
  clientLan: boolean
  version: string
}

export interface SystemInfo {
  hostname: string
  os: string
  arch: string
  uptime: number
  cpuPercent: number
  memTotal: number
  memUsed: number
  netRx: number
  netTx: number
  selfMem: number
}

// ---- 设备发现 ----

export type DeviceType = '' | 'router' | 'switch' | 'ap' | 'nas' | 'server' | 'pc' | 'phone' | 'tv' | 'printer' | 'camera' | 'iot'

export interface Service {
  port: number
  proto: string
  scheme: 'http' | 'https' | 'tcp'
  name: string
  ruleId?: string
  category?: string
  icon?: string
  color?: Tint | ''
  url?: string
  title?: string
  server?: string
  banner?: string
  source: 'port' | 'mdns' | 'ssdp' | 'wsd'
}

export interface IPRecord {
  ip: string
  from: string
  to?: string
}

export interface Device {
  key: string
  mac?: string
  ip: string
  hostname?: string
  name?: string
  vendor?: string
  model?: string
  os?: string
  type?: DeviceType
  userType?: DeviceType
  sources: string[]
  services: Service[]
  randomized?: boolean
  self?: boolean
  gateway?: boolean
  online: boolean
  firstSeen: string
  lastSeen: string
  ipHistory: IPRecord[]
  raw?: { source: string; text: string }[]
  acked: boolean
}

export interface DiscoverySummary {
  online: number
  total: number
  new: number
  ipChanged: number
  web: number
  webHosts: number
}

export interface Phase {
  key: string
  label: string
  desc: string
  status: 'wait' | 'run' | 'done' | 'skip'
  done: number
  total: number
  detail: string
}

export interface ScanStatus {
  running: boolean
  mode?: 'full' | 'quick'
  trigger?: string
  subnets?: string[]
  start?: string
  phases?: Phase[]
  percent: number
  found: number
  nextFull?: string
  nextQuick?: string
  privilege: string
  warning?: string
}

export interface LogLine {
  seq: number
  time: string
  kind: string
  target: string
  msg: string
}

export interface ScanSummary {
  id: number
  mode: 'full' | 'quick'
  trigger: 'manual' | 'schedule' | 'startup'
  subnets: string[]
  start: string
  end: string
  online: number
  new: number
  ipChanged: number
  services: number
  canceled?: boolean
  error?: string
}

export interface DiscoveryConfig {
  autoSubnets: boolean
  subnets: string[]
  extraPorts: number[]
  concurrency: number
  timeoutMs: number
  fullInterval: number
  quickInterval: number
  scanOnStart: boolean
  protocols: { icmp: boolean; mdns: boolean; ssdp: boolean; netbios: boolean; wsd: boolean; dns: boolean; snmp: boolean }
  snmpCommunity: string
}

// ---- 端口规则 ----

export interface Cond {
  field: string
  op?: 'contains' | 'equals' | 'prefix' | 'regex' | 'exists'
  value?: string
}

export interface Rule {
  id: string
  name: string
  category: string
  ports: number[]
  proto: 'web' | 'http' | 'https' | 'tcp'
  match?: Cond[]
  all?: boolean
  probe?: string
  device?: DeviceType
  system?: boolean
  icon?: string
  color?: Tint | ''
  url?: string
  disabled?: boolean
  builtin: boolean
  modified: boolean
}

export interface TestResult {
  port: number
  open: boolean
  scheme?: string
  status?: number
  title?: string
  server?: string
  banner?: string
  favicon?: string
  matched: boolean
  ruleId?: string
  ruleName?: string
  url?: string
}

// ---- 阶段 3：联动与通知 ----

export interface ItemStatus {
  state: 'online' | 'warn' | 'offline'
  bound: boolean
  device?: string
  key?: string
  mac?: string
  ip?: string
  note?: string
}

export interface DeviceEvent {
  id: number
  time: string
  type: 'new' | 'ip_changed' | 'offline' | 'online'
  key: string
  mac?: string
  name: string
  ip: string
  oldIp?: string
  vendor?: string
  detail?: string
}

export interface NotifyConfig {
  enabled: boolean
  type: 'generic' | 'wecom' | 'dingtalk' | 'feishu' | 'bark' | 'serverchan'
  url: string
  events: { new: boolean; ipChanged: boolean; offline: boolean; online: boolean }
}
