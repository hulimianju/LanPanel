<script setup lang="ts">
import Toaster from '@/components/ui/Toaster.vue'
import ConfirmHost from '@/components/ui/ConfirmHost.vue'
</script>

<template>
  <RouterView />
  <Toaster />
  <ConfirmHost />
</template>
