<script setup lang="ts">
import type { NavItem } from './SidebarNav.vue'
import {
  Cog,
  LayoutDashboard,
  LogOut,
  ScrollText,
  ShieldCheck,
  Ticket,
  Users,
} from 'lucide-vue-next'
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useToastStore } from '@/stores/toast'
import { useUserStore } from '@/stores/user'
import Logo from './Logo.vue'
import SidebarNav from './SidebarNav.vue'
import ThemeToggle from './ThemeToggle.vue'

const router = useRouter()
const userStore = useUserStore()
const toast = useToastStore()

const items: NavItem[] = [
  { to: '/admin', label: '运营概览', icon: LayoutDashboard },
  { to: '/admin?tab=card', label: '卡密管理', icon: Ticket },
  { to: '/admin?tab=user', label: '用户管理', icon: Users },
  { to: '/admin?tab=log', label: '登录日志', icon: ScrollText },
  { to: '/admin?tab=system', label: '系统配置', icon: Cog },
]

async function logout() {
  try {
    await userStore.logout()
  }
  finally {
    toast.success('已退出管理员')
    router.push('/login')
  }
}

onMounted(() => {
  userStore.fetchUserInfo()
})
</script>

<template>
  <div class="relative mx-auto max-w-[1700px] min-h-screen flex flex-col bg-white md:flex-row dark:bg-zinc-950">
    <aside class="sticky top-0 hidden h-screen w-64 shrink-0 flex-col overflow-y-auto border-r border-amber-500/10 bg-white/60 backdrop-blur-xl md:flex dark:bg-zinc-950/60">
      <div class="flex items-center justify-between gap-2 border-b border-black/[0.05] px-5 py-5 dark:border-white/[0.04]">
        <Logo :size="34" text="FarmBot" subtitle="运营管理后台" />
        <span class="inline-flex shrink-0 items-center gap-1 rounded-md bg-amber-500/15 px-2 py-0.5 text-[10px] text-amber-400 font-medium ring-1 ring-amber-500/30">
          <ShieldCheck class="h-2.5 w-2.5" />
          ADMIN
        </span>
      </div>

      <div class="flex-1 px-3 py-5">
        <p class="mb-2 px-3 text-[10px] text-zinc-500 font-medium tracking-wider uppercase dark:text-zinc-600">
          管理
        </p>
        <SidebarNav :items="items" />
      </div>

      <div class="flex items-center gap-1 border-t border-black/[0.05] px-3 py-3 dark:border-white/[0.04]">
        <button
          class="flex flex-1 items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-zinc-500 transition-colors hover:bg-black/5 dark:text-zinc-400 hover:text-zinc-900 dark:hover:bg-white/5 dark:hover:text-zinc-100"
          @click="logout"
        >
          <LogOut class="h-4 w-4 text-zinc-500" />
          <span>退出管理员</span>
        </button>
        <ThemeToggle />
      </div>
    </aside>

    <header class="sticky top-0 z-30 h-12 flex items-center justify-between border-b border-black/[0.08] bg-white/85 px-4 backdrop-blur-xl md:hidden dark:border-white/[0.06] dark:bg-zinc-950/85">
      <Logo :size="26" text="FarmBot 管理" />
      <div class="flex items-center gap-2">
        <ThemeToggle />
        <span class="inline-flex items-center gap-1 rounded-md bg-amber-500/15 px-2 py-0.5 text-[10px] text-amber-400 font-medium ring-1 ring-amber-500/30">
          <ShieldCheck class="h-2.5 w-2.5" />
          ADMIN
        </span>
      </div>
    </header>

    <main class="relative min-w-0 flex-1">
      <div class="px-3 py-4 pb-24 lg:px-14 md:px-10 md:py-8 sm:px-6 sm:py-6 md:pb-16">
        <RouterView v-slot="{ Component }">
          <Transition name="fade" mode="out-in">
            <component :is="Component" />
          </Transition>
        </RouterView>
      </div>

      <nav class="fixed inset-x-0 bottom-0 z-30 flex justify-around border-t border-black/[0.08] bg-white/90 py-2 backdrop-blur-xl md:hidden dark:border-white/[0.06] dark:bg-zinc-950/90">
        <RouterLink
          v-for="item in items"
          :key="item.to"
          v-slot="{ isExactActive }"
          :to="item.to"
          custom
        >
          <a
            :href="item.to"
            class="flex flex-col items-center gap-0.5 rounded-lg px-2 py-1.5"
            :class="isExactActive ? 'text-amber-400' : 'text-zinc-500'"
            @click.prevent="$router.push(item.to)"
          >
            <component :is="item.icon" class="h-5 w-5" />
            <span class="text-[10px]">{{ item.label }}</span>
          </a>
        </RouterLink>
      </nav>
    </main>
  </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition:
    opacity 0.15s ease,
    transform 0.15s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(4px);
}
</style>
