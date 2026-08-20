<script setup lang="ts">
import type { CaptureFlowState, LoginPlatform } from '@/composables/useAccountLogin'
import BaseButton from '@/components/ui/BaseButton.vue'
import BaseInput from '@/components/ui/BaseInput.vue'

defineProps<{
  completing: boolean
  copiedField: 'host' | 'port' | ''
  currentStep: string
  deviceSteps: string[]
  editMode: boolean
  error: string
  flow: CaptureFlowState | null
  helpSteps: string[]
  loading: boolean
  nextStep: string
}>()

const emit = defineEmits<{
  cancel: []
  complete: []
  copy: [field: 'host' | 'port']
  openHelp: []
  start: []
}>()

const accountName = defineModel<string>('accountName', { required: true })
const platform = defineModel<LoginPlatform>('platform', { required: true })
const showHelp = defineModel<boolean>('showHelp', { required: true })
const helpMode = defineModel<'first' | 'daily'>('helpMode', { required: true })
const helpDevice = defineModel<'ios' | 'android'>('helpDevice', { required: true })
</script>

<template>
  <div class="space-y-4">
    <BaseInput
      v-model="accountName"
      label="账号备注（可选）"
      placeholder="留空则使用默认账号名"
      :disabled="!!flow"
    />

    <div class="flex flex-col gap-1.5">
      <label class="text-sm font-medium" :style="{ color: 'var(--theme-text)' }">平台</label>
      <div class="grid grid-cols-2 gap-2">
        <button
          type="button"
          class="h-9 rounded-lg px-3 text-sm transition-colors"
          :class="platform === 'qq' ? 'text-white' : 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-200'"
          :style="platform === 'qq' ? { background: 'var(--theme-gradient)' } : {}"
          :disabled="!!flow"
          @click="platform = 'qq'"
        >
          QQ 小程序
        </button>
        <button
          type="button"
          class="h-9 rounded-lg px-3 text-sm transition-colors"
          :class="platform === 'wx' ? 'text-white' : 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-200'"
          :style="platform === 'wx' ? { background: 'var(--theme-gradient)' } : {}"
          :disabled="!!flow"
          @click="platform = 'wx'"
        >
          微信小程序
        </button>
      </div>
    </div>

    <button
      v-if="!flow"
      type="button"
      class="h-11 w-full flex items-center justify-between border border-gray-200 rounded-lg px-3 text-left text-sm dark:border-gray-700"
      :style="{ color: 'var(--theme-text)' }"
      @click="emit('openHelp')"
    >
      <span class="flex items-center gap-2">
        <span class="i-carbon-help" :style="{ color: 'var(--theme-primary)' }" />
        使用说明
      </span>
      <span class="i-carbon-chevron-right opacity-60" />
    </button>

    <div v-if="error" class="rounded-lg bg-red-50 p-3 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-300">
      {{ error }}
    </div>

    <div v-if="!flow" class="flex flex-col items-center gap-3 py-4">
      <div class="h-16 w-16 flex items-center justify-center rounded-full bg-gray-100 dark:bg-gray-800">
        <div class="i-carbon-data-connected text-3xl" :style="{ color: 'var(--theme-primary)' }" />
      </div>
      <BaseButton variant="primary" :loading="loading" @click="emit('start')">
        开始抓取
      </BaseButton>
    </div>

    <template v-else>
      <div class="rounded-lg px-3 py-3 text-sm" style="background-color: color-mix(in srgb, var(--theme-primary) 10%, transparent); color: var(--theme-text);">
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="text-xs opacity-60">
              当前步骤
            </div>
            <div class="mt-1 break-words font-semibold">
              {{ currentStep }}
            </div>
            <div class="mt-1 break-words text-xs opacity-70">
              下一步：{{ nextStep }}
            </div>
          </div>
          <button
            type="button"
            class="h-8 w-8 flex flex-none items-center justify-center rounded-lg hover:bg-black/5 dark:hover:bg-white/10"
            title="使用说明"
            @click="emit('openHelp')"
          >
            <span class="i-carbon-help text-lg" />
          </button>
        </div>
      </div>

      <div class="grid grid-cols-2 gap-2 text-sm">
        <div class="min-w-0 flex items-center justify-between gap-1 border border-gray-200 rounded-lg px-3 py-3 dark:border-gray-700">
          <div class="min-w-0">
            <div class="text-xs opacity-60" :style="{ color: 'var(--theme-text)' }">
              代理服务器
            </div>
            <div class="mt-1 break-all font-semibold" :style="{ color: 'var(--theme-text)' }">
              {{ flow.publicInfo.host || '-' }}
            </div>
          </div>
          <BaseButton
            variant="ghost"
            size="sm"
            :title="copiedField === 'host' ? '已复制' : '复制代理服务器'"
            class="flex-none !px-2"
            @click="emit('copy', 'host')"
          >
            <span :class="copiedField === 'host' ? 'i-carbon-checkmark text-green-600' : 'i-carbon-copy'" />
          </BaseButton>
        </div>
        <div class="min-w-0 flex items-center justify-between gap-1 border border-gray-200 rounded-lg px-3 py-3 dark:border-gray-700">
          <div class="min-w-0">
            <div class="text-xs opacity-60" :style="{ color: 'var(--theme-text)' }">
              代理端口
            </div>
            <div class="mt-1 font-semibold" :style="{ color: 'var(--theme-text)' }">
              {{ flow.publicInfo.mitmPort || '-' }}
            </div>
          </div>
          <BaseButton
            variant="ghost"
            size="sm"
            :title="copiedField === 'port' ? '已复制' : '复制代理端口'"
            class="flex-none !px-2"
            @click="emit('copy', 'port')"
          >
            <span :class="copiedField === 'port' ? 'i-carbon-checkmark text-green-600' : 'i-carbon-copy'" />
          </BaseButton>
        </div>
      </div>

      <div class="rounded-lg bg-gray-50 p-3 text-sm dark:bg-gray-800">
        <div class="flex items-center justify-between gap-3">
          <span :style="{ color: 'var(--theme-text)' }">Code</span>
          <span :class="flow.codeCaptured ? 'text-green-600 dark:text-green-400' : 'text-amber-600 dark:text-amber-400'">
            {{ flow.codeCaptured ? '已获取' : '等待中' }}
          </span>
        </div>
        <div v-if="flow.platform === 'qq'" class="mt-2 flex items-center justify-between gap-3">
          <span :style="{ color: 'var(--theme-text)' }">好友 GID</span>
          <span :style="{ color: 'var(--theme-primary)' }">{{ flow.friendCount }} 个</span>
        </div>
        <div class="mt-2 flex items-center justify-between gap-3">
          <span :style="{ color: 'var(--theme-text)' }">剩余时间</span>
          <span :style="{ color: 'var(--theme-text)' }">{{ flow.publicInfo.remainingSec }} 秒</span>
        </div>
      </div>

      <div class="sticky bottom-0 z-10 flex flex-wrap justify-end gap-2 border-t border-gray-200 px-4 py-3 -mx-4 dark:border-gray-700" :style="{ background: 'var(--theme-bg)' }">
        <BaseButton
          variant="secondary"
          size="sm"
          :href="flow.publicInfo.certificateUrl"
        >
          <span class="i-carbon-certificate" />
          打开证书
        </BaseButton>
        <BaseButton variant="outline" size="sm" @click="emit('cancel')">
          取消抓取
        </BaseButton>
        <BaseButton
          v-if="flow.codeCaptured"
          variant="primary"
          size="sm"
          :loading="completing"
          @click="emit('complete')"
        >
          {{ editMode ? '立即更新' : '立即添加' }}
        </BaseButton>
      </div>
    </template>
  </div>

  <div
    v-if="showHelp"
    class="fixed inset-0 z-[10001] flex items-end justify-center bg-black/50 md:items-center"
    @click.self="showHelp = false"
  >
    <div class="max-h-[78vh] max-w-md w-full flex flex-col overflow-hidden rounded-t-lg shadow-2xl md:rounded-lg" :style="{ background: 'var(--theme-bg)' }">
      <div class="h-14 flex flex-none items-center justify-between border-b border-gray-200 px-4 dark:border-gray-700">
        <h4 class="text-base font-semibold" :style="{ color: 'var(--theme-text)' }">
          抓包登录使用说明
        </h4>
        <BaseButton variant="ghost" class="!h-9 !w-9 !p-0" title="关闭使用说明" @click="showHelp = false">
          <span class="i-carbon-close text-lg" />
        </BaseButton>
      </div>

      <div class="flex-1 overflow-y-auto p-4">
        <div class="grid grid-cols-2 gap-2">
          <button
            type="button"
            class="h-9 rounded-lg px-3 text-sm transition-colors"
            :class="helpMode === 'first' ? 'text-white' : 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-200'"
            :style="helpMode === 'first' ? { background: 'var(--theme-gradient)' } : {}"
            @click="helpMode = 'first'"
          >
            首次使用
          </button>
          <button
            type="button"
            class="h-9 rounded-lg px-3 text-sm transition-colors"
            :class="helpMode === 'daily' ? 'text-white' : 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-200'"
            :style="helpMode === 'daily' ? { background: 'var(--theme-gradient)' } : {}"
            @click="helpMode = 'daily'"
          >
            已装证书
          </button>
        </div>

        <div class="mt-4 divide-y divide-gray-200 dark:divide-gray-700">
          <div v-for="(step, index) in helpSteps" :key="step" class="flex items-start gap-3 py-3 first:pt-0">
            <span class="h-6 w-6 flex flex-none items-center justify-center rounded-full text-xs text-white font-semibold" :style="{ background: 'var(--theme-primary)' }">
              {{ index + 1 }}
            </span>
            <span class="min-w-0 break-words text-sm leading-6" :style="{ color: 'var(--theme-text)' }">
              {{ step }}
            </span>
          </div>
        </div>

        <div v-if="helpMode === 'first'" class="mt-3 border-t border-gray-200 pt-4 dark:border-gray-700">
          <div class="mb-3 text-sm font-semibold" :style="{ color: 'var(--theme-text)' }">
            证书安装帮助
          </div>
          <div class="grid grid-cols-2 gap-2">
            <button
              type="button"
              class="h-9 rounded-lg px-3 text-sm transition-colors"
              :class="helpDevice === 'ios' ? 'text-white' : 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-200'"
              :style="helpDevice === 'ios' ? { background: 'var(--theme-gradient)' } : {}"
              @click="helpDevice = 'ios'"
            >
              iPhone / iPad
            </button>
            <button
              type="button"
              class="h-9 rounded-lg px-3 text-sm transition-colors"
              :class="helpDevice === 'android' ? 'text-white' : 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-200'"
              :style="helpDevice === 'android' ? { background: 'var(--theme-gradient)' } : {}"
              @click="helpDevice = 'android'"
            >
              Android
            </button>
          </div>
          <div class="mt-3 space-y-2">
            <div v-for="(step, index) in deviceSteps" :key="step" class="flex items-start gap-2 text-xs leading-5" :style="{ color: 'var(--theme-text)' }">
              <span class="flex-none opacity-60">{{ index + 1 }}.</span>
              <span class="break-words">{{ step }}</span>
            </div>
          </div>
        </div>

        <div class="mt-4 rounded-lg bg-amber-50 px-3 py-3 text-xs text-amber-800 leading-5 dark:bg-amber-900/20 dark:text-amber-200">
          <div>每次任务的代理端口可能变化，请以当前页面显示为准。</div>
          <div class="mt-1">
            服务端会自动释放代理，但账号完成后仍需在手机上手动关闭 Wi-Fi 代理。
          </div>
          <div v-if="platform === 'wx'" class="mt-1">
            微信抓取成功后无法继续进入农场属于正常现象。
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
