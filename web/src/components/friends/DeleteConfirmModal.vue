<!-- eslint-disable style/max-statements-per-line -->
<script setup lang="ts">
defineProps<{ show: boolean, friendsCount: number, guardDogCount: number, targetCount: number, threshold: number | null, skipGuardDog: boolean, password: string, submitting: boolean }>()
const emit = defineEmits<{ 'close': [], 'submit': [], 'update:threshold': [value: number | null], 'update:skipGuardDog': [value: boolean], 'update:password': [value: string] }>()
</script>

<template>
  <Teleport to="body">
    <div v-if="show" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" @click.self="emit('close')">
      <div class="max-w-md w-full rounded-lg bg-white p-6 shadow-xl dark:bg-gray-800">
        <h3 class="mb-2 text-lg text-gray-800 font-semibold dark:text-gray-100">
          一键删除好友
        </h3><p class="mb-4 text-sm text-gray-500 dark:text-gray-400">
          将删除等级小于或等于指定值的所有好友，请谨慎操作！
        </p><div class="mb-4 text-sm text-gray-700 dark:text-gray-200">
          当前好友总数: <span class="font-semibold">{{ friendsCount }}</span>，其中等级 ≤ <span class="text-red-600 font-semibold">{{ Number.isFinite(Number(threshold)) && Number(threshold) > 0 ? threshold : '?' }}</span> 的好友: <span class="font-semibold">{{ targetCount }}</span> 个
        </div><div class="mb-4">
          <label class="mb-1 block text-sm font-medium">删除等级 ≤（含）</label><input :value="threshold ?? ''" type="number" min="1" placeholder="请输入等级，例如: 10" class="w-full border border-gray-300 rounded-lg bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-700" @input="emit('update:threshold', Number(($event.target as HTMLInputElement).value) || null)">
        </div><label class="mb-4 flex cursor-pointer items-center gap-2 text-sm"><input :checked="skipGuardDog" type="checkbox" class="h-4 w-4 border-gray-300 rounded text-blue-600" @change="emit('update:skipGuardDog', ($event.target as HTMLInputElement).checked)"><span>不删除有 <span class="text-red-600 font-medium">护主犬</span> 的好友（共 {{ guardDogCount }} 个）</span></label><div class="mb-6">
          <label class="mb-1 block text-sm font-medium">二级密码（验证身份）</label><input :value="password" type="password" placeholder="请输入二级密码" class="w-full border border-gray-300 rounded-lg bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-700" @input="emit('update:password', ($event.target as HTMLInputElement).value)">
        </div><div class="flex justify-end gap-3">
          <button class="border border-gray-300 rounded-lg bg-white px-4 py-2 text-sm dark:border-gray-600 dark:bg-gray-700" :disabled="submitting" @click="emit('close')">
            取消
          </button><button class="rounded-lg px-4 py-2 text-sm text-white disabled:opacity-50" :disabled="submitting || !password.trim() || targetCount === 0" :style="{ backgroundColor: '#ef4444' }" @click="emit('submit')">
            <div v-if="submitting" class="i-svg-spinners-90-ring-with-bg mr-1 inline-block align-text-bottom" />确定删除
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
