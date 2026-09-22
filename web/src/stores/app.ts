import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import { api } from '@/lib/api'
import type { Bootstrap, Settings } from '@/lib/types'
import { resolveTone, type ResolvedTone } from '@/lib/surface'
import { sampleImageLuminance } from '@/lib/luminance'

const media = window.matchMedia('(prefers-color-scheme: dark)')

function storageGet(key: string) {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}
function storageSet(key: string, v: string | null) {
  try {
    if (v === null) localStorage.removeItem(key)
    else localStorage.setItem(key, v)
  } catch {
    /* 隐私模式等场景下忽略 */
  }
}

export const useApp = defineStore('app', () => {
  const ready = ref(false)
  const needSetup = ref(false)
  const user = ref<string | null>(null)
  const settings = ref<Settings | null>(null)
  const clientLan = ref(true)
  const version = ref('')
  const systemDark = ref(media.matches)
  media.addEventListener('change', (e) => (systemDark.value = e.matches))

  /** 访客个人选择的地址模式（仅存本地），优先于站点默认值。 */
  const addressOverride = ref<'lan' | 'wan' | null>((storageGet('lp-addr') as 'lan' | 'wan' | null) ?? null)
  /** 图片壁纸的实时采样亮度（服务端未保存时使用）。 */
  const sampledLum = ref<number | undefined>()

  const uiTheme = computed<ResolvedTone>(() => {
    const t = settings.value?.theme ?? 'auto'
    return t === 'auto' ? (systemDark.value ? 'dark' : 'light') : t
  })

  const surfaceTone = computed<ResolvedTone>(() =>
    settings.value ? resolveTone(settings.value, uiTheme.value, sampledLum.value) : uiTheme.value,
  )

  const addressMode = computed<'lan' | 'wan'>(() => {
    if (addressOverride.value) return addressOverride.value
    const m = settings.value?.addressMode ?? 'auto'
    return m === 'auto' ? (clientLan.value ? 'lan' : 'wan') : m
  })

  function setAddressMode(m: 'lan' | 'wan') {
    addressOverride.value = m
    storageSet('lp-addr', m)
  }

  watch(
    uiTheme,
    (t) => {
      document.documentElement.dataset.theme = t
      storageSet('lp-theme', settings.value?.theme ?? 'auto')
    },
    { immediate: true },
  )

  // 图片壁纸且未保存亮度时，在客户端采样
  watch(
    () => settings.value?.wallpaper,
    async (wp) => {
      sampledLum.value = undefined
      if (wp?.type === 'image' && wp.luminance < 0 && wp.value) {
        try {
          sampledLum.value = await sampleImageLuminance(wp.value)
        } catch {
          /* 采样失败时按深色处理 */
        }
      }
    },
    { deep: true },
  )

  watch(
    () => settings.value?.siteTitle,
    (t) => {
      if (t) document.title = t
    },
  )

  async function load() {
    const b = await api.get<Bootstrap>('/api/bootstrap')
    needSetup.value = b.needSetup
    user.value = b.user
    settings.value = b.settings
    clientLan.value = b.clientLan
    version.value = b.version
    ready.value = true
  }

  async function login(username: string, password: string, remember: boolean) {
    const r = await api.post<{ user: string }>('/api/login', { username, password, remember })
    user.value = r.user
  }

  async function setup(username: string, password: string) {
    const r = await api.post<{ user: string }>('/api/setup', { username, password })
    user.value = r.user
    needSetup.value = false
  }

  async function logout() {
    await api.post('/api/logout')
    user.value = null
  }

  async function saveSettings(patch: Partial<Settings>) {
    settings.value = await api.put<Settings>('/api/settings', { ...settings.value, ...patch })
  }

  return {
    ready, needSetup, user, settings, clientLan, version, uiTheme, surfaceTone, addressMode, addressOverride,
    sampledLum, load, login, setup, logout, saveSettings, setAddressMode,
  }
})
