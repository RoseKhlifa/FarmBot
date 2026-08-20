<!-- eslint-disable style/max-statements-per-line, style/quote-props -->
<script setup lang="ts">
import { nextTick, reactive, ref, watch } from 'vue'
import BaseButton from '@/components/ui/BaseButton.vue'
import BaseInput from '@/components/ui/BaseInput.vue'
import BaseSelect from '@/components/ui/BaseSelect.vue'

const props = defineProps<{ logs: any[], modules: any[], events: any[], levels: any[], filter: { module: string, event: string, keyword: string, isWarn: string }, clearing: boolean, eventLabel: (event: string) => string, tagClass: (tag: string) => string, msgClass: (tag: string) => string, time: (value: string) => string }>()
const emit = defineEmits<{ filter: [], search: [], clear: [], updateFilter: [value: { module: string, event: string, keyword: string, isWarn: string }] }>()
const filterModel = reactive({ ...props.filter })
watch(filterModel, value => emit('updateFilter', { ...value }), { deep: true })
const container = ref<HTMLElement | null>(null); const autoScroll = ref(true)
function onScroll(event: Event) { const el = event.target as HTMLElement; autoScroll.value = el.scrollHeight - el.scrollTop - el.clientHeight < 50 }
function scrollBottom() {
  nextTick(() => {
    if (container.value && autoScroll.value)
      container.value.scrollTop = container.value.scrollHeight
  })
}
watch(() => container.value, scrollBottom); defineExpose({ scrollBottom })
</script>

<template>
  <div class="overview-card flex flex-1 flex-col p-5 md:overflow-hidden">
    <div class="mb-4 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
      <h3 class="flex shrink-0 items-center gap-2 whitespace-nowrap text-lg font-medium">
        <div class="i-carbon-document" /><span>运行日志</span>
      </h3><div class="flex flex-wrap items-center gap-2 text-sm">
        <BaseSelect v-model="filterModel.module" :options="modules" class="w-32" @change="emit('filter')" /><BaseSelect v-model="filterModel.event" :options="events" class="w-32" @change="emit('filter')" /><BaseSelect v-model="filterModel.isWarn" :options="levels" class="w-32" @change="emit('filter')" /><BaseInput v-model="filterModel.keyword" placeholder="关键词..." class="w-32" clearable @keyup.enter="emit('search')" @clear="emit('search')" /><BaseButton variant="primary" size="sm" @click="emit('search')">
          <div class="i-carbon-search" />
        </BaseButton><BaseButton variant="secondary" size="sm" :loading="clearing" @click="emit('clear')">
          <div class="i-carbon-trash-can mr-1" />清空
        </BaseButton>
      </div>
    </div><div ref="container" class="ui-subtle-panel max-h-[50vh] min-h-0 flex-1 overflow-y-auto rounded-lg p-4 text-sm leading-relaxed font-mono" @scroll="onScroll">
      <div v-if="!logs.length" class="py-8 text-center text-gray-400">
        <div class="i-carbon-document-blank mx-auto mb-3 text-3xl text-gray-300" /><div class="text-sm text-gray-500 dark:text-gray-400">
          暂无日志
        </div>
      </div><div v-for="log in logs" :key="log.ts + log.msg" class="mb-1 break-all" :class="log.recovered ? 'opacity-45' : ''">
        <span class="mr-2 select-none text-gray-400">[{{ time(log.time) }}]</span><span class="mr-2 rounded px-1.5 py-0.5 text-xs font-bold" :class="tagClass(log.tag)">{{ log.tag }}</span><span v-if="log.meta?.event" class="mr-2 rounded bg-blue-50 px-1.5 py-0.5 text-xs text-blue-500">{{ eventLabel(log.meta.event) }}</span><span :class="[msgClass(log.tag), log.recovered ? 'line-through decoration-gray-400' : '']">{{ log.msg }}<span v-if="log.recovered" class="ml-1 text-xs text-green-500">（已恢复）</span></span>
      </div>
    </div>
  </div>
</template>
