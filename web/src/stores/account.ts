import { useStorage } from '@vueuse/core'
import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { accountApi } from '@/api'

export interface Account {
  id: string
  name: string
  nick?: string
  uin?: number
  qq?: string | number
  wxid?: string
  gid?: string | number
  openId?: string
  avatar?: string
  avatarUrl?: string
  username?: string
  platform?: string
  running?: boolean
  // Add other fields as discovered
}

export interface RefreshWxCodesResult {
  total: number
  success: number
  failed: number
  skipped: number
  results: Array<{
    accountId: string
    name: string
    ok: boolean
    error?: string
  }>
}

export interface AccountLog {
  time: string
  action: string
  msg: string
  reason?: string
}

export function getPlatformLabel(p?: string) {
  if (p === 'qq')
    return 'QQ'
  if (p === 'wx')
    return '微信'
  return ''
}

export function getPlatformClass(p?: string) {
  if (p === 'qq')
    return 'bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400'
  if (p === 'wx')
    return 'bg-green-100 text-green-600 dark:bg-green-900/30 dark:text-green-400'
  return ''
}

export const useAccountStore = defineStore('account', () => {
  const accounts = ref<Account[]>([])
  const currentAccountId = useStorage('current_account_id', '')
  const loading = ref(false)
  const logs = ref<AccountLog[]>([])

  function applyAccounts(nextAccounts: Account[]) {
    accounts.value = nextAccounts

    if (accounts.value.length === 0) {
      currentAccountId.value = ''
      return
    }

    const found = accounts.value.find(a => String(a.id) === currentAccountId.value)
    if (!found && accounts.value[0]) {
      currentAccountId.value = String(accounts.value[0].id)
    }
  }

  const currentAccount = computed(() =>
    accounts.value.find(a => String(a.id) === currentAccountId.value),
  )

  async function fetchAccounts() {
    loading.value = true
    try {
      // api interceptor adds x-admin-token
      const res = await accountApi.getAccounts()
      if (res.data.ok && Array.isArray(res.data.data)) {
        applyAccounts(res.data.data as Account[])
      }
      else {
        console.warn('[account] fetchAccounts returned unexpected payload, keeping previous account state')
      }
    }
    catch (e) {
      console.error('获取账号失败', e)
      // Keep the last usable account selection to avoid blanking the app on transient failures.
    }
    finally {
      loading.value = false
    }
  }

  function selectAccount(id: string) {
    currentAccountId.value = id
  }

  function setCurrentAccount(acc: Account) {
    selectAccount(acc.id)
  }

  async function startAccount(id: string) {
    await accountApi.startAccount(id)
    await fetchAccounts()
  }

  async function stopAccount(id: string) {
    await accountApi.stopAccount(id)
    await fetchAccounts()
  }

  async function refreshWxCodes() {
    const res = await accountApi.refreshWXCodes({ timeout: 120000 })
    await fetchAccounts()
    return res.data as { ok: boolean, error?: string, data?: RefreshWxCodesResult }
  }

  async function deleteAccount(id: string) {
    await accountApi.deleteAccount(id)
    if (currentAccountId.value === id) {
      currentAccountId.value = ''
    }
    await fetchAccounts()
  }

  async function fetchLogs() {
    try {
      const res = await accountApi.getAccountLogs(100)
      if (res.data.ok && Array.isArray(res.data.data)) {
        logs.value = res.data.data
      }
    }
    catch (e) {
      console.error('获取账号日志失败', e)
    }
  }

  async function addAccount(payload: any) {
    try {
      await accountApi.createAccount(payload)
      await fetchAccounts()
    }
    catch (e) {
      console.error('添加账号失败', e)
      throw e
    }
  }

  async function updateAccount(id: string, payload: any) {
    try {
      // The Go API uses POST /api/accounts for both add and update (if id is present).
      await accountApi.createAccount({ ...payload, id })
      await fetchAccounts()
    }
    catch (e) {
      console.error('更新账号失败', e)
      throw e
    }
  }

  return {
    accounts,
    currentAccountId,
    currentAccount,
    loading,
    logs,
    fetchAccounts,
    selectAccount,
    startAccount,
    stopAccount,
    refreshWxCodes,
    deleteAccount,
    fetchLogs,
    addAccount,
    updateAccount,
    setCurrentAccount,
  }
})
