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
