<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useApp } from '@/stores/app'
import AuthShell from './AuthShell.vue'
import Field from '@/components/ui/Field.vue'
import Input from '@/components/ui/Input.vue'
import Button from '@/components/ui/Button.vue'

const app = useApp()
const route = useRoute()
const router = useRouter()
const username = ref('')
const password = ref('')
const remember = ref(true)
const loading = ref(false)
const error = ref('')

async function submit() {
  error.value = ''
  loading.value = true
  try {
    await app.login(username.value, password.value, remember.value)
    const next = typeof route.query.next === 'string' && route.query.next.startsWith('/') ? route.query.next : '/'
    router.replace(next)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <AuthShell :title="`登录 ${app.settings?.siteTitle ?? 'LanPanel'}`" subtitle="管理面板、设备与扫描规则">
    <form class="flex flex-col gap-4" @submit.prevent="submit">
      <Field label="用户名" for="lp-user">
        <Input id="lp-user" v-model="username" autocomplete="username" autofocus />
      </Field>
      <Field label="密码" for="lp-pass">
        <Input id="lp-pass" v-model="password" type="password" autocomplete="current-password" />
      </Field>
      <label class="flex cursor-pointer items-center gap-2 text-[13px] text-fg-2">
        <input v-model="remember" type="checkbox" class="size-4 accent-[var(--accent)]" />30 天内保持登录
      </label>
      <p v-if="error" class="m-0 rounded-sm bg-danger-soft px-3 py-2 text-[13px] text-danger-fg" role="alert">{{ error }}</p>
      <Button type="submit" variant="primary" :loading="loading" class="!h-10 !text-sm">登录</Button>
      <RouterLink v-if="app.settings?.publicPanel" to="/" class="self-center text-[13px] text-accent-soft-fg hover:underline">以访客身份浏览面板</RouterLink>
    </form>
  </AuthShell>
</template>
