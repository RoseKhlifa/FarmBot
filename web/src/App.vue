<script setup lang="ts">
import { onUnmounted, watch } from 'vue'
import { RouterView } from 'vue-router'
import ToastContainer from '@/components/ToastContainer.vue'
import { useRealtime } from '@/composables/useRealtime'
import { useAccountStore } from '@/stores/account'
import { useUserStore } from '@/stores/user'

const accountStore = useAccountStore()
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
  --theme-bg: #f5f8f5;
  --theme-text: #17231d;
  --theme-primary: #18794e;
  --theme-secondary: #14613f;
  --theme-accent: #c77926;
  --theme-gradient: #18794e;
  --theme-glass: #ffffff;
  --theme-border: #d9e4dc;
  --app-bg: #f1f5f2;
  --surface-1: #ffffff;
  --surface-2: #f5f8f5;
  --surface-3: #eaf1ec;
  --surface-border: #dce6df;
  --surface-border-strong: #c7d6cb;
  --surface-shadow: 0 14px 34px rgba(34, 63, 47, 0.1);
  --surface-shadow-soft: 0 5px 16px rgba(34, 63, 47, 0.07);
  --muted-text: #68786f;
  --input-bg: #fbfdfb;
  --panel-glow: transparent;
  color-scheme: light;
}

.dark {
  --theme-bg: #121916;
  --theme-text: #e9f1eb;
  --theme-primary: #63c995;
  --theme-secondary: #3da975;
  --theme-accent: #e1a45b;
  --theme-gradient: #3da975;
  --theme-glass: #1b2520;
  --theme-border: #2c3b32;
  --app-bg: #111713;
  --surface-1: #19221d;
  --surface-2: #202c25;
  --surface-3: #29382f;
  --surface-border: #2c3b32;
  --surface-border-strong: #3b4d41;
  --surface-shadow: 0 18px 44px rgba(0, 0, 0, 0.24);
  --surface-shadow-soft: 0 6px 18px rgba(0, 0, 0, 0.16);
  --muted-text: #9aac9f;
  --input-bg: #131b16;
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
    Inter,
    ui-sans-serif,
    system-ui,
    -apple-system,
    BlinkMacSystemFont,
    'Segoe UI',
    sans-serif;
  font-feature-settings: 'cv02', 'cv03', 'cv04', 'cv11';
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

/* Shared surfaces: keep dense operational pages coherent while old views migrate. */
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
