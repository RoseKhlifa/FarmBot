import type { ApiEnvelope, ApiRequestConfig } from './types'
import client from './client'

export function getAnalytics<T = unknown>(sort?: string, config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/analytics', {
    ...config,
    params: { ...config?.params, ...(sort ? { sort } : {}) },
  })
}
