<script setup lang="ts">
import type { AccountEditData, AccountLoginTab } from '@/composables/useAccountLogin'
import { computed, reactive, toRef } from 'vue'
import BaseButton from '@/components/ui/BaseButton.vue'
import { useAccountLogin } from '@/composables/useAccountLogin'
import LoginCaptureTab from './LoginCaptureTab.vue'
import LoginManualTab from './LoginManualTab.vue'
import LoginThirdPartyTab from './LoginThirdPartyTab.vue'
import LoginYybQrTab from './LoginYybQrTab.vue'
import LoginYybTab from './LoginYybTab.vue'

const props = defineProps<{
  show: boolean
  editData?: AccountEditData
}>()

const emit = defineEmits<{
  close: []
  saved: []
}>()

const login = reactive(useAccountLogin({
  show: toRef(props, 'show'),
  editData: toRef(props, 'editData'),
  onClose: () => emit('close'),
  onSaved: () => emit('saved'),
}))

const tabs = computed<Array<{ id: AccountLoginTab, label: string }>>(() => [
  { id: 'manual', label: '手动填码' },
  ...(login.captureEnabled ? [{ id: 'capture' as const, label: '抓包登录' }] : []),
  { id: 'yyb', label: '应用宝' },
  { id: 'yybqr', label: '应用宝扫码' },
  { id: 'yyb3rd', label: '第三方 YYB' },
])
</script>

<template>
  <div v-if="show" class="fixed inset-0 z-[9999] flex items-center justify-center bg-black/50 p-3">
    <div class="max-h-[90vh] max-w-lg w-full overflow-hidden rounded-lg shadow-xl" :style="{ background: 'var(--theme-bg)' }">
      <div class="flex items-center justify-between border-b p-4" :style="{ borderColor: 'color-mix(in srgb, var(--theme-text) 10%, transparent)' }">
        <h3 class="text-lg font-semibold" :style="{ color: 'var(--theme-text)' }">
          {{ editData ? '编辑账号' : '添加账号' }}
        </h3>
        <BaseButton variant="ghost" class="!p-1" title="关闭" @click="login.close">
          <span class="i-carbon-close text-xl" :style="{ color: 'var(--theme-text)' }" />
        </BaseButton>
      </div>

      <div class="max-h-[calc(90vh-80px)] overflow-y-auto p-4">
        <div v-if="login.error" class="mb-4 rounded p-3 text-sm" :style="{ background: 'rgba(239, 68, 68, 0.1)', color: '#ef4444' }">
          {{ login.error }}
        </div>

        <div class="mb-4 overflow-x-auto border-b" :style="{ borderColor: 'color-mix(in srgb, var(--theme-text) 10%, transparent)' }">
          <div class="min-w-max flex">
            <button
              v-for="tab in tabs"
              :key="tab.id"
              type="button"
              class="min-w-24 flex-1 whitespace-nowrap px-3 py-2 text-center text-sm font-medium transition-colors"
              :class="login.activeTab === tab.id ? 'border-b-2' : 'opacity-60'"
              :style="{
                color: login.activeTab === tab.id ? 'var(--theme-primary)' : 'var(--theme-text)',
                borderColor: 'var(--theme-primary)',
              }"
              @click="login.activeTab = tab.id"
            >
              {{ tab.label }}
            </button>
          </div>
        </div>

        <LoginManualTab
          v-if="login.activeTab === 'manual'"
          v-model:name="login.manualForm.name"
          v-model:code="login.manualForm.code"
          v-model:platform="login.manualForm.platform"
          :edit-mode="!!editData"
          :loading="login.loading"
          @cancel="login.close"
          @submit="login.submitManual"
        />

        <LoginCaptureTab
          v-else-if="login.activeTab === 'capture'"
          v-model:account-name="login.captureAccountName"
          v-model:platform="login.capturePlatform"
          v-model:show-help="login.showCaptureHelp"
          v-model:help-mode="login.captureHelpMode"
          v-model:help-device="login.captureHelpDevice"
          :completing="login.captureCompleting"
          :copied-field="login.captureCopiedField"
          :current-step="login.captureCurrentStep"
          :device-steps="login.captureDeviceSteps"
          :edit-mode="!!editData"
          :error="login.captureError"
          :flow="login.captureFlow"
          :help-steps="login.captureHelpSteps"
          :loading="login.captureLoading"
          :next-step="login.captureNextStep"
          @cancel="login.cancelCaptureSession"
          @complete="login.completeCaptureAccount"
          @copy="login.copyCaptureValue"
          @open-help="login.openCaptureHelp"
          @start="login.startCaptureSession"
        />

        <LoginYybTab
          v-else-if="login.activeTab === 'yyb'"
          v-model:api-base="login.yybApiBase"
          v-model:api-key="login.yybApiKey"
          v-model:auto-reconnect="login.yybAutoReconnect"
          v-model:reconnect-delay-min="login.yybReconnectDelayMin"
          v-model:reconnect-max-attempts="login.yybReconnectMaxAttempts"
          v-model:config-expanded="login.yybShowConfigEditor"
          v-model:account-name="login.yybAccountName"
          v-model:selected-openid="login.yybSelectedOpenid"
          :accounts="login.yybAccounts"
          :accounts-loading="login.yybAccountsLoading"
          :configured="login.yybConfigured"
          :config-saving="login.yybConfigSaving"
          :error="login.yybError"
          :login-loading="login.yybLoginLoading"
          @cancel="login.close"
          @copy-token="login.copyYybToken"
          @fetch-accounts="login.fetchYybAccounts"
          @save-config="login.saveYybConfig"
          @submit="login.submitYybLogin"
        />

        <LoginYybQrTab
          v-else-if="login.activeTab === 'yybqr'"
          :configured="login.yybConfigured"
          :error="login.yybQrError"
          :image="login.yybQrImage"
          :loading="login.yybQrLoading"
          :status="login.yybQrStatus"
          @reset="login.resetYybQr"
          @start="login.startYybQrLogin"
        />

        <LoginThirdPartyTab
          v-else
          v-model:api-base="login.yyb3rdApiBase"
          v-model:api-token="login.yyb3rdApiToken"
          v-model:openid="login.yyb3rdOpenid"
          v-model:account-name="login.yyb3rdAccountName"
          v-model:auto-reconnect="login.yyb3rdAutoReconnect"
          v-model:reconnect-delay-min="login.yyb3rdReconnectDelayMin"
          v-model:reconnect-max-attempts="login.yyb3rdReconnectMaxAttempts"
          :edit-mode="!!editData"
          :error="login.yyb3rdError"
          :loading="login.yyb3rdLoading"
          :token-masked="login.yyb3rdTokenMasked"
          @cancel="login.close"
          @submit="login.submitYyb3rdLogin"
        />
      </div>
    </div>
  </div>
</template>
