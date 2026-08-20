import type { ApiEnvelope, ApiRequestConfig } from './types'
import client from './client'

export function getCareer<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/career', config)
}
