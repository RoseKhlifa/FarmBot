<!-- eslint-disable ts/no-use-before-define, regexp/no-unused-capturing-group -->
<script setup lang="ts">
import { useIntervalFn } from '@vueuse/core'
import { storeToRefs } from 'pinia'
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { accountApi } from '@/api'
import AccountModal from '@/components/AccountModal.vue'
import BagPanel from '@/components/BagPanel.vue'
import CareerModal from '@/components/CareerModal.vue'
import AccountHeader from '@/components/dashboard/AccountHeader.vue'
import LogConsole from '@/components/dashboard/LogConsole.vue'
import TodayStatsPanel from '@/components/dashboard/TodayStatsPanel.vue'
import FriendsTabContent from '@/components/DashboardFriendsTab.vue'
import DashboardTabs from '@/components/DashboardTabs.vue'
import DogGiftsPanel from '@/components/DogGiftsPanel.vue'
import FarmPanel from '@/components/FarmPanel.vue'
import AutomationSettingsTab from '@/components/settings/AutomationSettingsTab.vue'
import StrategySettingsTab from '@/components/settings/StrategySettingsTab.vue'
import TaskPanel from '@/components/TaskPanel.vue'
import { useAutomationSettings } from '@/composables/settings/useAutomationSettings'
import { useStrategySettings } from '@/composables/settings/useStrategySettings'
import { useAccountScope } from '@/composables/useAccountScope'
import { useDashboardCountdown } from '@/composables/useDashboardCountdown'
import { useTodayStats } from '@/composables/useTodayStats'
import { useAccountStore } from '@/stores/account'
import { useAppStore } from '@/stores/app'
import { useBagStore } from '@/stores/bag'
import { useSettingStore } from '@/stores/setting'
import { useStatusStore } from '@/stores/status'
import { useToastStore } from '@/stores/toast'
import { formatCouponAmount, formatGoldAmount, formatGoldBeanAmount } from '@/utils/number-format'
import Analytics from '@/views/Analytics.vue'
import Illustrated from '@/views/Illustrated.vue'

const statusStore = useStatusStore()
const accountStore = useAccountStore()
const bagStore = useBagStore()
const toastStore = useToastStore()

const {
  status,
  logs: statusLogs,
  accountLogs: statusAccountLogs,
  realtimeConnected,
  currentStatusReady,
} = storeToRefs(statusStore)
const { currentAccountId, currentAccount } = storeToRefs(accountStore)
const { dashboardItems } = storeToRefs(bagStore)
const dashboardCountdown = useDashboardCountdown(() => currentStatusReady.value && !status.value?.connection?.connected)
const todayStats = useTodayStats(computed(() => status.value?.operations || {}))
function toggleTodayStats() {
  todayStats.expanded.value = !todayStats.expanded.value
}

const showAccountDropdown = ref(false)
const showAccountModal = ref(false)
const showCareerModal = ref(false)
const appStore = useAppStore()
const startBtnStyle = computed(() => appStore.isDark
  ? { background: '#34d399', boxShadow: 'none' }
  : { background: '#10b981', boxShadow: 'none' })
const startAllLoading = ref(false)
const startAllResults = ref<{ name: string, ok: boolean, msg: string }[]>([])
const showStartAllModal = ref(false)
const accountToEdit = ref<any>(null)

function openCareerModal() {
  showCareerModal.value = true
}

// 关闭下拉（点击外部）
onMounted(() => {
  document.addEventListener('click', closeAccountDropdown)
  // 初始化主题
  if (localStorage.getItem('theme-override') === 'dark') {
    document.documentElement.classList.add('dark')
  }
})
onUnmounted(() => {
  document.removeEventListener('click', closeAccountDropdown)
})
function closeAccountDropdown(e: MouseEvent) {
  const el = e.target as HTMLElement
  if (!el.closest('[data-account-dropdown]'))
    showAccountDropdown.value = false
}
async function handleAccountSaved() {
  showAccountModal.value = false
  accountToEdit.value = null
  await accountStore.fetchAccounts()
}

// 一键启动
const allAccountsRunning = computed(() => {
  const accs = accountStore.accounts
  return accs.length > 0 && accs.every(a => a.running)
})

async function startAllAccounts() {
  if (startAllLoading.value)
    return
  startAllLoading.value = true
  startAllResults.value = []

  const accs = accountStore.accounts
  const toStart = accs.filter(a => !a.running)

  if (toStart.length === 0) {
    // 全部在线，执行全部停止
    for (const acc of accs) {
      try {
        await accountStore.stopAccount(String(acc.id))
        startAllResults.value.push({ name: acc.name || acc.nick || acc.id, ok: true, msg: '已停止' })
      }
      catch {
        startAllResults.value.push({ name: acc.name || acc.nick || acc.id, ok: false, msg: '停止失败' })
      }
    }
  }
  else {
    // 启动所有离线账号
    for (const acc of toStart) {
      try {
        await accountApi.startAccount(acc.id)
      }
      catch {
        startAllResults.value.push({ name: acc.name || acc.nick || acc.id, ok: false, msg: '启动失败，请重新扫码' })
      }
    }
    // 等待 5 秒后检查实际运行状态
    await new Promise(r => setTimeout(r, 5000))
    await accountStore.fetchAccounts()
    const latest = accountStore.accounts
    for (const acc of toStart) {
      const updated = latest.find(a => String(a.id) === String(acc.id))
      if (updated?.running) {
        startAllResults.value.push({ name: acc.name || acc.nick || acc.id, ok: true, msg: '启动成功' })
      }
      else {
        startAllResults.value.push({ name: acc.name || acc.nick || acc.id, ok: false, msg: '启动失败，请重新扫码' })
      }
    }
  }

  startAllLoading.value = false
  showStartAllModal.value = true
}

// 当前账号的微信昵称（去括号备注）
const nickName = computed(() => {
  const acc = currentAccount.value
  if (!acc)
    return '选择账号'
  const status = statusStore.status?.status
  const live = status?.name && status?.name !== '未登录' ? String(status.name).trim() : ''
  return live || acc.nick || acc.name || acc.uin || acc.qq || '选择账号'
})

// 当前账号的头像 URL
const currentAvatarSrc = computed(() => {
  const acc = currentAccount.value
  if (!acc)
    return ''
  const status = statusStore.status?.status
  const live = status?.avatar || status?.avatarUrl || status?.avatar_url
  if (live)
    return String(live).trim()
  const qq = String(acc.uin || acc.qq || '').trim()
  if (/^\d+$/.test(qq))
    return `https://q1.qlogo.cn/g?b=qq&nk=${qq}&s=100`
  return ''
})

const lastBagFetchAt = ref(0)
const clearingLogs = ref(false)

const filter = reactive({
  module: '',
  event: '',
  keyword: '',
  isWarn: '',
})

const hasActiveLogFilter = computed(() =>
  !!(filter.module || filter.event || filter.keyword || filter.isWarn),
)
const activeTab = ref('overview')
const panelEl = ref<HTMLElement | null>(null)

const swipeStart = { x: 0, y: 0 }
function onSwipeStart(e: TouchEvent) {
  const t = e.changedTouches && e.changedTouches[0]
  if (!t)
    return
  swipeStart.x = t.clientX
  swipeStart.y = t.clientY
}
function onSwipeEnd(e: TouchEvent) {
  const t = e.changedTouches && e.changedTouches[0]
  if (!t)
    return
  const dx = t.clientX - swipeStart.x
  const dy = t.clientY - swipeStart.y
  if (Math.abs(dx) > 60 && Math.abs(dx) > Math.abs(dy) * 1.5) {
    const idx = dashboardTabs.findIndex(it => it.key === activeTab.value)
    if (idx === -1)
      return
    const next = dx < 0 ? idx + 1 : idx - 1
    const target = dashboardTabs[next]
    if (target)
      activeTab.value = target.key
  }
}

// 切换 tab 时给内容区一个轻微淡入，缓解滑动卡顿观感
watch(activeTab, () => {
  const el = panelEl.value
  if (!el)
    return
  el.classList.remove('tab-fade')
  void el.offsetWidth
  el.classList.add('tab-fade')
})

const dashboardTabs = [
  { key: 'overview', label: '概览', icon: 'i-carbon-chart-pie' },
  { key: 'farm', label: '农场', icon: 'i-carbon-tree' },
  { key: 'bag', label: '背包', icon: 'i-carbon-backpack' },
  { key: 'friends', label: '好友', icon: 'i-carbon-user-multiple' },
  { key: 'pet', label: '宠物', icon: 'i-carbon-dog-walker' },
  { key: 'tasks', label: '任务', icon: 'i-carbon-task' },
  { key: 'automation', label: '自动控制', icon: 'i-carbon-settings-adjust' },
  { key: 'strategy', label: '策略设置', icon: 'i-carbon-settings' },
  { key: 'illustrated', label: '图鉴', icon: 'i-carbon-book' },
  { key: 'analytics', label: '分析', icon: 'i-carbon-analytics' },
]

const settingStore = useSettingStore()
function showAlert(message: string, _type: 'primary' | 'danger' = 'primary') {
  toastStore.info(message)
}

const {
  localAutomationSettings,
  automationSaving,
  fertilizerLandTypeOptions,
  fertilizerOptions,
  syncLocalAutomationSettings,
  saveAutomationSettings,
} = useAutomationSettings({
  currentAccountId,
  showAlert,
})

const {
  settingsLoading,
  strategySaving,
  localStrategySettings,
  plantingStrategyOptions,
  bagFallbackStrategyOptions,
  bagSeeds,
  bagSeedsLoading,
  bagSeedsError,
  sortedBagSeeds,
  preferredSeedOptions,
  strategyPreviewLabel,
  resetBagSeedPriority,
  moveBagSeed,
  removeBagSeedPriority,
  startBagSeedDrag,
  dragOverBagSeed,
  dropBagSeed,
  loadStrategyData,
  saveStrategySettings,
  resetStrategyState,
} = useStrategySettings({
  currentAccountId,
  getAutomationSettings: () => ({ automation: localAutomationSettings.value }),
  showAlert,
})

// 标记使用以消除 TS 未引用警告 (实际动态使用)
void settingStore.clearSettingsState
void resetStrategyState
void syncLocalAutomationSettings
void loadStrategyData

const currentAccountDisconnected = computed(() =>
  currentStatusReady.value && !status.value?.connection?.connected,
)

// 解析日志时间戳：后端 accountLog 的 time 为 "YYYY-MM-DD HH:mm:ss"（空格分隔，非标准 ISO），
// Chrome/V8 下 Date.parse 会返回 NaN。若回退到 Date.now()，旧日志会被错误地排到列表最底部「常驻」，
// 表现为“时间已过仍显示在日志最下方”。因此这里把空格替换为 'T' 转成 ISO 再解析。
function parseLogTs(time: any): number {
  if (time === null || time === undefined || time === '')
    return Date.now()
  const t = Number(time)
  if (!Number.isNaN(t) && t > 0)
    return t
  let s = String(time).trim()
  if (s.includes(' ') && /^\d{4}-\d{2}-\d{2} \d{1,2}:\d{2}(:\d{2})?/.test(s))
    s = s.replace(' ', 'T')
  const parsed = Date.parse(s)
  if (!Number.isNaN(parsed))
    return parsed
  return Date.now()
}

const allLogs = computed(() => {
  const sLogs = statusLogs.value || []
  const aLogs = (statusAccountLogs.value || []).map((log: any) => ({
    ts: parseLogTs(log.ts ?? log.time),
    time: log.time,
    tag: log.action === 'Error' ? '错误' : '系统',
    msg: log.reason ? `${log.msg} (${log.reason})` : log.msg,
    action: log.action,
    isAccountLog: true,
  }))

  const merged = [...sLogs, ...aLogs]
    .sort((a: any, b: any) => (a.ts || 0) - (b.ts || 0))

  // 配对标记：若某条「连接中断」重连日志之后存在对应的「已恢复在线」日志，
  // 则将其标记为已恢复，前端灰显，避免重连记录刷屏、常驻显眼位置。
  const recoverTsList = merged
    .filter((l: any) => l.action === 'reconnect_success' || /已恢复在线/.test(l.msg || ''))
    .map((l: any) => l.ts || 0)
  for (const log of merged) {
    const isInterrupt = log.action === 'ws_reconnect_failed' || /连接中断|重连失败/.test(log.msg || '')
    if (isInterrupt) {
      const recovered = recoverTsList.some(ts => ts > (log.ts || 0))
      if (recovered)
        log.recovered = true
    }
  }

  return merged
})

const modules = [
  { label: '全部模块', value: '' },
  { label: '农场', value: 'farm' },
  { label: '好友', value: 'friend' },
  { label: '仓库', value: 'warehouse' },
  { label: '任务', value: 'task' },
  { label: '系统', value: 'system' },
]

const events = [
  { label: '全部事件', value: '' },
  { label: '农场巡查', value: 'farm_cycle' },
  { label: '收获作物', value: 'harvest_crop' },
  { label: '清理枯枝', value: 'remove_plant' },
  { label: '种植种子', value: 'plant_seed' },
  { label: '施加化肥', value: 'fertilize' },
  { label: '土地提醒', value: 'lands_notify' },
  { label: '选择种子', value: 'seed_pick' },
  { label: '购买种子', value: 'seed_buy' },
  { label: '购买化肥', value: 'fertilizer_buy' },
  { label: '开启礼盒', value: 'fertilizer_gift_open' },
  { label: '获取任务', value: 'task_scan' },
  { label: '完成任务', value: 'task_claim' },
  { label: '免费礼包', value: 'mall_free_gifts' },
  { label: '分享奖励', value: 'daily_share' },
  { label: '会员礼包', value: 'vip_daily_gift' },
  { label: '月卡礼包', value: 'month_card_gift' },
  { label: '图鉴奖励', value: 'illustrated_rewards' },
  { label: '邮箱领取', value: 'email_rewards' },
  { label: '出售成功', value: 'sell_success' },
  { label: '土地升级', value: 'upgrade_land' },
  { label: '土地解锁', value: 'unlock_land' },
  { label: '好友巡查', value: 'friend_cycle' },
  { label: '访问好友', value: 'visit_friend' },
]

const logLevels = [
  { label: '全部级别', value: '' },
  { label: '普通', value: 'info' },
  { label: '警告', value: 'warn' },
]

const eventLabelMap: Record<string, string> = Object.fromEntries(
  events.filter(event => event.value).map(event => [event.value, event.label]),
)

const expRate = computed(() => {
  const gain = status.value?.sessionExpGained || 0
  const uptime = status.value?.uptime || 0
  if (!uptime)
    return '0/小时'
  const rate = gain / (uptime / 3600)
  return `${Math.floor(rate)}/小时`
})

const timeToLevel = computed(() => {
  const gain = status.value?.sessionExpGained || 0
  const uptime = status.value?.uptime || 0
  const current = status.value?.levelProgress?.current || 0
  const needed = status.value?.levelProgress?.needed || 0

  if (!needed || !uptime || gain <= 0)
    return ''

  const ratePerHour = gain / (uptime / 3600)
  if (ratePerHour <= 0)
    return ''

  const expNeeded = Math.max(0, needed - current)
  const minsToLevel = expNeeded / (ratePerHour / 60)

  if (minsToLevel < 60)
    return `约 ${Math.ceil(minsToLevel)} 分钟后升级`
  return `约 ${(minsToLevel / 60).toFixed(1)} 小时后升级`
})

const fertilizerNormal = computed(() => dashboardItems.value.find((item: any) => Number(item.id) === 1011))
const fertilizerOrganic = computed(() => dashboardItems.value.find((item: any) => Number(item.id) === 1012))
const collectionNormal = computed(() => dashboardItems.value.find((item: any) => Number(item.id) === 3001))
const collectionRare = computed(() => dashboardItems.value.find((item: any) => Number(item.id) === 3002))

const { nextFarmCheck, nextHelpCheck, nextStealCheck, localUptime, farmPct, helpPct, stealPct, formatDuration, update: updateCountdowns, setFromStatus: setCountdownStatus, reset: resetCountdown } = dashboardCountdown

function resetDashboardState() {
  lastBagFetchAt.value = 0
  localUptime.value = 0
  resetCountdown()
}

function getEventLabel(event: string) {
  return eventLabelMap[event] || event
}

function formatBucketTime(item: any) {
  if (!item)
    return '0.0h'
  if (item.hoursText)
    return item.hoursText.replace('小时', 'h')
  return `${(Number(item.count || 0) / 3600).toFixed(1)}h`
}

watch(status, newVal => setCountdownStatus(newVal), { deep: true })

function getLogTagClass(tag: string) {
  if (tag === '错误')
    return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  if (tag === '系统')
    return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
  if (tag === '警告')
    return 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300'
  return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
}

function getLogMsgClass(tag: string) {
  if (tag === '错误')
    return 'text-red-600 dark:text-red-400'
  return 'text-gray-700 dark:text-gray-300'
}

function formatLogTime(timeStr: string) {
  if (!timeStr)
    return ''
  const parts = timeStr.split(' ')
  return parts.length > 1 ? (parts[1] || timeStr) : timeStr
}

function getExpPercent(progress: any) {
  if (!progress || !progress.needed)
    return 0
  return Math.min(100, Math.max(0, (progress.current / progress.needed) * 100))
}

async function refreshBag(force = false) {
  if (!currentAccountId.value || !currentAccount.value?.running || !currentStatusReady.value || !status.value?.connection?.connected)
    return

  const now = Date.now()
  if (!force && now - lastBagFetchAt.value < 2500)
    return

  lastBagFetchAt.value = now
  await bagStore.fetchBag(currentAccountId.value)
}

async function refresh(forceReloadLogs = false) {
  if (!currentAccountId.value)
    return

  const account = currentAccount.value
  if (!account)
    return

  // 首次加载、断线回退时走 HTTP；实时连接正常时优先依赖 WS 推送。
  if (!realtimeConnected.value) {
    await statusStore.fetchStatus(currentAccountId.value)
    await statusStore.fetchAccountLogs(currentAccountId.value)
  }

  if (forceReloadLogs || hasActiveLogFilter.value || !realtimeConnected.value) {
    await statusStore.fetchLogs(currentAccountId.value, {
      module: filter.module || undefined,
      event: filter.event || undefined,
      keyword: filter.keyword || undefined,
      isWarn: filter.isWarn === 'warn' ? true : filter.isWarn === 'info' ? false : undefined,
    })
  }

  // 仅在账号运行且连接稳定后再拉背包，避免启动阶段出现 500。
  await refreshBag()
}

function syncRealtimeAccount() {
  if (currentAccountId.value)
    statusStore.connectRealtime(currentAccountId.value)
}

function onLogFilterChange() {
  refresh(true)
}

function onLogSearchTrigger() {
  refresh(true)
}

useAccountScope(currentAccountId, async (newId, oldId) => {
  if (oldId !== undefined && newId !== oldId) {
    statusStore.clearAccountScopedData()
    bagStore.clearBag()
    resetDashboardState()
  }
  syncRealtimeAccount()
  await refresh(true)
  // 切换账号后重新拉取当前账号的策略设置与自动控制配置，避免残留上一账号的数据
  if (currentAccountId.value) {
    await loadStrategyData()
    syncLocalAutomationSettings()
  }
})

watch(() => status.value?.connection?.connected, (connected) => {
  if (connected)
    refreshBag(true)
})

watch(() => JSON.stringify(status.value?.operations || {}), (next, prev) => {
  if (!realtimeConnected.value || next === prev)
    return
  refreshBag()
})

watch(hasActiveLogFilter, (enabled) => {
  statusStore.setRealtimeLogsEnabled(!enabled)
  refresh()
})

async function clearLogs() {
  if (!currentAccountId.value)
    return

  clearingLogs.value = true
  try {
    const { data } = await accountApi.clearLogs()
    if (data?.ok) {
      toastStore.success('日志已清空')
      await refresh(true)
    }
    else {
      toastStore.error(`清空失败: ${data?.error || '未知错误'}`)
    }
  }
  catch (error: any) {
    const message = error?.response?.data?.error || error?.message || '请求失败'
    toastStore.error(`清空失败: ${message}`)
  }
  finally {
    clearingLogs.value = false
  }
}

onMounted(async () => {
  statusStore.setRealtimeLogsEnabled(!hasActiveLogFilter.value)
  syncRealtimeAccount()
  await refresh()
  if (currentAccountId.value) {
    await loadStrategyData()
    syncLocalAutomationSettings()
  }
})

// Auto refresh fallback every 10s (WS 断开或启用筛选时回退 HTTP)
useIntervalFn(refresh, 10000)
// Countdown timer (every 1s)
useIntervalFn(updateCountdowns, 1000)
</script>

<template>
  <div ref="panelEl" class="dashboard-view" @touchstart="onSwipeStart" @touchend="onSwipeEnd">
    <header class="dashboard-hero">
      <div class="dashboard-hero-copy">
        <div class="dashboard-eyebrow">
          <span class="status-dot" :class="realtimeConnected ? 'status-dot--live' : 'status-dot--idle'" />
          FARMBOT / CONTROL ROOM
        </div>
        <h1>农场运行总览</h1>
        <p>集中查看账号状态、巡查节奏和最近动作，把需要处理的事情放在同一个视线里。</p>
      </div>
      <div class="dashboard-hero-actions">
        <button class="dashboard-button dashboard-button--quiet" type="button" @click="showAccountModal = true">
          <span class="i-carbon-add" />
          添加账号
        </button>
        <button class="dashboard-button dashboard-button--primary" type="button" :disabled="startAllLoading" @click="startAllAccounts">
          <span :class="startAllLoading ? 'i-svg-spinners-90-ring-with-bg' : (allAccountsRunning ? 'i-carbon-stop-filled' : 'i-carbon-play-filled-alt')" />
          {{ startAllLoading ? '处理中' : (allAccountsRunning ? '停止全部' : '启动全部') }}
        </button>
      </div>
    </header>

    <section class="dashboard-switcher" aria-label="工作区视图">
      <div class="dashboard-switcher-copy">
        <span>当前工作区</span>
        <strong>{{ dashboardTabs.find(tab => tab.key === activeTab)?.label || '概览' }}</strong>
      </div>
      <DashboardTabs
        :tabs="dashboardTabs"
        :active-tab="activeTab"
        @update:active-tab="activeTab = $event"
      />
    </section>

    <section v-show="activeTab === 'overview'" class="dashboard-overview">
      <div class="dashboard-overview-main">
        <section class="dashboard-account-focus">
          <div class="section-heading">
            <div>
              <span class="section-kicker">CURRENT ACCOUNT</span>
              <h2>当前账号</h2>
            </div>
            <span class="section-status" :class="status?.connection?.connected ? 'section-status--live' : 'section-status--idle'">
              <span class="status-dot" :class="status?.connection?.connected ? 'status-dot--live' : 'status-dot--idle'" />
              {{ status?.connection?.connected ? '正在运行' : (currentAccount ? '等待启动' : '未选择') }}
            </span>
          </div>
          <AccountHeader
            :name="String(nickName)"
            :avatar="currentAvatarSrc"
            :level="Number(status?.status?.level || 0)"
            :gold="formatGoldAmount(status?.status?.gold || 0)"
            :coupon="formatCouponAmount(status?.status?.coupon || 0)"
            :gold-bean="formatGoldBeanAmount(status?.status?.goldBean || 0)"
            :uptime="formatDuration(localUptime)"
            :connected="!!status?.connection?.connected"
            :exp-percent="getExpPercent(status?.levelProgress)"
            :exp-current="status?.levelProgress?.current || 0"
            :exp-needed="status?.levelProgress?.needed || '?'"
            :exp-rate="expRate"
            :time-to-level="timeToLevel"
            :all-accounts-running="allAccountsRunning"
            :start-all-loading="startAllLoading"
            :start-btn-style="startBtnStyle"
            @career="openCareerModal"
            @start-all="startAllAccounts"
          />
        </section>

        <div class="dashboard-insight-grid">
          <TodayStatsPanel
            :operations="todayStats.filteredOperations.value"
            :rows="todayStats.rows.value"
            :expanded="todayStats.expanded.value"
            :disconnected="currentAccountDisconnected"
            :get-name="todayStats.getOpName"
            :get-icon="todayStats.getOpIcon"
            :get-color="todayStats.getOpColor"
            @toggle="toggleTodayStats"
          />

          <section class="dashboard-priority-panel">
            <div class="section-heading section-heading--compact">
              <div>
                <span class="section-kicker">NEXT UP</span>
                <h2>下一步</h2>
              </div>
              <span class="i-carbon-arrow-up-right text-lg text-[var(--theme-primary)]" />
            </div>
            <div class="priority-list">
              <button class="priority-item" type="button" @click="activeTab = 'farm'">
                <span class="priority-icon priority-icon--farm i-carbon-tree" />
                <span><strong>农场巡查</strong><small>下次检查 {{ nextFarmCheck }}</small></span>
                <span class="i-carbon-chevron-right" />
              </button>
              <button class="priority-item" type="button" @click="activeTab = 'tasks'">
                <span class="priority-icon priority-icon--task i-carbon-task" />
                <span><strong>每日任务</strong><small>{{ currentAccount ? '查看今日成长进度' : '先选择一个账号' }}</small></span>
                <span class="i-carbon-chevron-right" />
              </button>
              <button class="priority-item" type="button" @click="activeTab = 'automation'">
                <span class="priority-icon priority-icon--setting i-carbon-settings-adjust" />
                <span><strong>自动控制</strong><small>检查规则和执行策略</small></span>
                <span class="i-carbon-chevron-right" />
              </button>
            </div>
          </section>
        </div>

        <section class="dashboard-log-panel">
          <div class="section-heading">
            <div>
              <span class="section-kicker">ACTIVITY FEED</span>
              <h2>运行日志</h2>
            </div>
            <span class="log-count">{{ allLogs.length }} 条记录</span>
          </div>
          <LogConsole
            :logs="allLogs"
            :modules="modules"
            :events="events"
            :levels="logLevels"
            :filter="filter"
            :clearing="clearingLogs"
            :event-label="getEventLabel"
            :tag-class="getLogTagClass"
            :msg-class="getLogMsgClass"
            :time="formatLogTime"
            @filter="onLogFilterChange"
            @update-filter="Object.assign(filter, $event)"
            @search="onLogSearchTrigger"
            @clear="clearLogs"
          />
        </section>
      </div>

      <aside class="dashboard-overview-rail">
        <section class="dashboard-rail-panel">
          <div class="section-heading section-heading--compact">
            <div>
              <span class="section-kicker">SCHEDULE</span>
              <h2>下次巡查</h2>
            </div>
            <span class="i-carbon-hourglass text-lg text-[var(--theme-primary)]" />
          </div>
          <div class="schedule-list">
            <button class="schedule-row" type="button" @click="activeTab = 'farm'">
              <span class="schedule-label"><span class="schedule-dot schedule-dot--farm" />农场</span>
              <span class="schedule-value">{{ nextFarmCheck }}</span>
              <span class="schedule-track"><span :style="{ width: `${farmPct * 100}%` }" class="schedule-fill schedule-fill--farm" /></span>
            </button>
            <button class="schedule-row" type="button" @click="activeTab = 'friends'">
              <span class="schedule-label"><span class="schedule-dot schedule-dot--help" />帮助</span>
              <span class="schedule-value">{{ nextHelpCheck }}</span>
              <span class="schedule-track"><span :style="{ width: `${helpPct * 100}%` }" class="schedule-fill schedule-fill--help" /></span>
            </button>
            <button class="schedule-row" type="button" @click="activeTab = 'friends'">
              <span class="schedule-label"><span class="schedule-dot schedule-dot--steal" />偷菜</span>
              <span class="schedule-value">{{ nextStealCheck }}</span>
              <span class="schedule-track"><span :style="{ width: `${stealPct * 100}%` }" class="schedule-fill schedule-fill--steal" /></span>
            </button>
          </div>
        </section>

        <section class="dashboard-rail-panel">
          <div class="section-heading section-heading--compact">
            <div>
              <span class="section-kicker">INVENTORY</span>
              <h2>资源概览</h2>
            </div>
            <span class="i-carbon-box text-lg text-[var(--theme-primary)]" />
          </div>
          <div class="resource-grid">
            <div class="resource-item">
              <span class="i-fas-flask text-emerald-500" /><small>普通化肥</small><strong>{{ formatBucketTime(fertilizerNormal) }}</strong>
            </div>
            <div class="resource-item">
              <span class="i-fas-vial text-sky-500" /><small>有机化肥</small><strong>{{ formatBucketTime(fertilizerOrganic) }}</strong>
            </div>
            <div class="resource-item">
              <span class="i-fas-bookmark text-amber-500" /><small>普通收藏</small><strong>{{ collectionNormal?.count || 0 }}</strong>
            </div>
            <div class="resource-item">
              <span class="i-fas-gem text-violet-500" /><small>典藏收藏</small><strong>{{ collectionRare?.count || 0 }}</strong>
            </div>
          </div>
        </section>

        <section class="dashboard-rail-panel account-switcher-panel">
          <div class="section-heading section-heading--compact">
            <div>
              <span class="section-kicker">ACCOUNTS</span>
              <h2>账号列表</h2>
            </div>
            <span class="account-count">{{ accountStore.accounts.length }}</span>
          </div>
          <div v-if="accountStore.accounts.length" class="account-list">
            <button
              v-for="account in accountStore.accounts"
              :key="account.id"
              class="account-list-item"
              :class="{ 'account-list-item--active': String(account.id) === String(currentAccountId) }"
              type="button"
              @click="accountStore.setCurrentAccount(account)"
            >
              <span class="account-list-avatar">{{ (account.nick || account.name || account.uin || '?').toString().charAt(0).toUpperCase() }}</span>
              <span class="account-list-copy"><strong>{{ account.nick || account.name || account.uin || '未命名账号' }}</strong><small>{{ account.running ? '运行中' : '未启动' }}</small></span>
              <span class="account-list-state" :class="account.running ? 'account-list-state--live' : ''" />
            </button>
          </div>
          <div v-else class="account-empty">
            <span class="i-carbon-user-follow text-xl" />
            <p>还没有接入农场账号</p>
            <small>添加账号后，运行状态和日志会显示在这里。</small>
          </div>
          <button class="account-add-button" type="button" @click="showAccountModal = true">
            <span class="i-carbon-add" />添加账号
          </button>
        </section>
      </aside>
    </section>

    <!-- 农场（复用 FarmPanel） -->
    <div v-show="activeTab === 'farm'" class="h-full">
      <FarmPanel />
    </div>

    <!-- 背包（复用 BagPanel） -->
    <div v-show="activeTab === 'bag'" class="h-full">
      <BagPanel />
    </div>

    <!-- 好友（复用 FriendsFriendList） -->
    <div v-show="activeTab === 'friends'" class="h-full">
      <FriendsTabContent />
    </div>

    <!-- 任务（复用 TaskPanel） -->
    <div v-show="activeTab === 'tasks'" class="h-full">
      <TaskPanel />
    </div>

    <!-- 宠物（护主犬同气礼包） -->
    <div v-show="activeTab === 'pet'" class="h-full">
      <DogGiftsPanel
        :account-id="currentAccountId"
        :account-running="allAccountsRunning"
      />
    </div>

    <!-- 自动控制（完整设置） -->
    <div v-show="activeTab === 'automation'" class="h-full">
      <AutomationSettingsTab
        v-model:settings="localAutomationSettings"
        :current-account-name="currentAccount?.nick || currentAccount?.name || ''"
        :current-account-id="currentAccountId"
        :loading="settingsLoading"
        :saving="automationSaving"
        :fertilizer-land-type-options="fertilizerLandTypeOptions"
        :fertilizer-options="fertilizerOptions"
        @save="saveAutomationSettings"
      />
    </div>

    <!-- 策略设置（完整设置） -->
    <div v-show="activeTab === 'strategy'" class="h-full">
      <StrategySettingsTab
        v-model:settings="localStrategySettings"
        :current-account-name="currentAccount?.nick || currentAccount?.name || ''"
        :current-account-id="currentAccountId"
        :loading="settingsLoading"
        :saving="strategySaving"
        :planting-strategy-options="plantingStrategyOptions"
        :preferred-seed-options="preferredSeedOptions"
        :bag-fallback-strategy-options="bagFallbackStrategyOptions"
        :strategy-preview-label="strategyPreviewLabel"
        :bag-seeds="bagSeeds"
        :sorted-bag-seeds="sortedBagSeeds"
        :bag-seeds-loading="bagSeedsLoading"
        :bag-seeds-error="bagSeedsError"
        @reset-bag-seed-priority="resetBagSeedPriority"
        @move-bag-seed="moveBagSeed"
        @remove-bag-seed="removeBagSeedPriority"
        @start-bag-seed-drag="startBagSeedDrag"
        @drag-over-bag-seed="dragOverBagSeed"
        @drop-bag-seed="dropBagSeed"
        @save="saveStrategySettings"
      />
    </div>

    <!-- 图鉴 -->
    <div v-show="activeTab === 'illustrated'" class="illustrated-container h-full">
      <Illustrated />
    </div>

    <!-- 分析 -->
    <div v-show="activeTab === 'analytics'" class="analytics-container h-full">
      <Analytics />
    </div>
  </div>

  <Teleport to="body">
    <AccountModal
      :show="showAccountModal"
      :edit-data="accountToEdit"
      @close="showAccountModal = false; accountToEdit = null"
      @saved="handleAccountSaved"
    />
    <CareerModal :show="showCareerModal" @close="showCareerModal = false" />
  </Teleport>

  <!-- 一键启动结果弹窗 -->
  <Teleport to="body">
    <Transition name="fade">
      <div v-if="showStartAllModal" class="fixed inset-0 z-[9999] flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm" @click.self="showStartAllModal = false">
        <div class="glass-card max-w-sm w-full rounded-2xl p-5">
          <h3 class="mb-4 text-center text-base font-bold">
            🚀 一键启动结果
          </h3>
          <div class="flex flex-col gap-2">
            <div
              v-for="(r, i) in startAllResults"
              :key="i"
              class="flex items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-sm"
              :class="r.ok ? 'bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-300' : 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-300'"
            >
              <div class="h-5 w-5 flex items-center justify-center rounded-full text-xs text-white font-bold" :class="r.ok ? 'bg-green-500' : 'bg-red-500'">
                {{ r.ok ? '✓' : '✕' }}
              </div>
              <span class="font-medium">{{ r.name }}</span>
              <span class="ml-auto text-xs opacity-75">{{ r.msg }}</span>
            </div>
          </div>
          <button class="mt-4 w-full rounded-xl bg-blue-500 py-2.5 text-sm text-white font-semibold transition-colors hover:bg-blue-600" @click="showStartAllModal = false">
            确定
          </button>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.dashboard-view {
  --dashboard-panel: var(--theme-glass, rgba(255, 255, 255, 0.78));
  --dashboard-panel-strong: var(--surface-1, #fff);
  --dashboard-muted: var(--muted-text, #64748b);
  --dashboard-line: var(--theme-border, rgba(15, 23, 42, 0.1));
  --dashboard-shadow: 0 12px 30px rgba(15, 23, 42, 0.06);
  display: flex;
  flex-direction: column;
  gap: 18px;
  max-width: 1480px;
  margin: 0 auto;
  color: var(--theme-text, #0f172a);
}

.dashboard-hero {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  padding: 25px 28px;
  border: 1px solid var(--dashboard-line);
  border-radius: 12px;
  background: color-mix(in srgb, var(--dashboard-panel-strong) 92%, var(--theme-primary, #10b981) 8%);
  box-shadow: var(--dashboard-shadow);
}

.dashboard-hero-copy {
  min-width: 0;
}

.dashboard-eyebrow,
.section-kicker {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: var(--dashboard-muted);
  font-size: 10px;
  font-weight: 750;
  letter-spacing: 0.12em;
  line-height: 1.2;
  text-transform: uppercase;
}

.dashboard-hero h1 {
  margin: 9px 0 5px;
  color: var(--theme-text, #0f172a);
  font-size: clamp(25px, 3vw, 38px);
  font-weight: 800;
  letter-spacing: -0.02em;
  line-height: 1.1;
}

.dashboard-hero p {
  max-width: 650px;
  margin: 0;
  color: var(--dashboard-muted);
  font-size: 13px;
  line-height: 1.65;
}

.dashboard-hero-actions {
  display: flex;
  flex: 0 0 auto;
  gap: 9px;
}

.dashboard-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  min-height: 38px;
  padding: 0 14px;
  border: 1px solid var(--dashboard-line);
  border-radius: 8px;
  font-size: 12px;
  font-weight: 700;
  transition:
    transform 0.18s ease,
    background 0.18s ease,
    border-color 0.18s ease;
}

.dashboard-button:hover:not(:disabled) {
  transform: translateY(-1px);
}

.dashboard-button:disabled {
  cursor: wait;
  opacity: 0.6;
}

.dashboard-button--quiet {
  background: transparent;
  color: var(--theme-text, #0f172a);
}

.dashboard-button--quiet:hover:not(:disabled) {
  border-color: color-mix(in srgb, var(--theme-primary, #10b981) 40%, var(--dashboard-line));
  background: color-mix(in srgb, var(--theme-primary, #10b981) 7%, transparent);
}

.dashboard-button--primary {
  border-color: var(--theme-primary, #10b981);
  background: var(--theme-primary, #10b981);
  color: #062c20;
}

.dashboard-button--primary:hover:not(:disabled) {
  filter: brightness(0.96);
}

.dashboard-switcher {
  display: flex;
  align-items: center;
  gap: 18px;
  min-width: 0;
  padding: 10px 12px 0;
}

.dashboard-switcher-copy {
  display: flex;
  flex: 0 0 auto;
  flex-direction: column;
  gap: 3px;
  min-width: 100px;
}

.dashboard-switcher-copy span {
  color: var(--dashboard-muted);
  font-size: 10px;
  font-weight: 650;
}

.dashboard-switcher-copy strong {
  font-size: 15px;
  line-height: 1.2;
}

.dashboard-switcher :deep(.dashboard-tabs-wrapper) {
  flex: 1;
  min-width: 0;
  margin: 0;
}

.dashboard-switcher :deep(.dashboard-tabs) {
  border-radius: 9px;
  background: color-mix(in srgb, var(--dashboard-panel-strong) 82%, transparent);
  box-shadow: none;
}

.dashboard-overview {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  align-items: start;
  gap: 18px;
}

.dashboard-overview-main,
.dashboard-overview-rail {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 18px;
}

.dashboard-account-focus,
.dashboard-priority-panel,
.dashboard-log-panel,
.dashboard-rail-panel {
  min-width: 0;
  border: 1px solid var(--dashboard-line);
  border-radius: 10px;
  background: var(--dashboard-panel);
  box-shadow: var(--dashboard-shadow);
}

.dashboard-account-focus,
.dashboard-log-panel,
.dashboard-rail-panel {
  padding: 20px;
}

.dashboard-account-focus :deep(.overview-card),
.dashboard-log-panel :deep(.overview-card) {
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}

.dashboard-account-focus :deep(.overview-card) {
  padding: 0;
}

.dashboard-log-panel :deep(.overview-card) {
  padding: 0;
}

.section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  margin-bottom: 17px;
}

.section-heading h2 {
  margin: 5px 0 0;
  color: var(--theme-text, #0f172a);
  font-size: 18px;
  font-weight: 780;
  line-height: 1.2;
}

.section-heading--compact {
  margin-bottom: 14px;
}

.section-heading--compact h2 {
  font-size: 16px;
}

.section-status,
.log-count,
.account-count {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex: 0 0 auto;
  color: var(--dashboard-muted);
  font-size: 11px;
  font-weight: 700;
}

.section-status--live {
  color: #059669;
}

.section-status--idle {
  color: var(--dashboard-muted);
}

.status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
}

.status-dot--live {
  color: #10b981;
  box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.14);
}

.status-dot--idle {
  color: #94a3b8;
}

.dashboard-insight-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) minmax(260px, 0.9fr);
  gap: 18px;
}

.dashboard-insight-grid > :deep(.overview-card) {
  min-width: 0;
  border: 1px solid var(--dashboard-line);
  border-radius: 10px;
  background: var(--dashboard-panel);
  box-shadow: var(--dashboard-shadow);
}

.dashboard-insight-grid > :deep(.overview-card) {
  padding: 20px;
}

.dashboard-priority-panel {
  padding: 20px;
}

.priority-list,
.schedule-list,
.account-list {
  display: flex;
  flex-direction: column;
  gap: 7px;
}

.priority-item,
.schedule-row,
.account-list-item {
  display: flex;
  align-items: center;
  width: 100%;
  min-width: 0;
  border: 1px solid transparent;
  border-radius: 8px;
  background: color-mix(in srgb, var(--theme-text, #0f172a) 3%, transparent);
  text-align: left;
  transition:
    background 0.18s ease,
    border-color 0.18s ease,
    transform 0.18s ease;
}

.priority-item {
  gap: 10px;
  padding: 10px;
}

.priority-item:hover,
.schedule-row:hover,
.account-list-item:hover {
  border-color: color-mix(in srgb, var(--theme-primary, #10b981) 30%, var(--dashboard-line));
  background: color-mix(in srgb, var(--theme-primary, #10b981) 7%, transparent);
  transform: translateX(2px);
}

.priority-item > span:nth-child(2),
.account-list-copy {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
  gap: 3px;
}

.priority-item strong,
.account-list-copy strong {
  overflow: hidden;
  color: var(--theme-text, #0f172a);
  font-size: 12px;
  font-weight: 750;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.priority-item small,
.account-list-copy small {
  overflow: hidden;
  color: var(--dashboard-muted);
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.priority-icon {
  display: grid;
  width: 30px;
  height: 30px;
  flex: 0 0 auto;
  place-items: center;
  border-radius: 8px;
  font-size: 15px;
}

.priority-icon--farm {
  background: rgba(16, 185, 129, 0.13);
  color: #059669;
}

.priority-icon--task {
  background: rgba(14, 165, 233, 0.13);
  color: #0284c7;
}

.priority-icon--setting {
  background: rgba(245, 158, 11, 0.14);
  color: #d97706;
}

.schedule-row {
  position: relative;
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 6px 12px;
  padding: 10px;
}

.schedule-label,
.schedule-value {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: var(--theme-text, #0f172a);
  font-size: 11px;
  font-weight: 700;
}

.schedule-value {
  color: var(--dashboard-muted);
  font-variant-numeric: tabular-nums;
}

.schedule-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.schedule-dot--farm,
.schedule-fill--farm {
  background: #10b981;
}

.schedule-dot--help,
.schedule-fill--help {
  background: #0ea5e9;
}

.schedule-dot--steal,
.schedule-fill--steal {
  background: #f59e0b;
}

.schedule-track {
  grid-column: 1 / -1;
  height: 4px;
  overflow: hidden;
  border-radius: 99px;
  background: color-mix(in srgb, var(--theme-text, #0f172a) 9%, transparent);
}

.schedule-fill {
  display: block;
  height: 100%;
  min-width: 3px;
  border-radius: inherit;
  transition: width 0.3s ease;
}

.resource-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.resource-item {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 3px 7px;
  min-width: 0;
  padding: 10px;
  border: 1px solid var(--dashboard-line);
  border-radius: 8px;
  background: color-mix(in srgb, var(--theme-text, #0f172a) 2%, transparent);
}

.resource-item > span {
  grid-row: span 2;
  align-self: center;
  font-size: 15px;
}

.resource-item small {
  overflow: hidden;
  color: var(--dashboard-muted);
  font-size: 9px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.resource-item strong {
  overflow: hidden;
  color: var(--theme-text, #0f172a);
  font-size: 13px;
  font-weight: 780;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-list-item {
  gap: 9px;
  padding: 8px;
}

.account-list-item--active {
  border-color: color-mix(in srgb, var(--theme-primary, #10b981) 45%, var(--dashboard-line));
  background: color-mix(in srgb, var(--theme-primary, #10b981) 10%, transparent);
}

.account-list-avatar {
  display: grid;
  width: 30px;
  height: 30px;
  flex: 0 0 auto;
  place-items: center;
  border-radius: 8px;
  background: color-mix(in srgb, var(--theme-primary, #10b981) 15%, var(--dashboard-panel-strong));
  color: var(--theme-primary, #10b981);
  font-size: 12px;
  font-weight: 800;
}

.account-list-state {
  width: 7px;
  height: 7px;
  flex: 0 0 auto;
  border-radius: 50%;
  background: #cbd5e1;
}

.account-list-state--live {
  background: #10b981;
  box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.14);
}

.account-empty {
  display: flex;
  align-items: center;
  flex-direction: column;
  gap: 7px;
  padding: 18px 10px;
  color: var(--dashboard-muted);
  text-align: center;
}

.account-empty p {
  margin: 0;
  color: var(--theme-text, #0f172a);
  font-size: 12px;
  font-weight: 700;
}

.account-empty small {
  max-width: 220px;
  font-size: 10px;
  line-height: 1.5;
}

.account-add-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  width: 100%;
  min-height: 34px;
  margin-top: 10px;
  border: 1px dashed color-mix(in srgb, var(--theme-primary, #10b981) 45%, var(--dashboard-line));
  border-radius: 8px;
  background: transparent;
  color: var(--theme-primary, #10b981);
  font-size: 11px;
  font-weight: 750;
  transition: background 0.18s ease;
}

.account-add-button:hover {
  background: color-mix(in srgb, var(--theme-primary, #10b981) 8%, transparent);
}

.dashboard-log-panel :deep(.ui-subtle-panel) {
  min-height: 260px;
  border-color: var(--dashboard-line) !important;
  background: color-mix(in srgb, var(--theme-text, #0f172a) 3%, transparent) !important;
}

.dashboard-log-panel :deep(.flex.flex-1.flex-col > .mb-4) {
  margin-bottom: 12px;
}

/* Keep embedded views on the same neutral surface when switching tabs. */
:deep(.illustrated-container),
:deep(.analytics-container),
.illustrated-container :deep(.rounded-lg),
.analytics-container :deep(.rounded-lg) {
  background: var(--dashboard-panel) !important;
  border-color: var(--dashboard-line) !important;
  box-shadow: none !important;
}

:deep(.illustrated-container .bg-white),
:deep(.analytics-container .bg-white),
:deep(.illustrated-container .bg-gray-50),
:deep(.analytics-container .bg-gray-50) {
  background: transparent !important;
}

.tab-fade {
  animation: tabFadeIn 0.2s ease;
}

@keyframes tabFadeIn {
  from {
    opacity: 0.35;
  }
  to {
    opacity: 1;
  }
}

@media (max-width: 1180px) {
  .dashboard-overview {
    grid-template-columns: minmax(0, 1fr);
  }

  .dashboard-overview-rail {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    align-items: start;
  }
}

@media (max-width: 820px) {
  .dashboard-hero {
    align-items: flex-start;
    flex-direction: column;
    padding: 21px 20px;
  }

  .dashboard-hero-actions {
    width: 100%;
  }

  .dashboard-button {
    flex: 1;
  }

  .dashboard-switcher {
    align-items: flex-start;
    flex-direction: column;
    gap: 10px;
    padding: 3px 0 0;
  }

  .dashboard-switcher-copy {
    flex-direction: row;
    align-items: baseline;
    gap: 8px;
  }

  .dashboard-overview-rail,
  .dashboard-insight-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 560px) {
  .dashboard-view {
    gap: 14px;
  }

  .dashboard-hero {
    border-radius: 10px;
    padding: 18px 16px;
  }

  .dashboard-hero h1 {
    font-size: 25px;
  }

  .dashboard-hero p {
    font-size: 12px;
  }

  .dashboard-account-focus,
  .dashboard-log-panel,
  .dashboard-rail-panel,
  .dashboard-priority-panel {
    padding: 16px;
    border-radius: 9px;
  }

  .dashboard-insight-grid > :deep(.overview-card) {
    padding: 16px;
  }

  .dashboard-log-panel :deep(.ui-subtle-panel) {
    min-height: 220px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .dashboard-button,
  .priority-item,
  .schedule-row,
  .account-list-item,
  .tab-fade {
    transition: none !important;
    animation: none !important;
  }
}
</style>
