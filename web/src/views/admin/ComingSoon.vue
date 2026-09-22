<script setup lang="ts">
import { useRoute } from 'vue-router'
import { Radar } from 'lucide-vue-next'
const route = useRoute()
</script>

<template>
  <div class="flex flex-col gap-5">
    <h1 class="m-0 text-[22px] font-semibold tracking-[-0.01em]">{{ route.meta.title }}</h1>
    <div class="flex flex-col items-center gap-3 rounded-lg border border-line bg-surface px-8 py-16 text-center">
      <Radar class="size-8 text-fg-3" />
      <p class="m-0 text-[15px] font-medium">即将推出</p>
      <p class="m-0 max-w-sm text-[13px] text-fg-3">局域网设备发现（ARP、mDNS、SSDP、NetBIOS、端口指纹）将在阶段 2 完成。</p>
    </div>
  </div>
</template>
