import type { ApiBody, ApiEnvelope, ApiRequestConfig } from './types'
import client from './client'

export function createQR<T = any>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/qr/create', payload, config)
}

export function checkQR<T = any>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/qr/check', payload, config)
}
