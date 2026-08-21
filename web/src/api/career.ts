import type { ApiEnvelope, ApiRequestConfig } from './types'
import client from './client'

export function getCareer<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/career', config)
}
