import type { ApiBody, ApiEnvelope, ApiRequestConfig } from './types'
import client from './client'

export interface FarmOperationInput {
  operation: string
}

export interface FertilizeInput {
  landIds: number[]
  fertilizerId: number
}

export interface RemovePlantsInput {
  landIds: number[]
}

export interface FertilizerPurchaseInput {
  type: string
  count: number
  force?: boolean
}

export function getStatus<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/status', config)
}

export function saveAutomation<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/automation', payload, config)
}

export function getLands<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/lands', config)
}

export function operate<T = unknown>(payload: FarmOperationInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/farm/operate', payload, config)
}

export function fertilize<T = unknown>(payload: FertilizeInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/land/fertilize', payload, config)
}

export function removePlants<T = unknown>(payload: RemovePlantsInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/land/remove', payload, config)
}

export function removeAllPlants<T = unknown>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/land/remove-all', {}, config)
}

export function buyFertilizer<T = unknown>(payload: FertilizerPurchaseInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/fertilizer/buy', payload, config)
}

export function checkAndBuyFertilizer<T = unknown>(payload: FertilizerPurchaseInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/fertilizer/check-and-buy', payload, config)
}
