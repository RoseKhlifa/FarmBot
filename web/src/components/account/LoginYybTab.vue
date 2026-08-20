<script setup lang="ts">
import type { YybAccount } from '@/composables/useAccountLogin'
import BaseButton from '@/components/ui/BaseButton.vue'
import BaseInput from '@/components/ui/BaseInput.vue'
import WxConfigEditor from './WxConfigEditor.vue'

defineProps<{
  accounts: YybAccount[]
  accountsLoading: boolean
  configured: boolean
  configSaving: boolean
  error: string
  loginLoading: boolean
}>()

const emit = defineEmits<{
  cancel: []
  copyToken: []
  fetchAccounts: []
  saveConfig: []
  submit: []
}>()

const apiBase = defineModel<string>('apiBase', { required: true })
const apiKey = defineModel<string>('apiKey', { required: true })
const autoReconnect = defineModel<boolean>('autoReconnect', { required: true })
const reconnectDelayMin = defineModel<number>('reconnectDelayMin', { required: true })
const reconnectMaxAttempts = defineModel<number>('reconnectMaxAttempts', { required: true })
const configExpanded = defineModel<boolean>('configExpanded', { required: true })
const accountName = defineModel<string>('accountName', { required: true })
const selectedOpenid = defineModel<string>('selectedOpenid', { required: true })
</script>

<template>
  <div class="space-y-4">
    <WxConfigEditor
      v-model:api-base="apiBase"
      v-model:api-key="apiKey"
      v-model:auto-reconnect="autoReconnect"
      v-model:reconnect-delay-min="reconnectDelayMin"
      v-model:reconnect-max-attempts="reconnectMaxAttempts"
      v-model:expanded="configExpanded"
      :configured="configured"
      :saving="configSaving"
      @copy-token="emit('copyToken')"
      @save="emit('saveConfig')"
    />

    <div v-if="configured" class="space-y-3">
      <div class="flex items-center justify-between">
        <span class="text-sm opacity-70" :style="{ color: 'var(--theme-text)' }">
          接口：{{ apiBase }}
        </span>
        <BaseButton variant="ghost" size="sm" :loading="accountsLoading" @click="emit('fetchAccounts')">
          刷新列表
        </BaseButton>
      </div>

      <div class="border rounded-lg p-3 text-sm" :style="{ borderColor: 'color-mix(in srgb, var(--theme-primary) 25%, transparent)', background: 'color-mix(in srgb, var(--theme-primary) 6%, transparent)', color: 'var(--theme-text)' }">
        需要添加新账号？请切换到「应用宝扫码」标签页，使用应用宝扫码授权登录。
      </div>

      <BaseInput
        v-model="accountName"
        label="账号备注（可选）"
        placeholder="留空则使用应用宝昵称"
      />

      <div v-if="accounts.length > 0" class="max-h-60 overflow-y-auto space-y-2">
        <label
          v-for="account in accounts"
          :key="account.openid"
          class="flex cursor-pointer items-center gap-3 border rounded-lg p-3 transition-colors"
          :style="{
            borderColor: selectedOpenid === account.openid ? 'var(--theme-primary)' : 'color-mix(in srgb, var(--theme-text) 15%, transparent)',
            background: selectedOpenid === account.openid ? 'color-mix(in srgb, var(--theme-primary) 8%, transparent)' : 'transparent',
          }"
        >
          <input
            v-model="selectedOpenid"
            type="radio"
            :value="account.openid"
            class="h-4 w-4"
            :style="{ accentColor: 'var(--theme-primary)' }"
          >
          <div class="min-w-0 flex-1">
            <div class="truncate text-sm font-medium" :style="{ color: 'var(--theme-text)' }">
              {{ account.nickname || account.alias || account.openid }}
            </div>
            <div class="truncate text-xs opacity-60" :style="{ color: 'var(--theme-text)' }">
              openid: {{ account.openid }}
            </div>
          </div>
          <span
            v-if="account.status"
            class="rounded px-2 py-0.5 text-xs"
            :style="{
              background: account.status === 'alive' ? 'color-mix(in srgb, #22c55e 15%, transparent)' : 'color-mix(in srgb, #ef4444 15%, transparent)',
              color: account.status === 'alive' ? '#22c55e' : '#ef4444',
            }"
          >
            {{ account.status === 'alive' ? '在线' : account.status }}
          </span>
        </label>
      </div>

      <div v-else-if="!accountsLoading" class="py-4 text-center text-sm opacity-60" :style="{ color: 'var(--theme-text)' }">
        暂无账号，点击“刷新列表”获取
      </div>

      <div class="flex justify-end gap-2 pt-2">
        <BaseButton variant="outline" @click="emit('cancel')">
          取消
        </BaseButton>
        <BaseButton
          variant="primary"
          :loading="loginLoading"
          :disabled="!selectedOpenid"
          @click="emit('submit')"
        >
          应用宝登录
        </BaseButton>
      </div>
    </div>

    <div v-if="error" class="text-sm text-red-500">
      {{ error }}
    </div>
  </div>
</template>
