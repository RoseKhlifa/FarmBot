import { useStorage } from '@vueuse/core'
import axios, { AxiosHeaders } from 'axios'
import { useToastStore } from '@/stores/toast'

const tokenRef = useStorage('admin_token', '')
const accountIdRef = useStorage('current_account_id', '')

const apiClient = axios.create({
  baseURL: '/',
  timeout: 20000,
})

let lastNetworkToastAt = 0

function showNetworkToast(message: string) {
  const now = Date.now()
  if (now - lastNetworkToastAt < 5000)
    return
  lastNetworkToastAt = now
  useToastStore().error(message)
}

apiClient.interceptors.request.use((config) => {
  const headers = AxiosHeaders.from(config.headers)

  // Legacy callers may still supply these headers while P8 consumers migrate.
  // Always replace them so storage remains the only source of request identity.
  headers.delete('x-admin-token')
  headers.delete('x-account-id')

  const token = tokenRef.value.trim()
  if (token)
    headers.set('x-admin-token', token)

  const accountId = accountIdRef.value.trim()
  if (accountId)
    headers.set('x-account-id', accountId)

  config.headers = headers
  return config
}, error => Promise.reject(error))

apiClient.interceptors.response.use(response => response, (error) => {
  if (axios.isCancel(error) || error?.code === 'ERR_CANCELED')
    return Promise.reject(error)

  const toast = useToastStore()

  if (error.response) {
    if (error.response.status === 401) {
      if (!window.location.pathname.includes('/login')) {
        tokenRef.value = ''
        window.location.href = '/login'
        toast.warning('登录已过期，请重新登录')
      }
    }
    else if (error.response.status >= 500) {
      const backendError = String(error.response.data?.error || error.response.data?.message || '')
      // Stopped accounts do not have a game runtime yet. Read-only callers
      // may still race account startup, so do not present that expected state
      // as a global server fault.
      const accountOffline = backendError === '账号未运行'
        || backendError.includes(' is offline')
        || backendError.includes('账号离线')
        || backendError.includes('账号未启动')
      if (accountOffline || backendError === 'API Timeout' || backendError === 'Request Timeout')
        return Promise.reject(error)
      toast.error(`服务器错误 ${error.response.status} ${error.response.statusText}`)
    }
    else {
      toast.error('请求失败，请联系管理员')
    }
  }
  else if (error.request) {
    if (error.code === 'ECONNABORTED')
      showNetworkToast('请求超时，请稍后重试')
    else
      showNetworkToast('网络错误，无法连接到服务器')
  }
  else {
    toast.error(`错误: ${error.message}`)
  }

  return Promise.reject(error)
})

export default apiClient
