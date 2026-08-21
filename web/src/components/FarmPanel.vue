<script setup lang="ts">
import { useIntervalFn } from '@vueuse/core'
import { storeToRefs } from 'pinia'
import { computed, onUnmounted, ref, watch } from 'vue'
import ConfirmModal from '@/components/ConfirmModal.vue'
import LandCard from '@/components/LandCard.vue'
import { useAccountStore } from '@/stores/account'
import { useFarmStore } from '@/stores/farm'
import { useStatusStore } from '@/stores/status'

const farmStore = useFarmStore()
const accountStore = useAccountStore()
const statusStore = useStatusStore()
const { lands, loading } = storeToRefs(farmStore)
const { currentAccountId, currentAccount } = storeToRefs(accountStore)
const { status, loading: statusLoading, realtimeConnected, currentStatusReady } = storeToRefs(statusStore)

const operating = ref(false)
const farmLoaded = ref(false)
const confirmVisible = ref(false)
const moreActionsVisible = ref(false)
type PendingLandAction = 'fertilize' | 'remove'

const confirmConfig = ref({
  title: '',
  message: '',
  opType: '',
  bulkAction: '' as 'removeAll' | '',
  landAction: '' as PendingLandAction | '',
  land: null as any | null,
  type: 'primary' as 'primary' | 'danger',
})

async function executeOperate() {
  if (!currentAccountId.value)
    return
  const config = confirmConfig.value
  if (!config.opType && !config.bulkAction && (!config.landAction || !config.land))
    return
  confirmVisible.value = false
  moreActionsVisible.value = false
  operating.value = true
  try {
    if (config.opType)
      await farmStore.operate(currentAccountId.value, config.opType)
    else if (config.bulkAction === 'removeAll')
      await farmStore.removeAllPlants(currentAccountId.value)
    else if (config.landAction === 'fertilize')
      await farmStore.fertilizeLand(currentAccountId.value, Number(config.land.id))
    else if (config.landAction === 'remove')
      await farmStore.removePlant(currentAccountId.value, Number(config.land.id))
  }
  finally { operating.value = false }
}

function handleOperate(opType: string) {
  if (!currentAccountId.value)
    return
  moreActionsVisible.value = false
  const confirmMap: Record<string, string> = {
    harvest: '确定要收获所有成熟作物吗？',
    clear: '确定要照料当前农场吗？将自动浇水、除草、除虫。',
    plant: '确定要补种当前空地吗？将按种植策略执行。',
    upgrade: '确定要升级所有可升级的土地吗？会消耗金币。',
    all: '确定要处理当前农场吗？将依次收获、照料、补种并升级土地。',
  }
  confirmConfig.value = {
    title: '确认操作',
    message: confirmMap[opType] || '确定执行此操作吗？',
    opType,
    bulkAction: '',
    landAction: '',
    land: null,
    type: 'primary',
  }
  confirmVisible.value = true
}

function handleRemoveAllPlants() {
  if (!currentAccountId.value)
    return
  moreActionsVisible.value = false
  confirmConfig.value = {
    title: '确认清理作物',
    message: '确定要清理全部已种植作物吗？此操作不可恢复。',
    opType: '',
    bulkAction: 'removeAll',
    landAction: '',
    land: null,
    type: 'danger',
  }
  confirmVisible.value = true
}

function getLandActionName(land: any) {
  return `#${land?.id ?? '-'} ${land?.plantName || '该作物'}`
}

function handleLandFertilize(land: any) {
  if (!currentAccountId.value)
    return
  confirmConfig.value = {
    title: '确认催熟',
    message: `确定要对 ${getLandActionName(land)} 使用有机肥料催熟吗？`,
    opType: '',
    bulkAction: '',
    landAction: 'fertilize',
    land,
    type: 'primary',
  }
  confirmVisible.value = true
}

function handleLandRemove(land: any) {
  if (!currentAccountId.value)
    return
  confirmConfig.value = {
    title: '确认铲除',
    message: `确定要铲除 ${getLandActionName(land)} 吗？此操作不可恢复。`,
    opType: '',
    bulkAction: '',
    landAction: 'remove',
    land,
    type: 'danger',
  }
  confirmVisible.value = true
}

const operations = [
  { type: 'harvest', label: '收获', icon: 'i-carbon-wheat', title: '收获成熟作物' },
  { type: 'clear', label: '照料', icon: 'i-carbon-clean', title: '浇水、除草、除虫' },
  { type: 'plant', label: '补种', icon: 'i-carbon-sprout', title: '为闲置土地补种' },
  { type: 'upgrade', label: '升级土地', icon: 'i-carbon-upgrade' },
]

async function refresh() {
  if (currentAccountId.value) {
    const acc = currentAccount.value
    if (!acc)
      return
    try {
      if (!realtimeConnected.value)
        await statusStore.fetchStatus(currentAccountId.value)
      if (acc.running)
        await farmStore.fetchLands(currentAccountId.value)
    }
    finally {
      farmLoaded.value = true
    }
  }
}

const showInitialLoading = computed(() => !farmLoaded.value && (loading.value || statusLoading.value))

watch(currentAccountId, (newId, oldId) => {
  if (oldId !== undefined && newId !== oldId) {
    farmLoaded.value = false
    moreActionsVisible.value = false
    farmStore.clearFarmData()
    statusStore.clearAccountScopedData()
  }
  refresh()
}, { immediate: true })

watch(() => currentAccount.value?.running, () => {
  refresh()
})

const { pause, resume } = useIntervalFn(() => {
  if (lands.value) {
    lands.value = lands.value.map((l: any) =>
      l.matureInSec > 0 ? { ...l, matureInSec: l.matureInSec - 1 } : l,
    )
  }
}, 1000)

const { pause: pauseRefresh, resume: resumeRefresh } = useIntervalFn(refresh, 60000)
resume()
resumeRefresh()
onUnmounted(() => {
  pause()
  pauseRefresh()
})
</script>

<template>
  <div class="farm-panel" @click="moreActionsVisible = false">
    <div class="farm-actions">
      <div class="farm-actions-title">
        <span>农场操作</span>
      </div>

      <div class="farm-actionbar">
        <button
          type="button"
          class="farm-action-primary"
          :disabled="operating || !currentAccountId"
          title="依次收获、照料、补种并升级土地"
          @click="handleOperate('all')"
        >
          <span class="farm-action-icon i-carbon-checkmark-outline" />
          <span>处理农场</span>
        </button>

        <span class="farm-action-separator" aria-hidden="true" />

        <button
          v-for="op in operations"
          :key="op.type"
          type="button"
          class="farm-action-button"
          :disabled="operating || !currentAccountId"
          :title="op.title || op.label"
          @click="handleOperate(op.type)"
        >
          <span :class="op.icon" class="farm-action-icon" />
          <span>{{ op.label }}</span>
        </button>

        <div class="farm-more-actions">
          <button
            type="button"
            class="farm-action-button farm-action-more-trigger"
            :class="{ 'is-open': moreActionsVisible }"
            :disabled="operating || !currentAccountId"
            aria-haspopup="menu"
            :aria-expanded="moreActionsVisible"
            @click.stop="moreActionsVisible = !moreActionsVisible"
          >
            <span class="farm-action-icon i-carbon-overflow-menu-horizontal" />
            <span>更多</span>
          </button>

          <div v-if="moreActionsVisible" class="farm-more-menu" role="menu" @click.stop>
            <button type="button" role="menuitem" @click="handleRemoveAllPlants">
              <span class="farm-menu-icon i-carbon-trash-can" />
              <span>清理全部作物</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- 加载 -->
    <div v-if="showInitialLoading" class="farm-loading">
      <div class="i-svg-spinners-90-ring-with-bg text-3xl" :style="{ color: 'var(--theme-primary)' }" />
    </div>

    <!-- 未登录 -->
    <div v-else-if="!currentAccountId" class="farm-empty">
      <div class="i-carbon-user-offline text-4xl opacity-20" />
      <div class="empty-text">
        未登录账号
      </div>
    </div>

    <!-- 无数据 -->
    <div v-else-if="!lands || lands.length === 0" class="farm-empty">
      <div class="empty-text">
        暂无土地数据
      </div>
    </div>

    <!-- 未连接 -->
    <div v-else-if="currentStatusReady && !status?.connection?.connected" class="farm-empty">
      <div class="i-carbon-connection-signal-off text-4xl opacity-20" />
      <div class="empty-text">
        账号未登录
      </div>
      <div class="empty-sub">
        请先运行账号或检查网络连接
      </div>
    </div>

    <!-- 土地网格 -->
    <div v-else class="land-grid">
      <LandCard
        v-for="land in lands"
        :key="land.id"
        :land="land"
        @fertilize="handleLandFertilize"
        @remove="handleLandRemove"
      />
    </div>

    <ConfirmModal
      :show="confirmVisible"
      :title="confirmConfig.title"
      :message="confirmConfig.message"
      :type="confirmConfig.type"
      @confirm="executeOperate"
      @close="confirmVisible = false"
      @cancel="confirmVisible = false"
    />
  </div>
</template>

<style scoped>
.farm-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.farm-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  min-height: 56px;
  padding: 8px 10px 8px 14px;
  border: 1px solid var(--theme-border);
  border-radius: 10px;
  background: color-mix(in srgb, var(--theme-glass) 88%, transparent);
}

.farm-actions-title {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--theme-text);
  font-size: 13px;
  font-weight: 700;
  white-space: nowrap;
}

.farm-actionbar {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 2px;
  min-width: 0;
}

.farm-action-icon {
  flex: 0 0 auto;
  font-size: 15px;
  line-height: 1;
}

.farm-action-primary,
.farm-action-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  height: 34px;
  padding: 0 10px;
  border: 0;
  border-radius: 6px;
  cursor: pointer;
  font-family: inherit;
  font-size: 12px;
  font-weight: 600;
  line-height: 1;
  white-space: nowrap;
  transition:
    background-color 0.16s ease,
    color 0.16s ease,
    transform 0.16s ease;
  user-select: none;
}

.farm-action-primary {
  padding: 0 13px;
  background: var(--theme-primary);
  color: #fff;
  box-shadow: 0 1px 2px color-mix(in srgb, var(--theme-primary) 28%, transparent);
}

.farm-action-primary:hover {
  background: color-mix(in srgb, var(--theme-primary) 86%, #000);
}

.farm-action-button {
  color: var(--theme-text-secondary);
}

.farm-action-button:hover,
.farm-action-button.is-open {
  background: color-mix(in srgb, var(--theme-text) 7%, transparent);
  color: var(--theme-text);
}

.farm-action-primary:active,
.farm-action-button:active {
  transform: translateY(1px);
}

.farm-action-primary:disabled,
.farm-action-button:disabled {
  opacity: 0.4;
  cursor: not-allowed;
  transform: none;
}

.farm-action-separator {
  width: 1px;
  height: 18px;
  margin: 0 5px;
  background: var(--theme-border);
}

.farm-more-actions {
  position: relative;
}

.farm-more-menu {
  position: absolute;
  z-index: 20;
  top: calc(100% + 6px);
  right: 0;
  min-width: 156px;
  padding: 4px;
  border: 1px solid var(--theme-border);
  border-radius: 8px;
  background: var(--surface-1, #fff);
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.16);
}

.farm-more-menu button {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-height: 32px;
  padding: 0 9px;
  border: 0;
  border-radius: 5px;
  background: transparent;
  color: #dc2626;
  cursor: pointer;
  font-family: inherit;
  font-size: 12px;
  text-align: left;
}

.farm-more-menu button:hover {
  background: color-mix(in srgb, #ef4444 9%, transparent);
}

.farm-menu-icon {
  font-size: 14px;
}

/* ===== 土地网格 ===== */
.land-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  grid-auto-flow: dense;
  gap: 10px;
}
@media (max-width: 480px) {
  .farm-actions {
    align-items: stretch;
    flex-direction: column;
    gap: 8px;
    padding: 10px;
  }

  .farm-actionbar {
    justify-content: flex-start;
    flex-wrap: wrap;
  }

  .land-grid {
    grid-template-columns: repeat(3, 1fr);
    gap: 8px;
  }
}

/* ===== 空/加载状态 ===== */
.farm-loading,
.farm-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 0;
  gap: 8px;
}
.empty-text {
  font-size: 14px;
  color: var(--theme-text-secondary);
}
.empty-sub {
  font-size: 12px;
  color: var(--theme-text-secondary);
  opacity: 0.6;
}
</style>
