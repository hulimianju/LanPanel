<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { LayoutGrid, ListChecks, LogOut, Menu, Radar, RefreshCw, SlidersHorizontal, X } from 'lucide-vue-next'
import { useApp } from '@/stores/app'
import { useDiscovery } from '@/stores/discovery'

const app = useApp()
const disc = useDiscovery()
const route = useRoute()

// 侧边栏角标：新设备数量、扫描状态
let timer: number | undefined
const refreshStatus = () => disc.loadStatus().catch(() => {})
onMounted(() => {
  refreshStatus()
  timer = window.setInterval(refreshStatus, 30000)
})
onBeforeUnmount(() => clearInterval(timer))
const router = useRouter()
const mobileOpen = ref(false)
watch(() => route.fullPath, () => (mobileOpen.value = false))

const nav = [
  { to: '/', label: '面板', icon: LayoutGrid },
  { to: '/admin/discovery', label: '设备发现', icon: Radar },
  { to: '/admin/scan', label: '扫描任务', icon: RefreshCw },
  { to: '/admin/rules', label: '端口规则', icon: ListChecks },
  { to: '/admin/settings', label: '设置', icon: SlidersHorizontal },
]

async function logout() {
  await app.logout()
  router.replace('/login')
}
</script>

<template>
  <div class="flex min-h-dvh bg-bg text-fg">
    <!-- 移动端顶栏 -->
    <header class="fixed inset-x-0 top-0 z-30 flex h-14 items-center gap-3 border-b border-line bg-surface px-4 md:hidden">
      <button type="button" class="lp-focus -ml-1 flex size-9 items-center justify-center rounded-sm" aria-label="打开菜单" @click="mobileOpen = true">
        <Menu class="size-5" />
      </button>
      <span class="text-[15px] font-semibold">{{ route.meta.title ?? '管理' }}</span>
    </header>
    <div v-if="mobileOpen" class="fixed inset-0 z-40 bg-[var(--overlay)] md:hidden" @click="mobileOpen = false" />

    <nav
      aria-label="主导航"
      class="fixed inset-y-0 left-0 z-50 flex w-[232px] flex-col gap-1 border-r border-line bg-bg-subtle px-3 py-[18px] transition-transform md:sticky md:top-0 md:h-dvh md:translate-x-0"
      :class="mobileOpen ? 'translate-x-0' : '-translate-x-full'"
    >
      <div class="mb-3.5 flex h-9 items-center gap-2.5 px-2">
        <div class="flex size-[26px] items-center justify-center rounded-[7px] bg-accent">
          <LayoutGrid class="size-[15px] text-white" />
        </div>
        <span class="grow truncate text-[15px] font-semibold tracking-[-0.01em]">{{ app.settings?.siteTitle }}</span>
        <button type="button" class="lp-focus flex size-8 items-center justify-center rounded-sm md:hidden" aria-label="关闭菜单" @click="mobileOpen = false">
          <X class="size-4" />
        </button>
      </div>
      <span class="px-2.5 pb-1.5 text-[11px] font-medium text-fg-3">工作台</span>
      <RouterLink
        v-for="n in nav"
        :key="n.to"
        :to="n.to"
        class="lp-focus flex h-9 items-center gap-2.5 rounded-sm px-2.5 text-sm transition-colors"
        :class="route.path === n.to ? 'bg-surface-2 font-medium text-fg' : 'text-fg-2 hover:bg-surface-hover hover:text-fg'"
      >
        <component :is="n.icon" class="size-[17px]" :stroke-width="1.8" /><span class="grow">{{ n.label }}</span>
        <span
          v-if="n.to === '/admin/discovery' && disc.summary?.new"
          class="flex h-5 min-w-5 items-center justify-center rounded-full bg-accent-soft px-1.5 text-[11px] font-semibold text-accent-soft-fg"
        >{{ disc.summary.new }}</span>
        <RefreshCw v-if="n.to === '/admin/scan' && disc.status?.running" class="size-3.5 animate-spin text-accent" />
      </RouterLink>
      <div class="grow" />
      <div class="flex flex-col gap-1 rounded-md border border-line bg-surface p-3">
        <div class="flex items-center gap-2 text-xs text-fg-2">
          <span class="size-[7px] rounded-full bg-success" />{{ disc.status?.running ? '正在扫描…' : '运行中' }}
        </div>
        <div v-if="disc.summary" class="text-[11px] text-fg-3">{{ disc.summary.online }} 台设备在线</div>
        <div class="font-mono text-[11px] leading-relaxed text-fg-3">LanPanel {{ app.version }}</div>
      </div>
      <div class="mt-1.5 flex h-11 items-center gap-2.5 px-2">
        <div class="flex size-7 items-center justify-center rounded-full bg-surface-2 text-xs font-semibold text-fg-2">
          {{ app.user?.slice(0, 1).toUpperCase() }}
        </div>
        <span class="grow truncate text-[13px] text-fg-2">{{ app.user }}</span>
        <button type="button" class="lp-focus flex size-7 items-center justify-center rounded-xs text-fg-3 hover:bg-surface-2 hover:text-fg" aria-label="退出登录" @click="logout">
          <LogOut class="size-[15px]" />
        </button>
      </div>
    </nav>

    <main class="min-w-0 grow px-4 pb-24 pt-20 md:px-8 md:pt-7">
      <div class="mx-auto max-w-[1100px]"><RouterView /></div>
    </main>
  </div>
</template>
