<!-- eslint-disable style/max-statements-per-line -->
<script setup lang="ts">
import type { InteractRecord } from '@/stores/friend'
import { formatInteractTime, getInteractAvatar, getInteractAvatarKey, getInteractBadgeClass } from '@/composables/useFriendFormatters'

defineProps<{
  records: InteractRecord[]
  filteredRecords: InteractRecord[]
  visibleRecords: InteractRecord[]
  filter: string
  filters: { key: string, label: string }[]
  loading: boolean
  error: string
  avatarErrorKeys: Set<string>
}>()

const emit = defineEmits<{
  'update:filter': [value: string]
  'refresh': []
  'avatarError': [record: InteractRecord]
}>()
</script>

<template>
  <div class="space-y-4">
    <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <div class="rounded-2xl bg-white px-4 py-3 shadow dark:bg-gray-800">
        <div class="text-xs text-gray-500 dark:text-gray-400">
          访客总数
        </div><div class="mt-1 text-lg font-semibold">
          {{ records.length }}
        </div>
      </div>
      <div class="rounded-2xl bg-white px-4 py-3 shadow dark:bg-gray-800">
        <div class="text-xs text-gray-500 dark:text-gray-400">
          偷菜记录
        </div><div class="mt-1 text-lg font-semibold">
          {{ records.filter(record => Number(record?.actionType) === 1).length }}
        </div>
      </div>
      <div class="rounded-2xl bg-white px-4 py-3 shadow dark:bg-gray-800">
        <div class="text-xs text-gray-500 dark:text-gray-400">
          帮忙记录
        </div><div class="mt-1 text-lg font-semibold">
          {{ records.filter(record => Number(record?.actionType) === 2).length }}
        </div>
      </div>
      <div class="rounded-2xl bg-white px-4 py-3 shadow dark:bg-gray-800">
        <div class="text-xs text-gray-500 dark:text-gray-400">
          捣乱记录
        </div><div class="mt-1 text-lg font-semibold">
          {{ records.filter(record => Number(record?.actionType) === 3).length }}
        </div>
      </div>
    </div>
    <div class="flex flex-wrap items-center gap-2">
      <button v-for="item in filters" :key="item.key" class="rounded-full px-3 py-1 text-xs transition" :class="filter === item.key ? 'text-white' : 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'" :style="filter === item.key ? { backgroundColor: 'var(--theme-primary)' } : {}" @click="emit('update:filter', item.key)">
        {{ item.label }}
      </button>
      <button class="rounded bg-gray-100 px-3 py-1.5 text-xs text-gray-600 transition disabled:cursor-not-allowed dark:bg-gray-700 hover:bg-gray-200 dark:text-gray-300 disabled:opacity-60 dark:hover:bg-gray-600" :disabled="loading" @click="emit('refresh')">
        {{ loading ? '刷新中...' : '刷新' }}
      </button>
      <div class="text-xs text-gray-400">
        仅展示最近 50 条访客记录
      </div>
    </div>
    <div v-if="error" class="rounded-lg bg-red-50 px-4 py-6 text-center text-sm text-red-600 dark:bg-red-900/20 dark:text-red-300">
      {{ error }}
    </div>
    <div v-else-if="visibleRecords.length === 0" class="rounded-lg bg-white p-8 text-center text-gray-500 shadow dark:bg-gray-800">
      <div class="i-carbon-user-activity mx-auto mb-3 text-4xl text-gray-300" /><div class="text-base text-gray-700 font-medium dark:text-gray-200">
        暂无访客记录
      </div><p class="mt-2 text-sm text-gray-400">
        有新访客后会同步展示在这里，QQ 账号也会用它补充已知 GID；如果这里长期为空，好友同步也会相对受限。
      </p>
    </div>
    <div v-else class="space-y-3">
      <div v-for="record in visibleRecords" :key="record.key" class="flex items-start gap-3 rounded-lg bg-white p-4 shadow dark:bg-gray-800">
        <div class="h-12 w-12 flex shrink-0 items-center justify-center overflow-hidden rounded-full bg-gray-200 ring-1 ring-gray-100 dark:bg-gray-700 dark:ring-gray-600">
          <img v-if="getInteractAvatar(record) && !avatarErrorKeys.has(getInteractAvatarKey(record))" :src="getInteractAvatar(record)" class="h-full w-full object-cover" loading="lazy" @error="emit('avatarError', record)"><div v-else class="i-carbon-user-avatar text-xl text-gray-400" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="mb-1 flex flex-wrap items-center gap-2">
            <span class="max-w-full truncate text-base text-gray-800 font-medium dark:text-gray-100">{{ record.nick || `GID:${record.visitorGid}` }}</span><span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="getInteractBadgeClass(Number(record.actionType))">{{ record.actionLabel }}</span><span v-if="record.level" class="rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-500 dark:bg-gray-700 dark:text-gray-300">Lv.{{ record.level }}</span><span v-if="record.visitorGid" class="text-xs text-gray-400">GID {{ record.visitorGid }}</span>
          </div><div class="text-sm text-gray-600 dark:text-gray-300">
            {{ record.actionDetail || record.actionLabel }}
          </div>
        </div>
        <div class="shrink-0 text-right text-xs text-gray-400">
          {{ formatInteractTime(record.serverTimeMs) }}
        </div>
      </div>
      <div v-if="filteredRecords.length > visibleRecords.length" class="text-center text-xs text-gray-400">
        仅展示最近 {{ visibleRecords.length }} 条
      </div>
    </div>
  </div>
</template>
