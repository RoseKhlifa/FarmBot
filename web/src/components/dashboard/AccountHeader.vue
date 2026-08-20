<!-- eslint-disable style/max-statements-per-line -->
<script setup lang="ts">
defineProps<{ name: string, avatar: string, level: number, gold: unknown, coupon: unknown, goldBean: unknown, uptime: string, connected: boolean, expPercent: number, expCurrent: unknown, expNeeded: unknown, expRate: string, timeToLevel: string, allAccountsRunning: boolean, startAllLoading: boolean, startBtnStyle: Record<string, string> }>()
const emit = defineEmits<{ career: [], startAll: [] }>()
</script>

<template>
  <div class="overview-card">
    <div class="flex flex-col">
      <div class="account-card-header">
        <div class="flex items-center gap-2">
          <span class="account-card-icon i-carbon-user-avatar" />
          <span>当前账号</span>
        </div>
        <button class="account-run-toggle" :disabled="startAllLoading" :title="allAccountsRunning ? '停止全部账号' : '一键启动全部账号'" @click="emit('startAll')">
          <span class="account-run-track" :style="startBtnStyle" />
          <span class="account-run-knob" :class="{ 'account-run-knob--active': allAccountsRunning }"><span v-if="startAllLoading" class="i-svg-spinners-90-ring-with-bg" /><span v-else-if="allAccountsRunning" class="i-carbon-stop-filled" /><span v-else class="i-carbon-play-filled-alt" /></span>
        </button>
      </div><div class="account-card-body">
        <div class="account-identity">
          <div class="account-avatar" @click="emit('career')">
            <img v-if="avatar" :src="avatar" :alt="name" class="h-full w-full object-cover"><div v-else class="h-full flex items-center justify-center text-2xl text-emerald-400 font-bold">
              {{ (name || '?').charAt(0).toUpperCase() }}
            </div><div class="account-level">
              Lv.{{ level }}
            </div>
          </div><div class="account-name">
            {{ name }}
          </div><div class="account-exp">
            <div class="account-exp-track">
              <div class="account-exp-fill" :style="{ width: `${expPercent}%` }" />
            </div><div class="mt-1 text-center text-[9px] text-gray-400">
              EXP {{ expCurrent }} / {{ expNeeded }}
            </div><div class="mt-0.5 text-center text-[9px] text-gray-400">
              效率: {{ expRate }}
            </div><div class="text-center text-[9px] text-gray-400">
              {{ timeToLevel }}
            </div>
          </div>
        </div><div class="account-metrics">
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
            <span><i class="i-fas-clock" />在线</span><strong><b class="account-status-dot" :class="connected ? 'account-status-dot--live' : 'account-status-dot--offline'" />{{ uptime }}</strong>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.account-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 48px;
  padding: 0 16px;
  border-bottom: 1px solid var(--surface-border);
  color: var(--muted-text);
  font-size: 12px;
  font-weight: 700;
}
.account-card-icon {
  color: var(--theme-primary);
  font-size: 16px;
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
.account-card-body {
  display: flex;
  gap: 18px;
  padding: 18px 16px 16px;
}
.account-identity {
  width: 112px;
  display: flex;
  flex: 0 0 auto;
  flex-direction: column;
  align-items: center;
  gap: 7px;
}
.account-avatar {
  position: relative;
  width: 72px;
  height: 72px;
  overflow: hidden;
  border: 1px solid var(--surface-border);
  border-radius: 14px;
  background: color-mix(in srgb, var(--theme-primary) 10%, var(--surface-2));
  cursor: pointer;
}
.account-level {
  position: absolute;
  right: 50%;
  bottom: -2px;
  transform: translateX(50%);
  padding: 2px 8px;
  border: 2px solid var(--surface-1);
  border-radius: 999px;
  background: var(--theme-primary);
  color: #052e1b;
  font-size: 9px;
  font-weight: 800;
  white-space: nowrap;
}
.account-name {
  max-width: 105px;
  overflow: hidden;
  color: var(--theme-text);
  font-size: 12px;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.account-exp {
  width: 100%;
  padding: 0 3px;
  color: var(--muted-text);
  font-size: 9px;
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
  flex: 1;
  flex-direction: column;
}
.account-metric {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 36px;
  border-bottom: 1px solid var(--surface-border);
  color: var(--muted-text);
  font-size: 11px;
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
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}
.account-metric--last {
  border-bottom: 0;
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
  .account-card-body {
    gap: 11px;
    padding: 15px 12px;
  }
  .account-identity {
    width: 92px;
  }
  .account-avatar {
    width: 62px;
    height: 62px;
  }
  .account-name {
    max-width: 88px;
  }
}
</style>
