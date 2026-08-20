<script setup lang="ts">
defineProps<{
  totalCards: number
  unusedTimeCardsCount: number
  totalUsers: number
  loginLogsTotal: number
}>()
</script>

<template>
  <header class="admin-panel-header">
    <div class="admin-panel-intro">
      <span class="page-kicker">FARMBOT / ADMIN</span>
      <h1><span class="i-carbon-user-admin" />运营控制台</h1>
      <p>管理卡密、用户、登录日志和公开配置，所有动作都会即时反馈。</p>
    </div>
    <div class="admin-kpis" aria-label="运营概览">
      <div><span>卡密总数</span><strong>{{ totalCards }}</strong></div>
      <div><span>可领时间卡</span><strong class="is-green">{{ unusedTimeCardsCount }}</strong></div>
      <div><span>用户数量</span><strong class="is-amber">{{ totalUsers }}</strong></div>
      <div><span>登录日志</span><strong class="is-red">{{ loginLogsTotal }}</strong></div>
    </div>
  </header>
</template>

<style scoped>
.admin-panel-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 14px;
  padding: 2px 2px 12px;
}
.admin-panel-intro {
  min-width: 0;
}
.admin-panel-intro h1 {
  display: flex;
  align-items: center;
  gap: 9px;
  margin: 8px 0 6px;
  color: var(--theme-text);
  font-size: 24px;
  letter-spacing: -0.03em;
  line-height: 1.15;
}
.admin-panel-intro h1 span {
  color: var(--theme-primary);
  font-size: 20px;
}
.admin-panel-intro p {
  margin: 0;
  color: var(--muted-text);
  font-size: 12px;
  line-height: 1.6;
}
.admin-kpis {
  display: grid;
  grid-template-columns: repeat(4, minmax(85px, 1fr));
  gap: 8px;
  min-width: min(100%, 390px);
}
.admin-kpis > div {
  min-width: 0;
  padding: 9px 11px;
  border: 1px solid var(--surface-border);
  border-radius: 9px;
  background: var(--surface-1);
}
.admin-kpis span {
  display: block;
  overflow: hidden;
  color: var(--muted-text);
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.admin-kpis strong {
  display: block;
  margin-top: 4px;
  color: var(--theme-text);
  font-size: 19px;
  font-variant-numeric: tabular-nums;
  line-height: 1;
}
.admin-kpis .is-green {
  color: var(--theme-primary);
}
.admin-kpis .is-amber {
  color: #d97706;
}
.admin-kpis .is-red {
  color: #ef4444;
}
@media (max-width: 900px) {
  .admin-panel-header {
    flex-direction: column;
    gap: 14px;
  }
  .admin-kpis {
    width: 100%;
    min-width: 0;
  }
}
@media (max-width: 520px) {
  .admin-kpis {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
