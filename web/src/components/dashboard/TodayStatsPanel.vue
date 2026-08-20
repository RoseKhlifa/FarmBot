<!-- eslint-disable style/max-statements-per-line -->
<script setup lang="ts">
defineProps<{ operations: Record<string, number>, rows: { key: string }[][], expanded: boolean, disconnected: boolean, getName: (key: string) => string, getIcon: (key: string) => string, getColor: (key: string) => string }>()
const emit = defineEmits<{ toggle: [] }>()
</script>

<template>
  <div class="overview-card p-5">
    <div class="mb-3 flex items-center justify-between">
      <div class="flex items-center gap-2 text-sm text-gray-500">
        <div class="i-carbon-chart-column" /><span>今日统计</span>
      </div>
      <button v-if="Object.keys(operations).length" class="flex items-center gap-1 text-xs text-gray-400 hover:text-blue-500" @click="emit('toggle')">
        {{ expanded ? '收起' : '展开' }}<div class="i-carbon-chevron-down text-sm" :class="{ 'rotate-180': expanded }" />
      </button>
    </div>
    <div v-if="disconnected" class="flex flex-col items-center justify-center gap-4 py-8 text-center text-gray-500">
      <div class="i-carbon-connection-signal-off text-4xl text-gray-400" /><div class="text-base font-medium">
        账号未登录
      </div><div class="text-sm text-gray-400">
        请先运行账号或检查网络连接。
      </div>
    </div>
    <div v-else-if="!Object.keys(operations).length" class="flex flex-col items-center justify-center gap-3 py-6 text-center">
      <div class="i-carbon-chart-column text-3xl text-gray-300" /><div class="text-sm font-medium">
        暂无主动作统计
      </div><div class="text-xs text-gray-400">
        通常是刚启动、刚切换账号，或本轮巡查尚未完成。
      </div>
    </div>
    <div v-else class="flex flex-col gap-2">
      <div v-for="(row, ri) in rows" :key="ri" class="flex gap-2">
        <div v-for="cell in row" :key="cell.key" class="flex flex-1 items-center justify-between rounded-lg px-3 py-2" :class="cell.key ? 'ui-subtle-panel' : 'invisible'">
          <template v-if="cell.key">
            <div class="flex items-center gap-1.5">
              <div class="text-sm" :class="[getIcon(cell.key), getColor(cell.key)]" /><span class="text-xs text-gray-500">{{ getName(cell.key) }}</span>
            </div><span class="text-sm font-bold">{{ operations[cell.key] }}</span>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>
