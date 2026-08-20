import type { ApiEnvelope, ApiRequestConfig } from './types'
import client from './client'

export function getLoginLogs<T = unknown>(limit = 100, offset = 0, config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/admin/login-logs', {
    ...config,
    params: { ...config?.params, limit, offset },
  })
}

export function clearLoginLogs<T = unknown>(config?: ApiRequestConfig) {
  return client.delete<ApiEnvelope<T>>('/api/admin/login-logs', config)
}
