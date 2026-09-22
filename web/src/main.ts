import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { onUnauthorized } from './lib/api'
import { useApp } from './stores/app'
import './styles/main.css'

const app = createApp(App)
app.use(createPinia())
app.use(router)

// 会话失效：清空用户并跳转登录（访客模式下的面板页除外）
onUnauthorized(() => {
  const s = useApp()
  s.user = null
  const r = router.currentRoute.value
  if (r.name !== 'login' && (r.meta.admin || !s.settings?.publicPanel)) {
    router.replace({ name: 'login', query: { next: r.fullPath } })
  }
})

app.mount('#app')
