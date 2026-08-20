<!-- eslint-disable style/max-statements-per-line -->
<script setup lang="ts">
defineProps<{ show: boolean, modelValue: string, saving: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string], 'close': [], 'submit': [] }>()
</script>

<template>
  <Teleport to="body">
    <div v-if="show" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" @click.self="emit('close')">
      <div class="max-w-lg w-full rounded-lg bg-white p-6 shadow-xl dark:bg-gray-800">
        <h3 class="mb-4 text-lg text-gray-800 font-semibold dark:text-gray-100">
          批量新增 GID
        </h3><p class="mb-3 text-sm text-gray-500 dark:text-gray-400">
          支持一行一个或用逗号/空格分隔，自动去重
        </p><textarea :value="modelValue" rows="8" placeholder="每行一个 GID，或用逗号、空格分隔" class="mb-4 w-full border border-gray-300 rounded-lg bg-white p-3 text-sm dark:border-gray-600 focus:border-blue-500 dark:bg-gray-700 dark:text-white" @input="emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)" /><div class="flex justify-end gap-3">
          <button class="border border-gray-300 rounded-lg bg-white px-4 py-2 text-sm dark:border-gray-600 dark:bg-gray-700" @click="emit('close')">
            取消
          </button><button class="rounded-lg px-4 py-2 text-sm text-white disabled:opacity-50" :disabled="saving || !modelValue.trim()" :style="{ backgroundColor: 'var(--theme-primary)' }" @click="emit('submit')">
            <div v-if="saving" class="i-svg-spinners-90-ring-with-bg mr-1 inline-block align-text-bottom" />确认添加
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
