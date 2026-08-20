<script setup lang="ts">
import { Moon, Sun } from 'lucide-vue-next'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()
</script>

<template>
  <button
    :title="appStore.isDark ? '切换浅色' : '切换深色'"
    class="h-8 w-8 inline-flex items-center justify-center rounded-lg text-zinc-500 transition-colors hover:bg-black/5 hover:text-zinc-900 dark:hover:bg-white/5 dark:hover:text-zinc-100"
    @click="appStore.toggleDark()"
  >
    <Sun v-if="appStore.isDark" class="h-4 w-4" />
    <Moon v-else class="h-4 w-4" />
  </button>
</template>
