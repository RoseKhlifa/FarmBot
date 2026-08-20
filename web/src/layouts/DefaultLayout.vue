<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import MysteryMerchantBanner from '@/components/shop/MysteryMerchantBanner.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'
import TopAccountMenu from '@/components/TopAccountMenu.vue'
import { menuRoutes } from '@/router/menu'
import { useAccountStore } from '@/stores/account'
import { useStatusStore } from '@/stores/status'
import { useUserStore } from '@/stores/user'

const route = useRoute()
const router = useRouter()
const accountStore = useAccountStore()
const statusStore = useStatusStore()
const userStore = useUserStore()
const sidebarCollapsed = ref(false)
const mobileMenuOpen = ref(false)
const navItems = computed(() => menuRoutes.filter(item => !item.adminOnly || userStore.isAdmin))
const primaryMobileItems = computed(() => navItems.value.filter(item => ['dashboard', 'personal', 'friends', 'activity'].includes(item.name)).slice(0, 4))
const activeItem = computed(() => navItems.value.find(item => route.name === item.name) || navItems.value[0])
const pageTitle = computed(() => activeItem.value?.label || '工作台')
const pageSubtitle = computed(() => ({
  dashboard: '账号运行概况与农场任务',
  personal: '管理当前账号的个人资产',
  friends: '好友互动、黑名单与访客',
  activity: '查看正在进行的限时活动',
  shop: '种子、道具与限时商品',
  illustrated: '收集进度与物品图鉴',
  analytics: '运行数据与趋势分析',
  settings: '自动化规则与系统设置',
  admin: '系统运营与账号管理',
}[activeItem.value?.name || ''] || 'FarmBot operations'))
const accountCount = computed(() => accountStore.accounts.length)
const onlineCount = computed(() => accountStore.accounts.filter(account => account.running).length)

onMounted(() => accountStore.fetchAccounts())
function closeMobileMenu() {
  mobileMenuOpen.value = false
}
function goTo(path: string) {
  closeMobileMenu()
  router.push(path)
}
</script>

<template>
  <div class="workbench" :class="{ 'workbench--collapsed': sidebarCollapsed }">
    <aside class="workbench-sidebar" :class="{ 'workbench-sidebar--collapsed': sidebarCollapsed }">
      <div class="sidebar-brand">
        <RouterLink to="/" class="brand-lockup" aria-label="返回首页">
          <span class="brand-mark"><span class="i-carbon-sprout" /></span>
          <span v-if="!sidebarCollapsed" class="brand-copy"><strong>FarmBot</strong><small>OPERATIONS</small></span>
        </RouterLink>
        <button class="icon-button sidebar-collapse" :aria-label="sidebarCollapsed ? '展开侧栏' : '收起侧栏'" :title="sidebarCollapsed ? '展开侧栏' : '收起侧栏'" @click="sidebarCollapsed = !sidebarCollapsed">
          <span :class="sidebarCollapsed ? 'i-carbon-chevron-right' : 'i-carbon-chevron-left'" />
        </button>
      </div>
      <div v-if="!sidebarCollapsed" class="sidebar-context">
        <span class="status-dot" :class="statusStore.realtimeConnected ? 'status-dot--live' : 'status-dot--idle'" /><span>{{ statusStore.realtimeConnected ? '实时连接正常' : '等待实时连接' }}</span>
      </div>
      <nav class="sidebar-nav" aria-label="主导航">
        <p v-if="!sidebarCollapsed" class="nav-section-label">
          工作区
        </p>
        <RouterLink v-for="item in navItems" :key="item.name" :to="item.path ? `/${item.path}` : '/'" class="sidebar-nav-item" :class="{ 'sidebar-nav-item--active': route.name === item.name }" :title="sidebarCollapsed ? item.label : undefined" @click="closeMobileMenu">
          <span class="sidebar-nav-icon" :class="item.icon" /><span v-if="!sidebarCollapsed" class="sidebar-nav-label">{{ item.label }}</span><span v-if="!sidebarCollapsed && route.name === item.name" class="sidebar-nav-active-bar" />
        </RouterLink>
      </nav>
      <div class="sidebar-footer">
        <div v-if="!sidebarCollapsed" class="sidebar-account-summary">
          <span class="sidebar-account-icon i-carbon-user-avatar" /><span><strong>{{ accountCount }} 个账号</strong><small>{{ onlineCount }} 个在线</small></span>
        </div>
        <button class="sidebar-footer-link" :title="sidebarCollapsed ? '帮助与反馈' : undefined" @click="goTo('/settings')">
          <span class="i-carbon-help" /><span v-if="!sidebarCollapsed">帮助与反馈</span>
        </button>
      </div>
    </aside>

    <div class="workbench-main">
      <header class="workbench-topbar">
        <div class="topbar-leading">
          <button class="icon-button mobile-menu-button" aria-label="打开导航" title="打开导航" @click="mobileMenuOpen = true">
            <span class="i-carbon-menu" />
          </button>
          <div class="page-heading">
            <span class="page-kicker">FARMBOT / {{ activeItem?.name?.toUpperCase() }}</span><h1>{{ pageTitle }}</h1><p>{{ pageSubtitle }}</p>
          </div>
        </div>
        <div class="topbar-actions">
          <div class="topbar-runtime">
            <span class="status-dot" :class="statusStore.realtimeConnected ? 'status-dot--live' : 'status-dot--idle'" /><span class="topbar-runtime-label">{{ statusStore.realtimeConnected ? 'LIVE' : 'OFFLINE' }}</span>
          </div><ThemeToggle /><TopAccountMenu />
        </div>
      </header>
      <MysteryMerchantBanner />
      <main class="workbench-content custom-scrollbar">
        <RouterView v-slot="{ Component, route: currentRoute }">
          <component :is="Component" :key="currentRoute.path" />
        </RouterView>
      </main>
    </div>

    <nav class="mobile-nav" aria-label="移动端主导航">
      <RouterLink v-for="item in primaryMobileItems" :key="item.name" :to="item.path ? `/${item.path}` : '/'" class="mobile-nav-item" :class="{ 'mobile-nav-item--active': route.name === item.name }">
        <span :class="item.icon" /><span>{{ item.label }}</span>
      </RouterLink>
      <button class="mobile-nav-item" :class="{ 'mobile-nav-item--active': mobileMenuOpen }" @click="mobileMenuOpen = true">
        <span class="i-carbon-menu" /><span>更多</span>
      </button>
    </nav>

    <Transition name="mobile-sheet">
      <div v-if="mobileMenuOpen" class="mobile-sheet-backdrop" @click.self="closeMobileMenu">
        <aside class="mobile-sheet" aria-label="全部导航">
          <div class="mobile-sheet-header">
            <div><span class="page-kicker">FARMBOT</span><h2>全部模块</h2></div><button class="icon-button" aria-label="关闭导航" title="关闭导航" @click="closeMobileMenu">
              <span class="i-carbon-close" />
            </button>
          </div><div class="mobile-sheet-grid">
            <button v-for="item in navItems" :key="item.name" class="mobile-sheet-item" @click="goTo(item.path ? `/${item.path}` : '/')">
              <span class="mobile-sheet-icon" :class="item.icon" /><span>{{ item.label }}</span><span class="mobile-sheet-arrow i-carbon-arrow-right" />
            </button>
          </div>
        </aside>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.workbench {
  --sidebar-width: 240px;
  display: flex;
  width: 100%;
  height: 100dvh;
  overflow: hidden;
  background: var(--app-bg);
  color: var(--theme-text);
}
.workbench-sidebar {
  width: var(--sidebar-width);
  display: flex;
  flex: 0 0 var(--sidebar-width);
  flex-direction: column;
  border-right: 1px solid var(--surface-border);
  background: var(--surface-1);
  transition:
    width 0.2s ease,
    flex-basis 0.2s ease;
}
.workbench-sidebar--collapsed,
.workbench--collapsed .workbench-sidebar {
  width: 76px;
  flex-basis: 76px;
}
.sidebar-brand {
  min-height: 76px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 16px;
  border-bottom: 1px solid var(--surface-border);
}
.brand-lockup {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--theme-text);
  text-decoration: none;
}
.brand-mark {
  width: 36px;
  height: 36px;
  display: grid;
  flex: 0 0 auto;
  place-items: center;
  border: 1px solid color-mix(in srgb, var(--theme-primary) 32%, var(--surface-border));
  border-radius: 10px;
  background: color-mix(in srgb, var(--theme-primary) 10%, var(--surface-1));
  color: var(--theme-primary);
  font-size: 19px;
}
.brand-copy {
  display: flex;
  min-width: 0;
  flex-direction: column;
  line-height: 1.15;
}
.brand-copy strong {
  font-size: 15px;
  letter-spacing: 0.02em;
}
.brand-copy small {
  margin-top: 4px;
  color: var(--muted-text);
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.16em;
}
.icon-button {
  width: 34px;
  height: 34px;
  display: inline-grid;
  flex: 0 0 auto;
  place-items: center;
  border: 1px solid var(--surface-border);
  border-radius: 8px;
  background: transparent;
  color: var(--muted-text);
  cursor: pointer;
  transition: 0.18s ease;
}
.icon-button:hover {
  border-color: var(--surface-border-strong);
  background: var(--surface-2);
  color: var(--theme-text);
}
.sidebar-context {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 14px 16px 4px;
  color: var(--muted-text);
  font-size: 11px;
  font-weight: 600;
}
.status-dot {
  width: 7px;
  height: 7px;
  flex: 0 0 auto;
  border-radius: 50%;
  background: currentColor;
}
.status-dot--live {
  color: #22a06b;
  box-shadow: 0 0 0 3px color-mix(in srgb, #22a06b 14%, transparent);
}
.status-dot--idle {
  color: #a5adab;
}
.sidebar-nav {
  flex: 1;
  overflow-y: auto;
  padding: 20px 10px;
}
.nav-section-label {
  margin: 0 10px 9px;
  color: var(--muted-text);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.13em;
  text-transform: uppercase;
}
.sidebar-nav-item {
  position: relative;
  min-height: 42px;
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 3px 0;
  padding: 0 12px;
  border: 1px solid transparent;
  border-radius: 8px;
  color: var(--muted-text);
  font-size: 13px;
  font-weight: 600;
  text-decoration: none;
  transition: 0.18s ease;
}
.workbench-sidebar--collapsed .sidebar-nav-item {
  justify-content: center;
  padding: 0;
}
.sidebar-nav-item:hover {
  background: var(--surface-2);
  color: var(--theme-text);
}
.sidebar-nav-item--active {
  border-color: color-mix(in srgb, var(--theme-primary) 18%, transparent);
  background: color-mix(in srgb, var(--theme-primary) 10%, var(--surface-1));
  color: var(--theme-primary);
}
.sidebar-nav-icon {
  width: 18px;
  height: 18px;
  flex: 0 0 auto;
  font-size: 17px;
}
.sidebar-nav-active-bar {
  position: absolute;
  top: 9px;
  right: 0;
  bottom: 9px;
  width: 3px;
  border-radius: 3px 0 0 3px;
  background: var(--theme-primary);
}
.sidebar-footer {
  padding: 12px;
  border-top: 1px solid var(--surface-border);
}
.sidebar-account-summary {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px;
  border-radius: 8px;
  background: var(--surface-2);
}
.sidebar-account-icon {
  color: var(--theme-primary);
  font-size: 18px;
}
.sidebar-account-summary span:last-child {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 3px;
}
.sidebar-account-summary strong {
  color: var(--theme-text);
  font-size: 11px;
}
.sidebar-account-summary small {
  color: var(--muted-text);
  font-size: 10px;
}
.sidebar-footer-link {
  width: 100%;
  min-height: 36px;
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 7px;
  padding: 0 10px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--muted-text);
  cursor: pointer;
  font-size: 12px;
  text-align: left;
}
.sidebar-footer-link:hover {
  background: var(--surface-2);
  color: var(--theme-text);
}
.workbench-main {
  min-width: 0;
  min-height: 0;
  display: flex;
  flex: 1;
  flex-direction: column;
}
.workbench-topbar {
  min-height: 76px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding: 14px 28px;
  border-bottom: 1px solid var(--surface-border);
  background: var(--surface-1);
}
.topbar-leading,
.topbar-actions {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 16px;
}
.page-heading {
  min-width: 0;
}
.page-kicker {
  display: block;
  color: var(--muted-text);
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.15em;
  line-height: 1.2;
  text-transform: uppercase;
}
.page-heading h1 {
  margin: 5px 0 0;
  color: var(--theme-text);
  font-size: 20px;
  font-weight: 750;
  line-height: 1.2;
}
.page-heading p {
  margin: 4px 0 0;
  color: var(--muted-text);
  font-size: 11px;
}
.topbar-runtime {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--muted-text);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.08em;
}
.workbench-content {
  min-height: 0;
  flex: 1;
  overflow-x: hidden;
  overflow-y: auto;
  padding: 24px 28px 32px;
}
.mobile-menu-button,
.mobile-nav,
.mobile-sheet-backdrop {
  display: none;
}
@media (max-width: 900px) {
  .workbench-sidebar {
    width: 76px;
    flex-basis: 76px;
  }
  .brand-copy,
  .sidebar-context,
  .nav-section-label,
  .sidebar-nav-label,
  .sidebar-account-summary,
  .sidebar-footer-link span:last-child {
    display: none;
  }
  .sidebar-brand {
    justify-content: center;
    padding: 16px 12px;
  }
  .sidebar-collapse {
    display: none;
  }
  .sidebar-nav-item {
    justify-content: center;
    padding: 0;
  }
  .sidebar-nav-active-bar {
    right: -1px;
  }
  .workbench-topbar {
    padding: 12px 18px;
  }
  .workbench-content {
    padding: 20px 18px 28px;
  }
}
@media (max-width: 640px) {
  .workbench-sidebar {
    display: none;
  }
  .workbench-topbar {
    min-height: 68px;
    padding: 12px 14px;
  }
  .mobile-menu-button {
    display: inline-grid;
  }
  .topbar-leading,
  .topbar-actions {
    gap: 10px;
  }
  .topbar-runtime-label {
    display: none;
  }
  .topbar-actions :deep(.account-trigger) {
    max-width: 42px;
    padding: 0;
  }
  .topbar-actions :deep(.account-trigger-copy),
  .topbar-actions :deep(.account-trigger-chevron) {
    display: none;
  }
  .workbench-content {
    padding: 16px 14px calc(76px + env(safe-area-inset-bottom));
  }
  .mobile-nav {
    position: fixed;
    right: 0;
    bottom: 0;
    left: 0;
    z-index: 80;
    display: grid;
    grid-template-columns: repeat(5, 1fr);
    gap: 4px;
    padding: 8px 10px calc(8px + env(safe-area-inset-bottom));
    border-top: 1px solid var(--surface-border);
    background: var(--surface-1);
  }
  .mobile-nav-item {
    min-width: 0;
    min-height: 48px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 4px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: var(--muted-text);
    cursor: pointer;
    font-size: 10px;
    font-weight: 650;
    text-decoration: none;
  }
  .mobile-nav-item > span:first-child {
    font-size: 17px;
  }
  .mobile-nav-item--active {
    background: color-mix(in srgb, var(--theme-primary) 10%, var(--surface-1));
    color: var(--theme-primary);
  }
  .mobile-sheet-backdrop {
    position: fixed;
    inset: 0;
    z-index: 100;
    display: flex;
    align-items: flex-end;
    background: rgba(12, 25, 19, 0.36);
  }
  .mobile-sheet {
    width: 100%;
    padding: 20px 16px calc(18px + env(safe-area-inset-bottom));
    border-top: 1px solid var(--surface-border);
    border-radius: 16px 16px 0 0;
    background: var(--surface-1);
    box-shadow: 0 -16px 40px rgba(15, 37, 25, 0.16);
  }
  .mobile-sheet-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    margin-bottom: 16px;
  }
  .mobile-sheet-header h2 {
    margin: 4px 0 0;
    color: var(--theme-text);
    font-size: 19px;
  }
  .mobile-sheet-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }
  .mobile-sheet-item {
    min-height: 56px;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 0 12px;
    border: 1px solid var(--surface-border);
    border-radius: 9px;
    background: var(--surface-2);
    color: var(--theme-text);
    cursor: pointer;
    font-size: 12px;
    font-weight: 650;
    text-align: left;
  }
  .mobile-sheet-item:active {
    border-color: var(--theme-primary);
  }
  .mobile-sheet-icon {
    color: var(--theme-primary);
    font-size: 17px;
  }
  .mobile-sheet-arrow {
    margin-left: auto;
    color: var(--muted-text);
    font-size: 14px;
  }
}
.mobile-sheet-enter-active,
.mobile-sheet-leave-active {
  transition: opacity 0.2s ease;
}
.mobile-sheet-enter-active .mobile-sheet,
.mobile-sheet-leave-active .mobile-sheet {
  transition: transform 0.2s ease;
}
.mobile-sheet-enter-from,
.mobile-sheet-leave-to {
  opacity: 0;
}
.mobile-sheet-enter-from .mobile-sheet,
.mobile-sheet-leave-to .mobile-sheet {
  transform: translateY(100%);
}
</style>
