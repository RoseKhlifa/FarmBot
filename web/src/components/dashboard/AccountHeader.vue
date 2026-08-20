<!-- eslint-disable style/max-statements-per-line -->
<script setup lang="ts">
import ThemeToggle from '@/components/ThemeToggle.vue'

defineProps<{ name: string, avatar: string, level: number, gold: unknown, coupon: unknown, goldBean: unknown, uptime: string, connected: boolean, expPercent: number, expCurrent: unknown, expNeeded: unknown, expRate: string, timeToLevel: string, allAccountsRunning: boolean, startAllLoading: boolean, startBtnStyle: Record<string, string> }>()
const emit = defineEmits<{ career: [], startAll: [] }>()
</script>

<template>
  <div class="overview-card">
    <div class="flex flex-col">
      <div class="flex items-center justify-between border-b border-gray-100/80 px-5 py-3 dark:border-gray-700/80">
        <ThemeToggle class="shrink-0" /><div class="flex items-center justify-center gap-2">
          <div class="i-fas-user-circle text-blue-500" /><span class="text-sm text-gray-700 font-semibold dark:text-gray-200">QQ农场智能助手</span>
        </div><button class="relative h-8 w-16 flex items-center justify-between rounded-full px-1.5" :disabled="startAllLoading" title="一键启动" @click="emit('startAll')">
          <span class="absolute inset-0 rounded-full" :style="startBtnStyle" /><span class="relative z-1 h-6 w-6 flex items-center justify-center rounded-full bg-white shadow-md" :class="{ 'translate-x-[18px]': allAccountsRunning }"><span v-if="startAllLoading" class="i-svg-spinners-90-ring-with-bg text-sm text-blue-500" /><span v-else-if="allAccountsRunning" class="i-carbon-stop-filled text-sm text-blue-600" /><span v-else class="i-carbon-play text-sm text-blue-600" /></span>
        </button>
      </div><div class="flex gap-4 px-5 py-4">
        <div class="w-[120px] flex shrink-0 flex-col items-center gap-2 pt-2">
          <div class="relative h-[80px] w-[80px] cursor-pointer overflow-hidden rounded-[20px] from-gray-200 to-gray-300 bg-gradient-to-br ring-1 ring-gray-200 dark:from-gray-600 dark:to-gray-700 dark:ring-gray-600" @click="emit('career')">
            <img v-if="avatar" :src="avatar" :alt="name" class="h-full w-full object-cover"><div v-else class="h-full flex items-center justify-center text-3xl text-white font-bold">
              {{ (name || '?').charAt(0).toUpperCase() }}
            </div><div class="absolute left-1/2 border-2 border-white rounded-full bg-blue-500 px-2 py-[1px] text-[10px] text-white font-semibold -bottom-[3px] -translate-x-1/2 dark:border-gray-800">
              Lv.{{ level }}
            </div>
          </div><div class="max-w-[100px] truncate text-xs text-gray-800 font-semibold dark:text-gray-200">
            {{ name }}
          </div><div class="w-full px-1">
            <div class="h-1.5 w-full overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
              <div class="h-full rounded-full bg-blue-500" :style="{ width: `${expPercent}%` }" />
            </div><div class="mt-0.5 text-center text-[9px] text-gray-400">
              EXP {{ expCurrent }} / {{ expNeeded }}
            </div><div class="mt-0.5 text-center text-[9px] text-gray-400">
              效率: {{ expRate }}
            </div><div class="text-center text-[9px] text-gray-400">
              {{ timeToLevel }}
            </div>
          </div>
        </div><div class="min-w-0 flex flex-1 flex-col">
          <div class="flex items-center border-b border-gray-100/80 py-2.5">
            <span class="w-16 text-xs text-gray-500">金币</span><span class="flex-1 text-right text-sm text-yellow-600 font-bold">{{ gold }}</span>
          </div><div class="flex items-center border-b border-gray-100/80 py-2.5">
            <span class="w-16 text-xs text-gray-500">点券</span><span class="flex-1 text-right text-sm text-emerald-500 font-bold">{{ coupon }}</span>
          </div><div class="flex items-center border-b border-gray-100/80 py-2.5">
            <span class="w-16 text-xs text-gray-500">金豆</span><span class="flex-1 text-right text-sm text-amber-500 font-bold">{{ goldBean }}</span>
          </div><div class="flex items-center py-2.5 text-sm font-bold">
            <span class="w-16 text-xs text-gray-500">在线</span><span class="flex flex-1 items-center justify-end gap-2"><span class="h-2 w-2 rounded-full" :class="connected ? 'bg-green-500' : 'bg-red-500'" />{{ uptime }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
