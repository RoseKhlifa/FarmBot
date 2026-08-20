<!-- eslint-disable style/max-statements-per-line -->
<script setup lang="ts">
defineProps<{
  blacklist: any[]
  friendsCount: number
}>()

const emit = defineEmits<{
  update: [gid: number, options: { skipSteal?: boolean, skipHelp?: boolean }]
  remove: [gid: number]
}>()
</script>

<template>
  <div class="space-y-4">
    <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
      <div class="rounded-2xl bg-white px-4 py-3 shadow dark:bg-gray-800">
        <div class="text-xs text-gray-500 dark:text-gray-400">
          黑名单数量
        </div><div class="mt-1 text-lg font-semibold">
          {{ blacklist.length }}
        </div>
      </div>
      <div class="rounded-2xl bg-white px-4 py-3 shadow dark:bg-gray-800">
        <div class="text-xs text-gray-500 dark:text-gray-400">
          好友总数
        </div><div class="mt-1 text-lg font-semibold">
          {{ friendsCount }}
        </div>
      </div>
      <div class="rounded-2xl bg-white px-4 py-3 shadow dark:bg-gray-800">
        <div class="text-xs text-gray-500 dark:text-gray-400">
          说明
        </div><div class="mt-1 text-sm text-gray-700 font-medium dark:text-gray-200">
          可为每位好友单独设置不偷 / 不帮忙
        </div>
      </div>
    </div>
    <div class="rounded-lg bg-white p-4 shadow dark:bg-gray-800">
      <p class="text-sm text-gray-500 dark:text-gray-400">
        加入黑名单的好友默认在自动偷菜和帮助时都会被跳过，也可单独关闭「不偷」或「不帮忙」中的某一项。
      </p>
    </div>
    <div v-if="blacklist.length === 0" class="rounded-lg bg-white p-8 text-center text-gray-500 shadow dark:bg-gray-800">
      <div class="i-carbon-list-blocked mx-auto mb-3 text-4xl text-gray-300" /><div class="text-base text-gray-700 font-medium dark:text-gray-200">
        暂无黑名单好友
      </div><p class="mt-2 text-sm text-gray-400">
        被加入黑名单的好友会在自动偷菜和帮助时被跳过，只有明确不想互动的对象才建议放进来。
      </p>
    </div>
    <div v-else class="space-y-2">
      <div v-for="item in blacklist" :key="item.gid" class="flex flex-col gap-3 rounded-lg bg-white p-4 shadow sm:flex-row sm:items-center sm:justify-between dark:bg-gray-800">
        <div class="min-w-0 flex items-center gap-3">
          <div class="h-10 w-10 flex shrink-0 items-center justify-center overflow-hidden rounded-full bg-gray-200 ring-1 ring-gray-100 dark:bg-gray-600 dark:ring-gray-700">
            <img v-if="item.avatarUrl" :src="item.avatarUrl" class="h-full w-full object-cover" loading="lazy" @error="($event.target as HTMLImageElement).style.display = 'none'"><div v-else class="i-carbon-user text-gray-400" />
          </div>
          <div class="min-w-0">
            <div class="truncate">
              <span class="font-medium">{{ item.name || `GID:${item.gid}` }}</span><span class="ml-2 text-sm text-gray-400">({{ item.gid }})</span>
            </div><div class="mt-1.5 flex flex-wrap items-center gap-3">
              <label class="flex cursor-pointer items-center gap-1.5 text-sm text-gray-700 dark:text-gray-200"><input type="checkbox" class="h-4 w-4 border-gray-300 rounded text-blue-600 dark:border-gray-600 dark:bg-gray-700 focus:ring-blue-500" :checked="item.skipSteal" @change="emit('update', item.gid, { skipSteal: ($event.target as HTMLInputElement).checked })">不偷</label><label class="flex cursor-pointer items-center gap-1.5 text-sm text-gray-700 dark:text-gray-200"><input type="checkbox" class="h-4 w-4 border-gray-300 rounded text-blue-600 dark:border-gray-600 dark:bg-gray-700 focus:ring-blue-500" :checked="item.skipHelp" @change="emit('update', item.gid, { skipHelp: ($event.target as HTMLInputElement).checked })">不帮忙</label><span v-if="!item.skipSteal && !item.skipHelp" class="rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-500 dark:bg-gray-700 dark:text-gray-300">当前不跳过任何操作</span>
            </div>
          </div>
        </div>
        <button class="w-full shrink-0 rounded bg-red-100 px-3 py-1.5 text-sm text-red-600 sm:w-auto dark:bg-red-900/30 hover:bg-red-200 dark:text-red-400 dark:hover:bg-red-900/50" @click="emit('remove', item.gid)">
          移出黑名单
        </button>
      </div>
    </div>
  </div>
</template>
