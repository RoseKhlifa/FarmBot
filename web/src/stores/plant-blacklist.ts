import { defineStore } from 'pinia'
import { ref } from 'vue'
import { friendApi } from '@/api'
import { isCurrentAccount } from '@/composables/useStaleGuard'
import { useAccountStore } from './account'

export interface PlantBlacklistItem {
  seedId: number
  name: string
}

export const usePlantBlacklistStore = defineStore('plant-blacklist', () => {
  const blacklist = ref<number[]>([])
  const loading = ref(false)
  let fetchRequestId = 0

  function clearBlacklistData() {
    blacklist.value = []
    loading.value = false
  }

  async function fetchBlacklist() {
    const accountStore = useAccountStore()
    const accountId = accountStore.currentAccountId
    if (!accountId)
      return
    const requestedId = String(accountId)
    const requestId = ++fetchRequestId
    loading.value = true
    try {
      const res = await friendApi.getPlantBlacklist()
      if (requestId !== fetchRequestId || !isCurrentAccount(requestedId))
        return
      if (res.data.ok) {
        blacklist.value = res.data.data || []
      }
    }
    catch { /* ignore */ }
    finally {
      if (requestId === fetchRequestId)
        loading.value = false
    }
  }

  async function addToBlacklist(seedId: number) {
    const accountStore = useAccountStore()
    const accountId = accountStore.currentAccountId
    if (!accountId)
      return
    const requestedId = String(accountId)
    const res = await friendApi.savePlantBlacklist({ seedId })
    if (isCurrentAccount(requestedId) && res.data.ok) {
      blacklist.value = res.data.data || []
    }
  }

  async function removeFromBlacklist(seedId: number) {
    const accountStore = useAccountStore()
    const accountId = accountStore.currentAccountId
    if (!accountId)
      return
    const requestedId = String(accountId)
    const res = await friendApi.deletePlantBlacklist(seedId)
    if (isCurrentAccount(requestedId) && res.data.ok) {
      blacklist.value = res.data.data || []
    }
  }

  async function toggleBlacklist(seedId: number) {
    if (blacklist.value.includes(seedId)) {
      await removeFromBlacklist(seedId)
    }
    else {
      await addToBlacklist(seedId)
    }
  }

  function isBlacklisted(seedId: number) {
    return blacklist.value.includes(seedId)
  }

  async function addAllToBlacklist(seedIds: number[]) {
    const accountStore = useAccountStore()
    const accountId = accountStore.currentAccountId
    if (!accountId)
      return
    const requestedId = String(accountId)
    const res = await friendApi.savePlantBlacklistBatch({ seedIds })
    if (isCurrentAccount(requestedId) && res.data.ok) {
      blacklist.value = res.data.data || []
    }
  }

  async function clearBlacklist() {
    const accountStore = useAccountStore()
    const accountId = accountStore.currentAccountId
    if (!accountId)
      return
    const requestedId = String(accountId)
    const res = await friendApi.clearPlantBlacklist()
    if (isCurrentAccount(requestedId) && res.data.ok) {
      blacklist.value = []
    }
  }

  return {
    blacklist,
    loading,
    clearBlacklistData,
    fetchBlacklist,
    addToBlacklist,
    removeFromBlacklist,
    toggleBlacklist,
    isBlacklisted,
    addAllToBlacklist,
    clearBlacklist,
  }
})
