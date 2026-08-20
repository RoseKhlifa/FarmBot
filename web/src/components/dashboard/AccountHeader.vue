<!-- eslint-disable style/max-statements-per-line -->
<script setup lang="ts">
defineProps<{ name: string, avatar: string, level: number, gold: unknown, coupon: unknown, goldBean: unknown, uptime: string, connected: boolean, expPercent: number, expCurrent: unknown, expNeeded: unknown, expRate: string, timeToLevel: string, allAccountsRunning: boolean, startAllLoading: boolean, startBtnStyle: Record<string, string> }>()
const emit = defineEmits<{ career: [], startAll: [] }>()
</script>

<template>
  <div class="account-card">
    <div class="account-card-top">
      <div class="account-card-person">
        <button class="account-avatar" type="button" :title="`${name} · 查看等级详情`" @click="emit('career')">
          <img v-if="avatar" :src="avatar" :alt="name" class="h-full w-full object-cover"><span v-else class="account-avatar-fallback">
            {{ (name || '?').charAt(0).toUpperCase() }}
          </span>
        </button>
        <div class="account-card-person-copy">
          <strong>{{ name }}</strong>
          <span><b>Lv.{{ level }}</b><i class="account-status-dot" :class="connected ? 'account-status-dot--live' : 'account-status-dot--offline'" />{{ connected ? '运行中' : '未连接' }}</span>
        </div>
      </div>
      <button class="account-run-toggle" type="button" :disabled="startAllLoading" :title="allAccountsRunning ? '停止全部账号' : '一键启动全部账号'" @click="emit('startAll')">
        <span class="account-run-track" :style="startBtnStyle" />
        <span class="account-run-knob" :class="{ 'account-run-knob--active': allAccountsRunning }"><span v-if="startAllLoading" class="i-svg-spinners-90-ring-with-bg" /><span v-else-if="allAccountsRunning" class="i-carbon-stop-filled" /><span v-else class="i-carbon-play-filled-alt" /></span>
      </button>
    </div>

    <div class="account-card-details">
      <div class="account-progress">
        <div class="account-progress-line">
          <span>经验进度</span><strong>EXP {{ expCurrent }} / {{ expNeeded }}</strong>
        </div>
        <div class="account-exp-track">
          <div class="account-exp-fill" :style="{ width: `${expPercent}%` }" />
        </div>
        <div class="account-progress-hint">
          <span>效率 {{ expRate }}</span><span>{{ timeToLevel || '保持运行以积累经验' }}</span>
        </div>
      </div>
      <div class="account-metrics">
        <div class="account-metric">
          <span><i class="i-fas-coins" />金币</span><strong class="text-amber-500">{{ gold }}</strong>
        </div>
        <div class="account-metric">
          <span><i class="i-fas-ticket-alt" />点券</span><strong class="text-emerald-500">{{ coupon }}</strong>
        </div>
        <div class="account-metric">
          <span><i class="i-carbon-circle" />金豆</span><strong class="text-amber-500">{{ goldBean }}</strong>
        </div>
        <div class="account-metric account-metric--last">
          <span><i class="i-fas-clock" />在线时长</span><strong>{{ uptime }}</strong>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.account-card {
  border: 1px solid var(--surface-border);
  border-radius: 9px;
  background: var(--surface-1);
}
.account-card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 14px 16px;
  border-bottom: 1px solid var(--surface-border);
}
.account-card-person {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.account-run-toggle {
  position: relative;
  width: 62px;
  height: 30px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  cursor: pointer;
}
.account-run-track {
  position: absolute;
  inset: 0;
  border-radius: 999px;
  background: var(--theme-primary);
  opacity: 0.88;
}
.account-run-knob {
  position: absolute;
  top: 4px;
  left: 4px;
  width: 22px;
  height: 22px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: #fff;
  color: var(--theme-primary);
  font-size: 12px;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.16);
  transition: transform 0.18s ease;
}
.account-run-knob--active {
  transform: translateX(32px);
}
.account-card-person-copy {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 5px;
}
.account-card-person-copy strong {
  overflow: hidden;
  color: var(--theme-text);
  font-size: 14px;
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.account-card-person-copy span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--muted-text);
  font-size: 10px;
}
.account-card-person-copy b {
  color: var(--theme-primary);
  font-weight: 800;
}
.account-card-details {
  display: grid;
  grid-template-columns: minmax(180px, 0.85fr) minmax(0, 1.15fr);
  gap: 20px;
  padding: 16px;
}
.account-progress {
  display: flex;
  min-width: 0;
  flex-direction: column;
  justify-content: center;
  gap: 9px;
}
.account-progress-line,
.account-progress-hint {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  color: var(--muted-text);
  font-size: 10px;
}
.account-progress-line strong {
  color: var(--theme-text);
  font-size: 10px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}
.account-progress-hint {
  font-size: 9px;
}
.account-progress-hint span:last-child {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.account-avatar-fallback {
  display: grid;
  width: 100%;
  height: 100%;
  place-items: center;
  color: var(--theme-primary);
  font-size: 21px;
  font-weight: 800;
}
.account-avatar {
  position: relative;
  width: 46px;
  height: 46px;
  overflow: hidden;
  flex: 0 0 auto;
  border: 1px solid var(--surface-border);
  border-radius: 10px;
  background: color-mix(in srgb, var(--theme-primary) 10%, var(--surface-2));
  cursor: pointer;
}
.account-exp-track {
  height: 5px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--surface-3);
}
.account-exp-fill {
  height: 100%;
  border-radius: inherit;
  background: var(--theme-primary);
  transition: width 0.35s ease;
}
.account-metrics {
  min-width: 0;
  display: flex;
  align-items: stretch;
  gap: 7px;
}
.account-metric {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-width: 0;
  padding: 8px 9px;
  border: 1px solid var(--surface-border);
  border-radius: 7px;
  background: var(--surface-2);
  color: var(--muted-text);
  font-size: 10px;
}
.account-metric > span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.account-metric i {
  color: var(--theme-primary);
  font-size: 13px;
}
.account-metric strong {
  color: var(--theme-text);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}
.account-status-dot {
  width: 7px;
  height: 7px;
  display: inline-block;
  margin-right: 6px;
  border-radius: 50%;
  background: currentColor;
}
.account-status-dot--live {
  color: #22c55e;
  box-shadow: 0 0 0 3px color-mix(in srgb, #22c55e 14%, transparent);
}
.account-status-dot--offline {
  color: #ef4444;
}
@media (max-width: 480px) {
  .account-card-details {
    grid-template-columns: 1fr;
    gap: 14px;
    padding: 14px 12px;
  }
  .account-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    display: grid;
  }
  .account-avatar {
    width: 42px;
    height: 42px;
  }
}
</style>
