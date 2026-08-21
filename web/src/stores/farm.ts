import { defineStore } from 'pinia'
import { ref } from 'vue'
import { farmApi } from '@/api'
import { isCurrentAccount } from '@/composables/useStaleGuard'

export interface Land {
  id: number
  plantName?: string
  phaseName?: string
  seedImage?: string
  status: string
  matureInSec: number
  needWater?: boolean
  needWeed?: boolean
  needBug?: boolean
  [key: string]: any
}

export const useFarmStore = defineStore('farm', () => {
  const lands = ref<Land[]>([])
  const seeds = ref<any[]>([])
  const summary = ref<any>({})
  const loading = ref(false)

  function clearFarmData() {
    lands.value = []
    seeds.value = []
    summary.value = {}
  }

  async function fetchLands(accountId: string) {
    if (!accountId)
      return
    const requestedId = String(accountId)
    loading.value = true
    try {
      const { data } = await farmApi.getLands()
      if (!isCurrentAccount(requestedId))
        return
      if (data && data.ok) {
        lands.value = Array.isArray(data.data) ? data.data : (data.data?.lands || [])
        summary.value = data.data?.summary || {}
      }
    }
    finally {
      loading.value = false
    }
  }

  async function fetchSeeds(accountId: string) {
    if (!accountId)
      return
    const requestedId = String(accountId)
    const { data } = await farmApi.getSeeds()
    if (!isCurrentAccount(requestedId))
      return
    if (data && data.ok)
      seeds.value = data.data || []
  }

  async function operate(accountId: string, opType: string) {
    if (!accountId)
      return
    await farmApi.operate({ operation: opType })
    await fetchLands(accountId)
  }

  async function fertilizeLand(accountId: string, landId: number) {
    if (!accountId)
      return
    const { data } = await farmApi.fertilize({ landIds: [landId], fertilizerId: 0 })
    await fetchLands(accountId)
    return data
  }

  async function removePlant(accountId: string, landId: number) {
    if (!accountId)
      return
    const { data } = await farmApi.removePlants({ landIds: [landId] })
    await fetchLands(accountId)
    return data
  }

  async function removeAllPlants(accountId: string) {
    if (!accountId)
      return
    const { data } = await farmApi.removeAllPlants()
    await fetchLands(accountId)
    return data
  }

  return { lands, summary, seeds, loading, clearFarmData, fetchLands, fetchSeeds, operate, fertilizeLand, removePlant, removeAllPlants }
})
