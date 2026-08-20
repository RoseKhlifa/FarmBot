import type { Ref } from 'vue'
import { computed, ref } from 'vue'

export const OP_META: Record<string, { label: string, icon: string, color: string }> = {
  harvest: { label: '收获', icon: 'i-carbon-crop-growth', color: 'text-green-500' },
  water: { label: '浇水', icon: 'i-carbon-rain-drop', color: 'text-blue-400' },
  weed: { label: '除草', icon: 'i-carbon-cut', color: 'text-yellow-500' },
  bug: { label: '除虫', icon: 'i-carbon-pest', color: 'text-red-400' },
  farming: { label: '一键务农', icon: 'i-carbon-clean', color: 'text-teal-500' },
  fertilize: { label: '施肥', icon: 'i-carbon-chemistry', color: 'text-emerald-500' },
  plant: { label: '种植', icon: 'i-carbon-tree', color: 'text-lime-500' },
  steal: { label: '偷菜', icon: 'i-carbon-run', color: 'text-orange-500' },
  helpWater: { label: '帮浇水', icon: 'i-carbon-rain-drop', color: 'text-blue-300' },
  goldenBugClear: { label: '清黄金虫', icon: 'i-carbon-clean', color: 'text-amber-500' },
  goldenBugPut: { label: '放黄金虫', icon: 'i-carbon-pest', color: 'text-yellow-500' },
  helpWeed: { label: '帮除草', icon: 'i-carbon-cut', color: 'text-yellow-400' },
  helpBug: { label: '帮除虫', icon: 'i-carbon-pest', color: 'text-red-300' },
  taskClaim: { label: '任务', icon: 'i-carbon-task-complete', color: 'text-indigo-500' },
  sell: { label: '收益', icon: 'i-carbon-money', color: 'text-pink-500' },
  tongQiGift: { label: '同气礼包', icon: 'i-carbon-gift', color: 'text-rose-500' },
}

const DEFAULT_KEYS = ['sell', 'tongQiGift', 'harvest', 'steal', 'plant', 'fertilize']

export function useTodayStats(operations: Ref<Record<string, number> | null | undefined>) {
  const expanded = ref(false)
  const filteredOperations = computed(() => {
    const result: Record<string, number> = {}
    for (const [key, value] of Object.entries(operations.value || {})) {
      if (key !== 'upgrade' && key !== 'levelUp')
        result[key] = Number(value) || 0
    }
    return result
  })
  const rows = computed(() => {
    const allKeys = Object.keys(filteredOperations.value)
    const keys = expanded.value ? DEFAULT_KEYS.concat(allKeys.filter(k => !DEFAULT_KEYS.includes(k))) : DEFAULT_KEYS
    const result: { key: string }[][] = []
    for (let i = 0; i < keys.length; i += 2) result.push([{ key: keys[i] || '' }, { key: keys[i + 1] || '' }])
    return result
  })
  return { expanded, filteredOperations, rows, opMeta: OP_META, getOpName: (key: string | number) => OP_META[String(key)]?.label || String(key), getOpIcon: (key: string | number) => OP_META[String(key)]?.icon || 'i-carbon-circle-dash', getOpColor: (key: string | number) => OP_META[String(key)]?.color || 'text-gray-400' }
}
