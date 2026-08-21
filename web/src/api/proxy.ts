import type { ApiBody, ApiEnvelope, ApiRequestConfig } from './types'
import client from './client'

export function proxy<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/proxy', payload, config)
}
