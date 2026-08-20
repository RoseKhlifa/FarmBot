<script setup lang="ts">
export type AdminTabKey = 'card' | 'user' | 'log' | 'system'

export interface AdminTabItem {
  key: AdminTabKey
  label: string
  icon: string
}

defineProps<{ tabs: readonly AdminTabItem[] }>()
const activeTab = defineModel<AdminTabKey>('activeTab', { required: true })
</script>

<template>
  <section class="admin-tabs-shell">
    <nav class="admin-tabs-nav" aria-label="管理模块">
      <button
        v-for="tab in tabs"
        :key="tab.key"
        type="button"
        class="admin-tab"
        :class="{ 'admin-tab--active': activeTab === tab.key }"
        @click="activeTab = tab.key"
      >
        <span :class="tab.icon" />
        <span>{{ tab.label }}</span>
      </button>
    </nav>
    <div class="admin-tab-content">
      <slot />
    </div>
  </section>
</template>

<style scoped>
.admin-tabs-shell {
  min-width: 0;
}
.admin-tabs-nav {
  display: flex;
  gap: 3px;
  overflow-x: auto;
  padding: 4px;
  border: 1px solid var(--surface-border);
  border-radius: 11px;
  background: var(--surface-1);
  box-shadow: var(--surface-shadow-soft);
  scrollbar-width: none;
}
.admin-tabs-nav::-webkit-scrollbar {
  display: none;
}
.admin-tab {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 7px;
  min-height: 34px;
  padding: 0 12px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--muted-text);
  cursor: pointer;
  font-size: 12px;
  font-weight: 700;
  transition:
    background 0.18s ease,
    color 0.18s ease;
}
.admin-tab:hover {
  background: color-mix(in srgb, var(--theme-text) 5%, transparent);
  color: var(--theme-text);
}
.admin-tab--active {
  background: color-mix(in srgb, var(--theme-primary) 13%, transparent);
  color: var(--theme-primary);
}
.admin-tab > span:first-child {
  font-size: 15px;
}
.admin-tab-content {
  min-width: 0;
  margin-top: 10px;
  padding: 16px;
  border: 1px solid var(--surface-border);
  border-radius: 12px;
  background: var(--surface-1);
  box-shadow: var(--surface-shadow-soft);
}
@media (max-width: 640px) {
  .admin-tab-content {
    padding: 11px;
  }
}
</style>
