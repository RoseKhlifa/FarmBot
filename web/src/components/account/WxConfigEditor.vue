<script setup lang="ts">
import BaseButton from '@/components/ui/BaseButton.vue'
import BaseInput from '@/components/ui/BaseInput.vue'

defineProps<{
  configured: boolean
  saving: boolean
}>()

const emit = defineEmits<{
  copyToken: []
  save: []
}>()

const apiBase = defineModel<string>('apiBase', { required: true })
const apiKey = defineModel<string>('apiKey', { required: true })
const autoReconnect = defineModel<boolean>('autoReconnect', { required: true })
const reconnectDelayMin = defineModel<number>('reconnectDelayMin', { required: true })
const reconnectMaxAttempts = defineModel<number>('reconnectMaxAttempts', { required: true })
const expanded = defineModel<boolean>('expanded', { required: true })
</script>

<template>
  <div v-if="!configured" class="space-y-3">
    <div class="text-sm opacity-70" :style="{ color: 'var(--theme-text)' }">
      请先配置应用宝接口地址和 API Token
    </div>
    <BaseInput
      v-model="apiBase"
      label="接口地址"
      placeholder="http://你的服务器地址:端口/wxapp/getCode"
    />
    <BaseInput
      v-model="apiKey"
      label="API Token（部署时已自动生成并预填）"
      placeholder="请输入你的应用宝 API Token"
    />
    <BaseButton variant="primary" :loading="saving" @click="emit('save')">
      保存并获取账号列表
    </BaseButton>
  </div>

  <template v-else>
    <div class="border rounded-lg p-3 space-y-2" :style="{ borderColor: 'color-mix(in srgb, var(--theme-text) 15%, transparent)' }">
      <div class="flex items-center justify-between">
        <span class="text-sm font-medium" :style="{ color: 'var(--theme-text)' }">应用宝接口配置</span>
        <BaseButton variant="ghost" size="sm" @click="expanded = !expanded">
          {{ expanded ? '收起' : '编辑' }}
        </BaseButton>
      </div>
      <template v-if="!expanded">
        <div class="text-xs opacity-60" :style="{ color: 'var(--theme-text)' }">
          接口地址
        </div>
        <div class="break-all text-sm" :style="{ color: 'var(--theme-text)' }">
          {{ apiBase }}
        </div>
        <div class="mt-1 text-xs opacity-60" :style="{ color: 'var(--theme-text)' }">
          API Token（已自动生成）
        </div>
        <div class="flex items-center gap-2">
          <code class="flex-1 break-all rounded px-2 py-1 text-sm" :style="{ background: 'color-mix(in srgb, var(--theme-text) 8%, transparent)', color: 'var(--theme-text)' }">{{ apiKey }}</code>
          <BaseButton variant="ghost" size="sm" title="复制 Token" @click="emit('copyToken')">
            复制
          </BaseButton>
        </div>
      </template>
      <template v-else>
        <BaseInput v-model="apiBase" label="接口地址" placeholder="http://127.0.0.1:8450" />
        <BaseInput v-model="apiKey" label="API Token" placeholder="请输入你的应用宝 API Token" />
        <BaseButton variant="primary" size="sm" :loading="saving" @click="emit('save')">
          保存
        </BaseButton>
      </template>
    </div>

    <div class="border rounded-lg p-3 space-y-2" :style="{ borderColor: 'color-mix(in srgb, var(--theme-text) 15%, transparent)' }">
      <div class="flex items-center justify-between">
        <span class="text-sm font-medium" :style="{ color: 'var(--theme-text)' }">
          离线自动重连
        </span>
        <label class="flex cursor-pointer items-center gap-2">
          <input
            v-model="autoReconnect"
            type="checkbox"
            class="h-4 w-4"
            :style="{ accentColor: 'var(--theme-primary)' }"
            @change="emit('save')"
          >
          <span class="text-xs" :style="{ color: 'var(--theme-text)' }">启用</span>
        </label>
      </div>
      <div v-if="autoReconnect" class="flex items-end gap-3">
        <div class="flex-1">
          <label class="mb-1 block text-xs opacity-70" :style="{ color: 'var(--theme-text)' }">离线几分钟后重连</label>
          <input
            v-model.number="reconnectDelayMin"
            type="number"
            min="1"
            max="60"
            class="w-full border rounded-lg px-2 py-1 text-sm"
            :style="{
              borderColor: 'color-mix(in srgb, var(--theme-text) 15%, transparent)',
              background: 'var(--surface-1, #fff)',
              color: 'var(--theme-text)',
            }"
            @change="emit('save')"
          >
        </div>
        <div class="flex-1">
          <label class="mb-1 block text-xs opacity-70" :style="{ color: 'var(--theme-text)' }">最大重试次数</label>
          <input
            v-model.number="reconnectMaxAttempts"
            type="number"
            min="1"
            max="10"
            class="w-full border rounded-lg px-2 py-1 text-sm"
            :style="{
              borderColor: 'color-mix(in srgb, var(--theme-text) 15%, transparent)',
              background: 'var(--surface-1, #fff)',
              color: 'var(--theme-text)',
            }"
            @change="emit('save')"
          >
        </div>
      </div>
      <div v-if="autoReconnect" class="text-xs opacity-50" :style="{ color: 'var(--theme-text)' }">
        账号离线后，等待 {{ reconnectDelayMin }} 分钟自动重新获取 code 并重连，失败 {{ reconnectMaxAttempts }} 次后停止
      </div>
    </div>
  </template>
</template>
