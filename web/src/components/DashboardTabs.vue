<script setup lang="ts">
import { ref } from 'vue'

export interface DashboardTab {
  key: string
  label: string
  icon: string
}

defineProps<{
  tabs: DashboardTab[]
  activeTab: string
}>()

const emit = defineEmits<{
  (e: 'update:activeTab', key: string): void
}>()

const scrollContainer = ref<HTMLElement | null>(null)

function selectTab(key: string) {
  emit('update:activeTab', key)
  // 滚动到选中 tab 居中
  if (scrollContainer.value) {
    const container = scrollContainer.value
    const activeEl = container.querySelector(`[data-tab-key="${key}"]`) as HTMLElement | null
    if (activeEl) {
      const containerRect = container.getBoundingClientRect()
      const elRect = activeEl.getBoundingClientRect()
      const scrollLeft = container.scrollLeft + elRect.left - containerRect.left - containerRect.width / 2 + elRect.width / 2
      container.scrollTo({ left: scrollLeft, behavior: 'smooth' })
    }
  }
}
</script>

<template>
  <div class="dashboard-tabs-wrapper">
    <!-- 环境光晕 -->
    <div class="tabs-ambient" aria-hidden="true" />

    <div
      ref="scrollContainer"
      class="dashboard-tabs"
    >
      <button
        v-for="tab in tabs"
        :key="tab.key"
        :data-tab-key="tab.key"
        class="tab-item"
        :class="{ 'tab-item--active': activeTab === tab.key }"
        @click="selectTab(tab.key)"
      >
        <span class="tab-icon" :class="tab.icon" />
        <span class="tab-label">{{ tab.label }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.dashboard-tabs-wrapper {
  position: relative;
  margin-bottom: 12px;
  z-index: 1;
}

/* 环境光晕 */
.tabs-ambient {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 60%;
  height: 80%;
  background: transparent;
  pointer-events: none;
  filter: none;
}

.dashboard-tabs {
  display: flex;
  gap: 2px;
  overflow-x: auto;
  scrollbar-width: none;
  -webkit-overflow-scrolling: touch;
  padding: 4px;
  border-radius: 11px;
  position: relative;
  background: var(--surface-1);
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
  border: 1px solid var(--theme-border);
  box-shadow: var(--surface-shadow-soft);
}

.dashboard-tabs::-webkit-scrollbar {
  display: none;
}

.tab-item {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 7px 11px;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  white-space: nowrap;
  flex-shrink: 0;
  background: transparent;
  color: color-mix(in srgb, var(--theme-text) 55%, transparent);
  transition:
    background 0.18s ease,
    color 0.18s ease;
  -webkit-tap-highlight-color: transparent;
  position: relative;
  user-select: none;
}

.tab-item:active {
  transform: none;
}

.tab-icon {
  font-size: 16px;
  transition: all 0.3s ease;
}

.tab-label {
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.01em;
  transition: all 0.3s ease;
}

/* Hover 效果（仅非触屏设备） */
@media (hover: hover) {
  .tab-item:hover:not(.tab-item--active) {
    color: color-mix(in srgb, var(--theme-text) 80%, transparent);
    background: color-mix(in srgb, var(--theme-text) 4%, transparent);
  }
}

/* 激活状态 — 光感 */
.tab-item--active {
  color: var(--theme-primary);
  background: color-mix(in srgb, var(--theme-primary) 10%, transparent);
  box-shadow: none;
}

.tab-item--active .tab-icon {
  filter: none;
}

/* 移动端适配 */
@media (max-width: 480px) {
  .dashboard-tabs-wrapper {
    margin-left: -12px;
    margin-right: -12px;
  }

  .dashboard-tabs {
    gap: 4px;
    padding: 4px 8px;
    border-radius: 14px;
  }

  .tab-item {
    padding: 7px 11px;
  }

  .tab-icon {
    font-size: 15px;
  }

  .tab-label {
    font-size: 12px;
  }
}

/* 无障碍动效 */
@media (prefers-reduced-motion: reduce) {
  .tab-item {
    transition: none !important;
  }
}
</style>
