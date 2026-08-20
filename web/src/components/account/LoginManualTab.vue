<script setup lang="ts">
import type { LoginPlatform } from '@/composables/useAccountLogin'
import BaseButton from '@/components/ui/BaseButton.vue'
import BaseInput from '@/components/ui/BaseInput.vue'
import BaseTextarea from '@/components/ui/BaseTextarea.vue'

defineProps<{
  editMode: boolean
  loading: boolean
}>()

const emit = defineEmits<{
  cancel: []
  submit: []
}>()

const name = defineModel<string>('name', { required: true })
const code = defineModel<string>('code', { required: true })
const platform = defineModel<LoginPlatform>('platform', { required: true })
</script>

<template>
  <div class="space-y-4">
    <BaseInput
      v-model="name"
      label="账号备注（可选）"
      placeholder="留空则使用默认账号名"
    />

    <BaseTextarea
      v-model="code"
      label="Code"
      placeholder="请输入登录 Code"
      :rows="3"
    />

    <div v-if="!editMode" class="flex gap-4">
      <label class="flex cursor-pointer items-center gap-2">
        <input
          v-model="platform"
          type="radio"
          value="qq"
          class="h-4 w-4"
          :style="{ accentColor: 'var(--theme-primary)' }"
        >
        <span class="text-sm" :style="{ color: 'var(--theme-text)' }">QQ 小程序</span>
      </label>
      <label class="flex cursor-pointer items-center gap-2">
        <input
          v-model="platform"
          type="radio"
          value="wx"
          class="h-4 w-4"
          :style="{ accentColor: 'var(--theme-primary)' }"
        >
        <span class="text-sm" :style="{ color: 'var(--theme-text)' }">微信小程序</span>
      </label>
    </div>

    <div class="flex justify-end gap-2 pt-4">
      <BaseButton variant="outline" @click="emit('cancel')">
        取消
      </BaseButton>
      <BaseButton variant="primary" :loading="loading" @click="emit('submit')">
        {{ editMode ? '保存' : '添加' }}
      </BaseButton>
    </div>
  </div>
</template>
