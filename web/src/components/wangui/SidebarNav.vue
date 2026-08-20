<script setup lang="ts">
import type { FunctionalComponent } from 'vue'

export interface NavItem {
  to: string
  label: string
  icon: FunctionalComponent
}

defineProps<{ items: NavItem[] }>()
</script>

<template>
  <nav class="space-y-1">
    <RouterLink
      v-for="item in items"
      :key="item.to"
      v-slot="{ isExactActive }"
      :to="item.to"
      custom
    >
      <a
        :href="item.to"
        class="group flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-all" :class="[
          isExactActive
            ? 'bg-emerald-500/15 text-emerald-300 ring-1 ring-emerald-500/25'
            : 'text-zinc-500 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-100 hover:bg-black/5 dark:hover:bg-white/5',
        ]"
        @click.prevent="$router.push(item.to)"
      >
        <component
          :is="item.icon"
          class="h-4 w-4 transition-colors"
          :class="isExactActive ? 'text-emerald-400' : 'text-zinc-500 group-hover:text-zinc-400 dark:text-zinc-300'"
        />
        <span class="font-medium">{{ item.label }}</span>
      </a>
    </RouterLink>
  </nav>
</template>
