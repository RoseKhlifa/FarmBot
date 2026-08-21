import type { ApiEnvelope, ApiRequestConfig } from './types'
import client from './client'

export interface SeedPurchaseInput {
  goodsId: number
  count: number
  price: number
}

export interface MallPurchaseInput {
  goodsId: number
  count: number
}

export interface MysteryPurchaseInput {
  npcId: number
}

export function getSeeds<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/seeds', config)
}

export function getSeedShop<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/shop/seed', config)
}

export function buySeed<T = any>(payload: SeedPurchaseInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/shop/buy', payload, config)
}

export function getPetShop<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/shop/pet', config)
}

export function getDecorationShop<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/shop/decoration', config)
}

export function getMall<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/shop/mall', config)
}

export function buyMallGoods<T = any>(payload: MallPurchaseInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/shop/mall/buy', payload, config)
}

export function getMysteryShop<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/shop/mystery', config)
}

export function buyMysteryGoods<T = any>(payload: MysteryPurchaseInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/shop/mystery/buy', payload, config)
}

export function abandonMysteryShop<T = any>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/shop/mystery/abandon', {}, config)
}
