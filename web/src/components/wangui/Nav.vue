<script setup lang="ts">
import { ChevronDown, LogOut } from 'lucide-vue-next'
import { onMounted, onUnmounted, ref } from 'vue'
import Logo from './Logo.vue'

defineProps<{ name: string }>()
defineEmits<{ (e: 'logout'): void }>()

const menuOpen = ref(false)
const menuRef = ref<HTMLElement | null>(null)

function onDocClick(e: MouseEvent) {
  if (menuRef.value && !menuRef.value.contains(e.target as Node)) {
    menuOpen.value = false
  }
}

onMounted(() => document.addEventListener('mousedown', onDocClick))
onUnmounted(() => document.removeEventListener('mousedown', onDocClick))
</script>

<template>
  <header class="sticky top-0 z-40 border-b border-black/[0.08] bg-white/70 backdrop-blur-xl dark:border-white/[0.06] dark:bg-zinc-950/70">
    <div class="mx-auto h-14 max-w-3xl flex items-center justify-between px-4">
      <Logo :size="28" />

      <div ref="menuRef" class="relative">
        <button
          class="flex items-center gap-2 rounded-lg py-1 pl-1 pr-2 transition-colors hover:bg-black/5 dark:hover:bg-white/5"
          @click="menuOpen = !menuOpen"
        >
          <span class="h-7 w-7 flex items-center justify-center rounded-md bg-emerald-500/15 text-xs text-emerald-400 font-semibold ring-1 ring-emerald-500/30">
            {{ name?.[0] || '?' }}
          </span>
          <span class="text-sm text-zinc-900 dark:text-zinc-200">{{ name }}</span>
          <ChevronDown
            class="h-3.5 w-3.5 text-zinc-500 transition-transform"
            :class="menuOpen ? 'rotate-180' : ''"
          />
        </button>

        <Transition name="menu">
          <div
            v-if="menuOpen"
            class="absolute right-0 top-full mt-2 w-44 overflow-hidden rounded-xl bg-zinc-100 shadow-2xl ring-1 ring-black/10 dark:bg-zinc-900 dark:ring-white/10"
          >
            <button
              class="w-full flex items-center gap-2 px-3 py-2.5 text-sm text-zinc-700 transition-colors hover:bg-black/5 dark:text-zinc-300 dark:hover:bg-white/5"
              @click="$emit('logout'); menuOpen = false"
            >
              <LogOut class="h-4 w-4 text-zinc-500" />
              <span>退出登录</span>
            </button>
          </div>
        </Transition>
      </div>
    </div>
  </header>
</template>

<style scoped>
.menu-enter-active,
.menu-leave-active {
  transition: all 0.15s ease;
}
.menu-enter-from,
.menu-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
