<script setup lang="ts">
import BaseButton from '@/components/ui/BaseButton.vue'
import BaseInput from '@/components/ui/BaseInput.vue'

defineProps<{
  editMode: boolean
  error: string
  loading: boolean
  tokenMasked: string
}>()

const emit = defineEmits<{
  cancel: []
  submit: []
}>()

const apiBase = defineModel<string>('apiBase', { required: true })
const apiToken = defineModel<string>('apiToken', { required: true })
const openid = defineModel<string>('openid', { required: true })
const accountName = defineModel<string>('accountName', { required: true })
const autoReconnect = defineModel<boolean>('autoReconnect', { required: true })
const reconnectDelayMin = defineModel<number>('reconnectDelayMin', { required: true })
const reconnectMaxAttempts = defineModel<number>('reconnectMaxAttempts', { required: true })
</script>

<template>
  <div class="space-y-4">
    <div class="text-sm opacity-70" :style="{ color: 'var(--theme-text)' }">
      第三方 YYB：填接口地址、APITOKEN、OPENID 即可获取 code 自动登录并启动；被踢或异地登录后按账号独立的重连设置恢复连接。
    </div>

    <BaseInput
      v-model="apiBase"
      label="接口地址"
      placeholder="http://211.154.25.123:28999"
    />
    <BaseInput
      v-model="apiToken"
      label="APITOKEN"
      :placeholder="tokenMasked ? '留空则不修改当前 Token' : '第三方接口 APITOKEN'"
    />
    <BaseInput
      v-model="openid"
      label="OPENID"
      placeholder="第三方账号 openid"
    />
    <BaseInput
      v-model="accountName"
      label="账号备注（可选）"
      placeholder="留空则使用 openid 后四位"
    />

    <div class="border border-gray-200 rounded-lg p-3 space-y-3 dark:border-gray-700">
      <label class="flex items-center gap-2 text-sm" :style="{ color: 'var(--theme-text)' }">
        <input v-model="autoReconnect" type="checkbox">
        启用离线自动重连
      </label>
      <div class="grid grid-cols-2 gap-3">
        <BaseInput
          v-model="reconnectDelayMin"
          label="离线几分钟后重连"
          type="number"
          min="1"
        />
        <BaseInput
          v-model="reconnectMaxAttempts"
          label="失败几次后停止"
          type="number"
          min="1"
        />
      </div>
      <div class="text-xs opacity-70" :style="{ color: 'var(--theme-text)' }">
        账号离线后，等待 {{ reconnectDelayMin || 5 }} 分钟重新获取 code 并重连，失败 {{ reconnectMaxAttempts || 3 }} 次后停止。
      </div>
    </div>

    <div v-if="error" class="text-sm text-red-500">
      {{ error }}
    </div>

    <div class="flex justify-end gap-2 pt-2">
      <BaseButton variant="outline" @click="emit('cancel')">
        取消
      </BaseButton>
      <BaseButton
        variant="primary"
        :loading="loading"
        :disabled="!apiBase || !openid || (!editMode && !apiToken)"
        @click="emit('submit')"
      >
        {{ editMode ? '保存' : '添加并登录' }}
      </BaseButton>
    </div>
  </div>
</template>
