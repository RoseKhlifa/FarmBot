<script setup lang="ts">
import type { Theme } from '@/stores/app'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()
</script>

<template>
  <button
    class="theme-toggle"
    :aria-label="appStore.isDark ? '切换到浅色模式' : '切换到深色模式'"
    :title="appStore.isDark ? '切换到浅色模式' : '切换到深色模式'"
    @click="appStore.toggleDark()"
  >
    <span class="theme-toggle-icon" :class="appStore.isDark ? 'i-carbon-moon' : 'i-carbon-sun'" />
    <span class="theme-toggle-label">{{ appStore.isDark ? '深色' : '浅色' }}</span>
  </button>

  <teleport to="body">
    <div v-if="appStore.showThemePanel" class="theme-panel-backdrop" @click="appStore.toggleThemePanel()">
      <section class="theme-panel" @click.stop>
        <div class="theme-panel-header">
          <div>
            <span class="page-kicker">APPEARANCE</span>
            <h2>界面主题</h2>
          </div>
          <button class="theme-panel-close" aria-label="关闭主题面板" @click="appStore.toggleThemePanel()">
            <span class="i-carbon-close" />
          </button>
        </div>
        <button
          v-for="(theme, key) in appStore.themes"
          :key="key"
          class="theme-option"
          :class="{ 'theme-option--active': appStore.currentTheme === key }"
          @click="appStore.applyTheme(key as Theme); appStore.toggleThemePanel()"
        >
          <span class="theme-option-swatch" :style="{ background: theme.primary }" />
          <span>
            <strong>{{ theme.name }}</strong>
            <small>{{ theme.isDark ? '深色工作台' : '浅色工作台' }}</small>
          </span>
          <span v-if="appStore.currentTheme === key" class="theme-option-check i-carbon-checkmark" />
        </button>
      </section>
    </div>
  </teleport>
</template>

<style scoped>
.theme-toggle {
  min-height: 34px;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 0 10px;
  border: 1px solid var(--surface-border);
  border-radius: 8px;
  background: var(--surface-1);
  color: var(--muted-text);
  cursor: pointer;
  font-size: 11px;
  font-weight: 700;
  transition: 0.18s ease;
}
.theme-toggle:hover {
  border-color: var(--surface-border-strong);
  color: var(--theme-text);
}
.theme-toggle-icon {
  font-size: 15px;
}
.theme-toggle-label {
  line-height: 1;
}
.theme-panel-backdrop {
  position: fixed;
  inset: 0;
  z-index: 200;
  display: grid;
  place-items: center;
  padding: 20px;
  background: rgba(14, 26, 19, 0.34);
}
.theme-panel {
  width: min(100%, 360px);
  padding: 20px;
  border: 1px solid var(--surface-border);
  border-radius: 12px;
  background: var(--surface-1);
  box-shadow: var(--surface-shadow);
}
.theme-panel-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 16px;
}
.theme-panel-header h2 {
  margin: 4px 0 0;
  color: var(--theme-text);
  font-size: 20px;
}
.theme-panel-close {
  width: 32px;
  height: 32px;
  display: grid;
  place-items: center;
  border: 1px solid var(--surface-border);
  border-radius: 8px;
  background: transparent;
  color: var(--muted-text);
  cursor: pointer;
}
.theme-option {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border: 1px solid var(--surface-border);
  border-radius: 8px;
  background: var(--surface-2);
  color: var(--theme-text);
  cursor: pointer;
  text-align: left;
}
.theme-option + .theme-option {
  margin-top: 8px;
}
.theme-option--active {
  border-color: color-mix(in srgb, var(--theme-primary) 45%, var(--surface-border));
  background: color-mix(in srgb, var(--theme-primary) 9%, var(--surface-1));
}
.theme-option-swatch {
  width: 24px;
  height: 24px;
  flex: 0 0 auto;
  border-radius: 6px;
}
.theme-option span:nth-child(2) {
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.theme-option strong {
  font-size: 13px;
}
.theme-option small {
  color: var(--muted-text);
  font-size: 11px;
}
.theme-option-check {
  margin-left: auto;
  color: var(--theme-primary);
}
@media (max-width: 640px) {
  .theme-toggle-label {
    display: none;
  }
  .theme-toggle {
    width: 34px;
    justify-content: center;
    padding: 0;
  }
}
</style>
