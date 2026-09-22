<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'

const props = defineProps<{ seconds: boolean; subtitle?: string }>()
const now = ref(new Date())
const timer = setInterval(() => (now.value = new Date()), 1000)
onBeforeUnmount(() => clearInterval(timer))

const pad = (n: number) => String(n).padStart(2, '0')
const time = computed(() => {
  const d = now.value
  return `${pad(d.getHours())}:${pad(d.getMinutes())}` + (props.seconds ? `:${pad(d.getSeconds())}` : '')
})
const WEEK = ['日', '一', '二', '三', '四', '五', '六']
const date = computed(() => `${now.value.getMonth() + 1}月${now.value.getDate()}日 星期${WEEK[now.value.getDay()]}`)
</script>

<template>
  <div class="lp-text-shadow flex flex-col items-center gap-1.5 text-center">
    <time class="text-[64px] font-light leading-none tracking-[-0.04em] tabular-nums text-s-fg sm:text-[88px]" :datetime="now.toISOString()">{{ time }}</time>
    <p class="m-0 text-sm text-s-fg-2">
      {{ date }}<template v-if="subtitle"> · {{ subtitle }}</template>
    </p>
  </div>
</template>
