import type { ApiBody, ApiEnvelope, ApiRequestConfig, Identifier } from './types'
import client from './client'
import { pathSegment } from './types'

export function listActivities<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/activity/list', config)
}

export function getActivityGroup<T = unknown>(id: Identifier, uid?: Identifier, config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>(`/api/activity/group/${pathSegment(id)}`, {
    ...config,
    params: { ...config?.params, ...(uid === undefined ? {} : { uid }) },
  })
}

export function getServerVersion<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/server-version', config)
}

export function getHelu<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/activity/helu', config)
}

export function drawHelu<T = unknown>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/activity/helu/draw', payload, config)
}

export function claimHeluPassport<T = unknown>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/activity/helu/passport/claim', {}, config)
}

export function claimHeluSolar<T = unknown>(payload: { id: number }, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/activity/helu/solar/claim', payload, config)
}

export function exchangeHelu<T = unknown>(payload: { goodsId: number, count: number }, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/activity/helu/exchange', payload, config)
}

export function claimQingmei<T = unknown>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/activity/qingmei/claim', {}, config)
}

export function sellQingmeiWine<T = unknown>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/activity/qingmei/wine/sell', payload, config)
}

export function getGuanxing<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/activity/guanxing', config)
}

export function claimGuanxing<T = unknown>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/activity/guanxing/claim', {}, config)
}

export function getActivityShop<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/activity/shop', config)
}

export function buyActivityShop<T = unknown>(payload: { slotId: number, count: number }, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/activity/shop/buy', payload, config)
}

export function refreshActivityShop<T = unknown>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/activity/shop/refresh', {}, config)
}

export const getHeluActivity = getHelu
export const claimHeluSolarReward = claimHeluSolar
export const claimQingmeiSeeds = claimQingmei
export const brewAndSellQingmeiWine = sellQingmeiWine
