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
  type?: string
  count?: number
  force?: boolean
  buyOrganic?: boolean
  buyNormal?: boolean
  organicCount?: number
  organicThresholdHours?: number
  normalCount?: number
  normalThresholdHours?: number
}

export function getStatus<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/status', config)
}

export function saveAutomation<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/automation', payload, config)
}

export function getLands<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/lands', config)
}

export function getSeeds<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/seeds', config)
}

export function operate<T = any>(payload: FarmOperationInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/farm/operate', payload, config)
}

export function fertilize<T = any>(payload: FertilizeInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/land/fertilize', payload, config)
}

export function removePlants<T = any>(payload: RemovePlantsInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/land/remove', payload, config)
}

export function removeAllPlants<T = any>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/land/remove-all', {}, config)
}

export function buyFertilizer<T = any>(payload: FertilizerPurchaseInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/fertilizer/buy', payload, config)
}

export function checkAndBuyFertilizer<T = any>(payload: FertilizerPurchaseInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/fertilizer/check-and-buy', payload, config)
}
