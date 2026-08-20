<!-- eslint-disable style/max-statements-per-line -->
<script setup lang="ts">
defineProps<{ show: boolean, gids: { gid: number, synced: boolean }[], total: number, synced: number, unsynced: number, search: string, saving: boolean }>()
const emit = defineEmits<{ 'update:search': [value: string], 'close': [], 'remove': [gid: number], 'removeUnsynced': [] }>()
</script>

<template>
  <Teleport to="body">
    <div v-if="show" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" @click.self="emit('close')">
      <div class="max-h-[80vh] max-w-2xl w-full flex flex-col rounded-lg bg-white shadow-xl dark:bg-gray-800">
        <div class="flex shrink-0 items-center justify-between border-b border-gray-200 p-4 dark:border-gray-700">
          <div>
            <h3 class="text-lg text-gray-800 font-semibold dark:text-gray-100">
              已导入的 GID 列表
            </h3><p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              共 {{ total }} 个 GID，<span class="text-yellow-600 dark:text-yellow-400">已同步 {{ synced }} 个</span>，<span class="text-red-600 dark:text-red-400">未同步 {{ unsynced }} 个</span>
            </p>
          </div><button class="rounded-lg p-2 text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700" @click="emit('close')">
            <div class="i-carbon-close text-xl" />
          </button>
        </div><div class="shrink-0 border-b border-gray-200 p-4 dark:border-gray-700">
          <div class="flex gap-2">
            <input :value="search" type="text" placeholder="搜索 GID..." class="flex-1 border border-gray-300 rounded-lg bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-700" @input="emit('update:search', ($event.target as HTMLInputElement).value)"><button class="shrink-0 rounded-lg bg-red-100 px-3 py-2 text-sm text-red-700 disabled:opacity-50" :disabled="saving || unsynced === 0" @click="emit('removeUnsynced')">
              <div v-if="saving" class="i-svg-spinners-90-ring-with-bg mr-1 inline-block align-text-bottom" />删除未同步 ({{ unsynced }})
            </button>
          </div>
        </div><div class="flex-1 overflow-y-auto p-4">
          <div v-if="gids.length === 0" class="py-8 text-center text-gray-500 dark:text-gray-400">
            暂无数据
          </div><div v-else class="grid gap-2 lg:grid-cols-3 sm:grid-cols-2">
            <div v-for="item in gids" :key="item.gid" class="flex items-center justify-between border rounded-lg p-2" :class="item.synced ? 'border-yellow-300 bg-yellow-50 dark:border-yellow-700/50 dark:bg-yellow-900/20' : 'border-red-300 bg-red-50 dark:border-red-700/50 dark:bg-red-900/20'">
              <div class="flex items-center gap-2">
                <span class="text-sm font-mono" :class="item.synced ? 'text-yellow-700 dark:text-yellow-400' : 'text-red-700 dark:text-red-400'">{{ item.gid }}</span><span class="rounded px-1 py-0.5 text-xs" :class="item.synced ? 'bg-yellow-200 text-yellow-700' : 'bg-red-200 text-red-700'">{{ item.synced ? '已同步' : '未同步' }}</span>
              </div><button class="rounded p-1 text-gray-400 hover:text-red-500" :disabled="saving" @click="emit('remove', item.gid)">
                <div class="i-carbon-trash-can text-sm" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>
