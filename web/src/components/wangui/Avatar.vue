<script setup lang="ts">
import { computed, ref, watch } from 'vue'

const props = withDefaults(
  defineProps<{
    src?: string
    name?: string
    size?: number
    rounded?: 'lg' | 'xl' | '2xl' | 'full'
  }>(),
  { src: '', name: '', size: 40, rounded: 'xl' },
)

const errored = ref(false)
watch(
  () => props.src,
  () => (errored.value = false),
)

const initial = computed(() => (props.name || '?').charAt(0))
const showImg = computed(() => !!props.src && !errored.value)
const roundedClass = computed(
  () =>
    ({
      'lg': 'rounded-lg',
      'xl': 'rounded-xl',
      '2xl': 'rounded-2xl',
      'full': 'rounded-full',
    })[props.rounded],
)
const fontSize = computed(() => `${Math.round(props.size * 0.45)}px`)
</script>

<template>
  <div
    class="relative flex shrink-0 items-center justify-center overflow-hidden bg-emerald-500/15 ring-1 ring-emerald-500/30"
    :class="roundedClass"
    :style="{ width: `${size}px`, height: `${size}px` }"
  >
    <img
      v-if="showImg"
      :src="src"
      :alt="name"
      referrerpolicy="no-referrer"
      class="absolute inset-0 h-full w-full object-cover"
      @error="errored = true"
    >
    <span
      v-else
      class="select-none text-emerald-400 font-semibold leading-none"
      :style="{ fontSize }"
    >
      {{ initial }}
    </span>
  </div>
</template>
