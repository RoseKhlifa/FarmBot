<script setup lang="ts">
import type { YybQrStatus } from '@/composables/useAccountLogin'
import BaseButton from '@/components/ui/BaseButton.vue'

defineProps<{
  error: string
  image: string
  loading: boolean
  status: YybQrStatus
}>()

const emit = defineEmits<{
  reset: []
  start: []
}>()
</script>

<template>
  <div class="space-y-4">
    <div v-if="status === 'idle'" class="flex flex-col items-center gap-3 py-4">
      <p class="text-center text-sm opacity-70" :style="{ color: 'var(--theme-text)' }">
        内置应用宝无需配置接口或 Token，点击下方按钮生成二维码后扫码授权。
      </p>
      <BaseButton variant="primary" :loading="loading" @click="emit('start')">
        开始扫码
      </BaseButton>
    </div>

    <div v-else class="border rounded-lg p-4 space-y-3" :style="{ borderColor: 'color-mix(in srgb, var(--theme-text) 15%, transparent)' }">
      <div class="flex items-center justify-between">
        <span class="text-sm font-medium" :style="{ color: 'var(--theme-text)' }">
          应用宝扫码登录
        </span>
        <BaseButton
          v-if="status === 'pending' || status === 'scanned' || status === 'authorizing'"
          variant="ghost"
          size="sm"
          @click="emit('reset')"
        >
          取消
        </BaseButton>
      </div>

      <div v-if="image && status !== 'success'" class="flex justify-center">
        <img :src="image" alt="应用宝二维码" class="max-w-[200px] w-full rounded">
      </div>

      <div class="text-center text-sm" :style="{ color: 'var(--theme-text)' }">
        <span v-if="status === 'loading'">正在生成二维码...</span>
        <span v-else-if="status === 'pending'" class="opacity-70">请使用应用宝扫描二维码</span>
        <span v-else-if="status === 'scanned'" class="text-green-500">已扫描，请在手机上确认授权</span>
        <span v-else-if="status === 'authorizing'" class="opacity-70">正在确认授权...</span>
        <span v-else-if="status === 'success'" class="text-green-500">授权成功，账号已添加到应用宝</span>
        <span v-else-if="status === 'expired'" class="text-red-500">{{ error || '二维码已过期' }}</span>
        <span v-else-if="status === 'error'" class="text-red-500">{{ error }}</span>
      </div>

      <div v-if="status === 'success'" class="text-center">
        <BaseButton variant="primary" size="sm" @click="emit('reset')">
          完成
        </BaseButton>
      </div>
    </div>
  </div>
</template>
