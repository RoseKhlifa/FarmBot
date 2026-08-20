import type { ApiEnvelope, ApiRequestConfig } from './types'
import client from './client'

export interface WarehouseItemInput {
  id: number
  count: number
  uid?: number
}

export interface UseItemInput {
  itemId?: number
  count?: number
  uid?: number
  landIds?: number[]
  items?: WarehouseItemInput[]
}

export interface SellItemsInput {
  items?: WarehouseItemInput[]
}

export function getBag<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/bag', config)
}

export function useItem<T = unknown>(payload: UseItemInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/bag/use', payload, config)
}

export function sellItems<T = unknown>(payload: SellItemsInput = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/bag/sell', payload, config)
}

export function getBagSeeds<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/bag/seeds', config)
}

export function getDailyGifts<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/daily-gifts', config)
}
