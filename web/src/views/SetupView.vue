<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useApp } from '@/stores/app'
import AuthShell from './AuthShell.vue'
import Field from '@/components/ui/Field.vue'
import Input from '@/components/ui/Input.vue'
import Button from '@/components/ui/Button.vue'

const app = useApp()
const router = useRouter()
const username = ref('admin')
const password = ref('')
const confirmPw = ref('')
const loading = ref(false)
const error = ref('')

async function submit() {
  error.value = ''
  if (password.value.length < 6) return (error.value = '密码至少 6 位')
  if (password.value !== confirmPw.value) return (error.value = '两次输入的密码不一致')
  loading.value = true
  try {
    await app.setup(username.value.trim(), password.value)
    router.replace('/')
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <AuthShell title="欢迎使用 LanPanel" subtitle="首次使用，请创建管理员账号">
    <form class="flex flex-col gap-4" @submit.prevent="submit">
      <Field label="用户名" for="lp-user">
        <Input id="lp-user" v-model="username" autocomplete="username" />
      </Field>
      <Field label="密码" for="lp-pass" hint="至少 6 位">
        <Input id="lp-pass" v-model="password" type="password" autocomplete="new-password" autofocus />
      </Field>
      <Field label="确认密码" for="lp-pass2">
        <Input id="lp-pass2" v-model="confirmPw" type="password" autocomplete="new-password" />
      </Field>
      <p v-if="error" class="m-0 rounded-sm bg-danger-soft px-3 py-2 text-[13px] text-danger-fg" role="alert">{{ error }}</p>
      <Button type="submit" variant="primary" :loading="loading" class="!h-10 !text-sm">创建并进入</Button>
    </form>
  </AuthShell>
</template>
