import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { bagApi } from '@/api'
import { useAccountStore } from '@/stores/account'

export const useBagStore = defineStore('bag', () => {
  const allItems = ref<any[]>([])
  const originalItems = ref<any[]>([])
  const loading = ref(false)

  function clearBag() {
    allItems.value = []
    originalItems.value = []
  }

  const items = computed(() => {
    const hiddenIds = new Set([1, 1001, 1002, 1101, 1011, 1012, 3001, 3002])
    return allItems.value.filter((it: any) => !hiddenIds.has(Number(it.id || 0)))
  })

  const dashboardItems = computed(() => {
    const targetIds = new Set([1011, 1012, 3001, 3002])
    return allItems.value.filter((it: any) => targetIds.has(Number(it.id || 0)))
  })

  async function fetchBag(accountId: string) {
    if (!accountId)
      return
    const requestedId = accountId
    loading.value = true
    try {
      const res = await bagApi.getBag()
      const acc = useAccountStore()
      const curId = String((acc.currentAccountId as { value?: string })?.value ?? acc.currentAccountId ?? '')
      if (curId !== requestedId)
        return
      if (res.data.ok && res.data.data) {
        allItems.value = Array.isArray(res.data.data.items) ? res.data.data.items : []
        originalItems.value = Array.isArray(res.data.data.originalItems) ? res.data.data.originalItems : []
      }
      else if (res.data && res.data.ok === false && res.data.error) {
        allItems.value = []
        originalItems.value = []
      }
    }
    catch (e) {
      const acc = useAccountStore()
      const curId = String((acc.currentAccountId as { value?: string })?.value ?? acc.currentAccountId ?? '')
      if (curId === requestedId) {
        allItems.value = []
        originalItems.value = []
      }
      console.error(e)
    }
    finally {
      loading.value = false
    }
  }

  async function useItem(_accountId: string, itemId: number, count = 1) {
    const res = await bagApi.useItem({ itemId, count })
    return res.data
  }

  async function sellItems(_accountId: string, items: Array<{ id: number, count: number, uid?: number }>) {
    const res = await bagApi.sellItems({ items })
    return res.data
  }

  return { items, allItems, originalItems, dashboardItems, loading, fetchBag, clearBag, useItem, sellItems }
})
