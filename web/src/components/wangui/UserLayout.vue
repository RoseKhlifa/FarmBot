<script setup lang="ts">
import type { NavItem } from './SidebarNav.vue'
import {
  Activity,
  BarChart3,
  BookOpen,
  Gift,
  LayoutDashboard,
  LogOut,
  Megaphone,
  Settings,
  ShoppingBag,
  User as UserIcon,
  Users,
} from 'lucide-vue-next'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { systemApi } from '@/api'
import { useAccountStore } from '@/stores/account'
import { useToastStore } from '@/stores/toast'
import { useUserStore } from '@/stores/user'
import AnnouncementCard from './AnnouncementCard.vue'
import Avatar from './Avatar.vue'
import Logo from './Logo.vue'
import SidebarNav from './SidebarNav.vue'
import ThemeToggle from './ThemeToggle.vue'

interface FarmAnnouncement {
  id: string | number
  title: string
  content: string
  level: 'info' | 'success' | 'warning' | 'critical'
  createdAt: number | string
  expiresAt?: number | string | null
}

const router = useRouter()
const userStore = useUserStore()
const accountStore = useAccountStore()
const toast = useToastStore()

const items: NavItem[] = [
  { to: '/', label: '工作台', icon: LayoutDashboard },
  { to: '/personal', label: '账号中心', icon: UserIcon },
  { to: '/friends', label: '好友互动', icon: Users },
  { to: '/activity', label: '限时活动', icon: Gift },
  { to: '/shop', label: '农场商城', icon: ShoppingBag },
  { to: '/illustrated', label: '作物图鉴', icon: BookOpen },
  { to: '/analytics', label: '运行分析', icon: BarChart3 },
  { to: '/settings', label: '系统设置', icon: Settings },
]
const navItems = computed(() => userStore.isAdmin
  ? [...items, { to: '/admin', label: '管理后台', icon: Activity }]
  : items)
const mobilePrimaryItems = items.slice(0, 4)
const mobileMoreItems = computed(() => navItems.value.filter(item => !mobilePrimaryItems.some(primary => primary.to === item.to)))
const mobileMenuOpen = ref(false)

const announcements = ref<FarmAnnouncement[]>([])
async function loadAnnouncements() {
  try {
    const response = await systemApi.getAnnouncement<any>()
    const payload = response.data?.data
    const list = Array.isArray(payload) ? payload : payload?.announcements || payload?.items || []
    announcements.value = list.filter((item: any) => item?.title && item?.content).map((item: any, index: number) => ({
      id: item.id ?? index,
      title: String(item.title),
      content: String(item.content),
      level: ['info', 'success', 'warning', 'critical'].includes(item.level) ? item.level : 'info',
      createdAt: item.createdAt ?? Date.now(),
      expiresAt: item.expiresAt ?? null,
    }))
  }
  catch {
    announcements.value = []
  }
}

const runningAccounts = computed(() => accountStore.accounts.filter(account => account.running).length)
let pollHandle: number | undefined

async function logout() {
  try {
    await userStore.logout()
  }
  finally {
    toast.success('已退出登录')
    router.push('/login')
  }
}

onMounted(() => {
  accountStore.fetchAccounts()
  userStore.fetchUserInfo()
  loadAnnouncements()
  pollHandle = window.setInterval(() => {
    loadAnnouncements()
    accountStore.fetchAccounts()
  }, 60_000)
})
onUnmounted(() => {
  if (pollHandle)
    clearInterval(pollHandle)
})
watch(() => router.currentRoute.value.fullPath, () => {
  mobileMenuOpen.value = false
})
</script>

<template>
  <div class="relative mx-auto max-w-[1700px] min-h-screen flex flex-col bg-white md:flex-row dark:bg-zinc-950">
    <!-- Sidebar (desktop, sticky below the global banner) -->
    <aside
      class="sticky top-0 hidden h-screen w-64 shrink-0 flex-col overflow-y-auto border-r border-black/[0.06] bg-white/60 backdrop-blur-xl md:flex dark:border-white/[0.05] dark:bg-zinc-950/60"
    >
      <div class="border-b border-black/[0.05] px-5 py-5 dark:border-white/[0.04]">
        <Logo :size="34" text="FarmBot" subtitle="农场自动化工作台" />
        <div
          v-if="accountStore.accounts.length > 0"
          class="mt-3 inline-flex items-center gap-1.5 rounded-md from-emerald-500/10 to-amber-500/10 bg-gradient-to-r px-2 py-1 text-[11px] text-zinc-700 ring-1 ring-emerald-500/20 dark:text-zinc-300"
          :title="`当前已接入 ${accountStore.accounts.length} 个农场账号，${runningAccounts} 个正在运行`"
        >
          <Activity class="h-3 w-3 text-emerald-400" />
          <span>运行中</span>
          <span class="font-mono-token text-emerald-600 font-semibold tabular-nums dark:text-emerald-300">
            {{ runningAccounts }}
          </span>
          <span>/ {{ accountStore.accounts.length }} 个账号</span>
        </div>
      </div>

      <div class="px-3 py-5">
        <p class="mb-2 px-3 text-[10px] text-zinc-500 font-medium tracking-wider uppercase dark:text-zinc-600">
          导航
        </p>
        <SidebarNav :items="navItems" />
      </div>

      <!-- Spacer that pushes announcements + user-info to the bottom. -->
      <div class="flex-1" />

      <!-- Announcements (sidebar slot): sits just above the user-info block.
           Compact card variant so titles + a short body fit the narrow
           sidebar; scrollable if there are many. -->
      <div
        v-if="announcements.length > 0"
        class="max-h-[40vh] overflow-y-auto border-t border-black/[0.05] px-3 py-3 space-y-1.5 dark:border-white/[0.04]"
      >
        <p class="mb-1 flex items-center gap-1 px-1 text-[10px] text-zinc-500 font-medium tracking-wider uppercase dark:text-zinc-600">
          <Megaphone class="h-3 w-3" />
          公告 · {{ announcements.length }}
        </p>
        <AnnouncementCard
          v-for="a in announcements"
          :key="a.id"
          :a="a"
          :compact="true"
        />
      </div>

      <div class="border-t border-black/[0.05] px-3 py-3 space-y-1 dark:border-white/[0.04]">
        <div
          v-if="userStore.userInfo"
          class="flex items-center gap-3 rounded-lg px-3 py-2"
        >
          <Avatar
            :src="userStore.avatar"
            :name="userStore.username"
            :size="36"
            rounded="lg"
          />
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm text-zinc-900 font-medium dark:text-zinc-200">
              {{ userStore.username }}
            </p>
            <p class="font-mono-token truncate text-[11px] text-zinc-500">
              {{ userStore.expireTimeText }}
            </p>
          </div>
          <ThemeToggle />
        </div>
        <button
          class="w-full flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-zinc-500 transition-colors hover:bg-black/5 dark:text-zinc-400 hover:text-zinc-900 dark:hover:bg-white/5 dark:hover:text-zinc-100"
          @click="logout"
        >
          <LogOut class="h-4 w-4 text-zinc-500" />
          <span>退出</span>
        </button>
      </div>
    </aside>

    <!-- Mobile top bar (sits below the global banner) -->
    <header class="sticky top-0 z-30 h-12 flex items-center justify-between border-b border-black/[0.08] bg-white/85 px-4 backdrop-blur-xl md:hidden dark:border-white/[0.06] dark:bg-zinc-950/85">
      <Logo :size="26" text="FarmBot" />
      <div class="flex items-center gap-1">
        <ThemeToggle />
        <button
          class="rounded-lg p-2 text-zinc-500 hover:bg-black/5 dark:text-zinc-400 hover:text-zinc-900 dark:hover:bg-white/5 dark:hover:text-zinc-100"
          @click="logout"
        >
          <LogOut class="h-4 w-4" />
        </button>
      </div>
    </header>

    <!-- Main — fills the space right of the sidebar, with inner padding -->
    <main class="relative min-w-0 flex-1">
      <!-- Mobile-only announcement strip. The sidebar slot is desktop-only
           (sidebar is hidden under md:); on mobile we render the same
           cards in the main flow so users on phones still see notices. -->
      <div
        v-if="announcements.length > 0"
        class="px-3 pt-3 md:hidden space-y-2"
      >
        <AnnouncementCard
          v-for="a in announcements"
          :key="a.id"
          :a="a"
        />
      </div>

      <div class="px-3 py-4 pb-24 lg:px-14 md:px-10 md:py-8 sm:px-6 sm:py-6 md:pb-16">
        <RouterView v-slot="{ Component }">
          <Transition name="fade" mode="out-in">
            <component :is="Component" />
          </Transition>
        </RouterView>
      </div>

      <!-- Mobile bottom nav -->
      <nav class="fixed inset-x-0 bottom-0 z-30 grid grid-cols-5 border-t border-black/[0.08] bg-white/90 py-2 backdrop-blur-xl md:hidden dark:border-white/[0.06] dark:bg-zinc-950/90">
        <RouterLink
          v-for="item in mobilePrimaryItems"
          :key="item.to"
          v-slot="{ isExactActive }"
          :to="item.to"
          custom
        >
          <a
            :href="item.to"
            class="flex flex-col items-center gap-0.5 rounded-lg px-3 py-1.5"
            :class="isExactActive ? 'text-emerald-400' : 'text-zinc-500'"
            @click.prevent="$router.push(item.to)"
          >
            <component :is="item.icon" class="h-5 w-5" />
            <span class="text-[10px]">{{ item.label }}</span>
          </a>
        </RouterLink>
        <button
          type="button"
          class="flex flex-col items-center gap-0.5 rounded-lg px-3 py-1.5 text-zinc-500"
          :class="mobileMenuOpen ? 'text-emerald-400' : 'text-zinc-500'"
          @click="mobileMenuOpen = !mobileMenuOpen"
        >
          <Settings class="h-5 w-5" />
          <span class="text-[10px]">更多</span>
        </button>
      </nav>

      <Transition name="mobile-sheet">
        <div v-if="mobileMenuOpen" class="fixed inset-0 z-40 md:hidden" @click.self="mobileMenuOpen = false">
          <div class="absolute inset-0 bg-black/25 backdrop-blur-[2px]" aria-hidden="true" />
          <div class="absolute inset-x-0 bottom-0 border-t border-black/[0.08] rounded-t-2xl bg-white px-4 pb-24 pt-4 shadow-2xl dark:border-white/[0.06] dark:bg-zinc-900">
            <div class="mx-auto mb-3 h-1 w-10 rounded-full bg-zinc-200 dark:bg-zinc-700" />
            <div class="mb-3 flex items-center justify-between">
              <div>
                <p class="text-sm text-zinc-900 font-semibold dark:text-zinc-100">
                  更多功能
                </p>
                <p class="mt-0.5 text-[11px] text-zinc-500">
                  进入完整管理与数据视图
                </p>
              </div>
              <button type="button" class="rounded-lg p-2 text-zinc-500 hover:bg-black/5 dark:hover:bg-white/5" aria-label="关闭更多功能" @click="mobileMenuOpen = false">
                <span class="i-carbon-close text-lg" />
              </button>
            </div>
            <div class="grid grid-cols-2 gap-2">
              <RouterLink
                v-for="item in mobileMoreItems"
                :key="item.to"
                :to="item.to"
                class="min-h-16 flex items-center gap-3 border border-black/[0.06] rounded-xl px-3 text-zinc-700 transition-colors dark:border-white/[0.06] hover:bg-emerald-50 dark:text-zinc-200 dark:hover:bg-emerald-950/30"
                @click="mobileMenuOpen = false"
              >
                <component :is="item.icon" class="h-5 w-5 text-emerald-500" />
                <span class="text-xs font-medium">{{ item.label }}</span>
              </RouterLink>
            </div>
          </div>
        </div>
      </Transition>
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
