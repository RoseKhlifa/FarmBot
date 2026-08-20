import type { ApiEnvelope, ApiRequestConfig } from './types'
import client from './client'

export interface SeedPurchaseInput {
  goodsId: number
  num: number
  price: number
}

export interface MallPurchaseInput {
  goodsId: number
  count: number
}

export interface MysteryPurchaseInput {
  npcId: number
}

export function getSeeds<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/seeds', config)
}

export function getSeedShop<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/shop/seed', config)
}

export function buySeed<T = unknown>(payload: SeedPurchaseInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/shop/buy', payload, config)
}

export function getPetShop<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/shop/pet', config)
}

export function getDecorationShop<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/shop/decoration', config)
}

export function getMall<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/shop/mall', config)
}

export function buyMallGoods<T = unknown>(payload: MallPurchaseInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/shop/mall/buy', payload, config)
}

export function getMysteryShop<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/shop/mystery', config)
}

export function buyMysteryGoods<T = unknown>(payload: MysteryPurchaseInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/shop/mystery/buy', payload, config)
}

export function abandonMysteryShop<T = unknown>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/shop/mystery/abandon', {}, config)
}
