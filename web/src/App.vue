<script setup lang="ts">
import { onUnmounted, watch } from 'vue'
import { RouterView } from 'vue-router'
import ToastContainer from '@/components/ToastContainer.vue'
import { useRealtime } from '@/composables/useRealtime'
import { useAccountStore } from '@/stores/account'
import { useAppStore } from '@/stores/app'
import { useUserStore } from '@/stores/user'

const accountStore = useAccountStore()
const appStore = useAppStore()
const userStore = useUserStore()
const realtime = useRealtime({
  getToken: () => String(userStore.token || ''),
  disconnectOnScopeDispose: true,
})

watch([
  () => String(userStore.token || ''),
  () => String(accountStore.currentAccountId || ''),
], ([token, accountId]) => {
  if (token && accountId)
    realtime.connect(accountId, token)
  else
    realtime.disconnect()
}, { immediate: true })

watch(() => String(accountStore.currentAccountId || ''), (accountId) => {
  if (accountId)
    void appStore.fetchTheme()
})

onUnmounted(() => realtime.disconnect())
</script>

<template>
  <div class="app-root h-screen w-full overflow-hidden" :style="{ color: 'var(--theme-text)' }">
    <RouterView />
    <ToastContainer />
  </div>
</template>

<style>
:root {
  --theme-bg: #fafafa;
  --theme-text: #18181b;
  --theme-primary: #10b981;
  --theme-secondary: #059669;
  --theme-accent: #f59e0b;
  --theme-gradient: #10b981;
  --theme-glass: rgba(255, 255, 255, 0.88);
  --theme-border: rgba(0, 0, 0, 0.08);
  --app-bg: #f4f5f4;
  --surface-1: #ffffff;
  --surface-2: #f4f4f5;
  --surface-3: #e4e4e7;
  --surface-border: rgba(0, 0, 0, 0.08);
  --surface-border-strong: rgba(0, 0, 0, 0.14);
  --surface-shadow: 0 18px 44px rgba(24, 24, 27, 0.1);
  --surface-shadow-soft: 0 4px 14px rgba(24, 24, 27, 0.06);
  --muted-text: #71717a;
  --input-bg: #ffffff;
  --panel-glow: transparent;
  color-scheme: light;
}

.dark {
  --theme-bg: #09090b;
  --theme-text: #f4f4f5;
  --theme-primary: #34d399;
  --theme-secondary: #10b981;
  --theme-accent: #fbbf24;
  --theme-gradient: #34d399;
  --theme-glass: rgba(24, 24, 27, 0.82);
  --theme-border: rgba(255, 255, 255, 0.07);
  --app-bg: #09090b;
  --surface-1: #18181b;
  --surface-2: #27272a;
  --surface-3: #3f3f46;
  --surface-border: rgba(255, 255, 255, 0.07);
  --surface-border-strong: rgba(255, 255, 255, 0.14);
  --surface-shadow: 0 18px 44px rgba(0, 0, 0, 0.3);
  --surface-shadow-soft: 0 4px 16px rgba(0, 0, 0, 0.2);
  --muted-text: #a1a1aa;
  --input-bg: #09090b;
  --panel-glow: transparent;
  color-scheme: dark;
}

* {
  box-sizing: border-box;
}
html,
body,
#app {
  min-height: 100%;
}
body {
  margin: 0;
  background: var(--app-bg);
  color: var(--theme-text);
  font-family:
    'JetBrains Mono',
    'JetBrains Mono NL',
    'HarmonyOS Sans SC',
    '鸿蒙黑体',
    'HarmonyOS Sans',
    'PingFang SC',
    'Microsoft YaHei',
    ui-sans-serif,
    system-ui,
    -apple-system,
    BlinkMacSystemFont,
    'Segoe UI',
    sans-serif;
  font-feature-settings: 'kern', 'liga';
  text-rendering: optimizeLegibility;
}
button,
input,
select,
textarea {
  font: inherit;
}
button:focus-visible,
a:focus-visible,
input:focus-visible,
select:focus-visible,
textarea:focus-visible {
  outline: 3px solid color-mix(in srgb, var(--theme-primary) 28%, transparent);
  outline-offset: 2px;
}

.app-root {
  background: var(--app-bg);
}

/* Shared surfaces: the reference UI uses quiet zinc surfaces and one accent. */
.bg-white {
  background-color: var(--surface-1) !important;
}
.bg-gray-50 {
  background-color: var(--surface-2) !important;
}
.dark .bg-gray-800,
.dark .bg-gray-900 {
  background-color: var(--surface-1) !important;
}
.dark .bg-gray-700 {
  background-color: var(--surface-3) !important;
}
.ui-card,
.ui-card-elevated,
.glass-card,
.glass-panel,
.overview-card {
  border: 1px solid var(--surface-border) !important;
  background: var(--surface-1) !important;
  box-shadow: var(--surface-shadow-soft) !important;
  backdrop-filter: none !important;
  -webkit-backdrop-filter: none !important;
}

.overview-card,
.ui-card,
.ui-card-elevated,
.glass-card,
.glass-panel {
  border-radius: 12px !important;
}

.workbench,
.auth-page {
  color: var(--theme-text);
}

.workbench .text-gray-900,
.workbench .text-gray-800,
.workbench .text-gray-700,
.workbench .text-gray-600,
.workbench .dark\:text-gray-100,
.workbench .dark\:text-gray-200,
.workbench .dark\:text-gray-300 {
  color: var(--theme-text) !important;
}

.workbench .text-gray-500,
.workbench .text-gray-400 {
  color: var(--muted-text) !important;
}

.workbench .border-gray-100,
.workbench .border-gray-200,
.workbench .dark\:border-gray-700 {
  border-color: var(--surface-border) !important;
}
.ui-subtle-panel {
  border: 1px solid var(--surface-border) !important;
  background: var(--surface-2) !important;
}
.shadow,
.shadow-sm,
.shadow-md {
  box-shadow: var(--surface-shadow-soft) !important;
}
.text-primary {
  color: var(--theme-primary) !important;
}
.bg-primary {
  background-color: var(--theme-primary) !important;
}
.border-primary {
  border-color: var(--theme-primary) !important;
}
.bg-gradient-primary {
  background: var(--theme-primary) !important;
}

::-webkit-scrollbar {
  width: 7px;
  height: 7px;
}
::-webkit-scrollbar-track {
  background: transparent;
}
::-webkit-scrollbar-thumb {
  border-radius: 5px;
  background: color-mix(in srgb, var(--muted-text) 35%, transparent);
}
::-webkit-scrollbar-thumb:hover {
  background: color-mix(in srgb, var(--theme-primary) 55%, transparent);
}
</style>
