import type { ApiEnvelope, ApiRequestConfig } from './types'
import client from './client'

export function getIllustrated<T = unknown>(refresh = false, illustratedType = 1, config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/illustrated', {
    ...config,
    params: { ...config?.params, refresh, illustrated_type: illustratedType },
  })
}

export function buyIllustrated<T = unknown>(payload: Record<string, unknown> = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/illustrated/buy', payload, config)
}

export function buyAllIllustrated<T = unknown>(illustratedType = 1, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/illustrated/buy-all', { illustrated_type: illustratedType }, config)
}

export const getIllustratedList = getIllustrated
