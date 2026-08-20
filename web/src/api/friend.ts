import type { ApiBody, ApiEnvelope, ApiRequestConfig, Identifier } from './types'
import client from './client'
import { pathSegment } from './types'

export interface FriendOperationInput { opType: string, landIds?: number[] }
export interface FriendApplyInput { gid?: number, uid?: number, openid?: string, shareKey?: string, token?: string }
export interface KnownFriendGidsInput { gid?: number, gids?: number[] }
export interface PlantBlacklistInput { seedId?: number, seedIds?: number[] }

export function getFriends<T = unknown>(forceSync = false, config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/friends', {
    ...config,
    params: { ...config?.params, ...(forceSync ? { forceSync: 'true' } : {}) },
  })
}

export function clearFriendCache<T = unknown>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/friends/clear-cache', {}, config)
}

export function fetchDogInfo<T = unknown>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/friends/fetch-dog-info', {}, config)
}

export function getInteractRecords<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/interact-records', config)
}

export function getFriendLands<T = unknown>(gid: Identifier, config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>(`/api/friend/${pathSegment(gid)}/lands`, config)
}

export function operateFriend<T = unknown>(gid: Identifier, payload: FriendOperationInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>(`/api/friend/${pathSegment(gid)}/op`, payload, config)
}

export function getFriendDog<T = unknown>(gid: Identifier, config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>(`/api/friend/${pathSegment(gid)}/dog`, config)
}

export function batchDeleteFriends<T = unknown>(payload: { gids: number[], password?: string }, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/friend/batch-delete', payload, config)
}

export function deleteFriend<T = unknown>(gid: Identifier, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>(`/api/friend/${pathSegment(gid)}/delete`, {}, config)
}

export function applyFriend<T = unknown>(payload: FriendApplyInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/friend/apply', payload, config)
}

export function getBlacklist<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/friend-blacklist', config)
}

export function toggleBlacklist<T = unknown>(payload: { gid: number, reason?: string, blocked?: boolean }, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/friend-blacklist/toggle', payload, config)
}

export function updateBlacklist<T = unknown>(payload: { gid: number, reason?: string, skipSteal?: boolean, skipHelp?: boolean }, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/friend-blacklist/update', payload, config)
}

export function getKnownFriendGids<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/friend-known-gids', config)
}

export function saveKnownFriendGids<T = unknown>(payload: KnownFriendGidsInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/friend-known-gids', payload, config)
}

export function removeKnownFriendGid<T = unknown>(payload: { gid: number }, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/friend-known-gids/remove', payload, config)
}

export function addKnownFriendGids<T = unknown>(payload: { gids: number[] }, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/friend-known-gids/batch-add', payload, config)
}

export function removeKnownFriendGids<T = unknown>(payload: { gids: number[] }, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/friend-known-gids/batch-remove', payload, config)
}

export function getDogGifts<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/dog/gifts', config)
}

export function claimDogGifts<T = unknown>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/dog/gifts/claim', {}, config)
}

export function getPlantBlacklist<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/plant-blacklist', config)
}

export function savePlantBlacklist<T = unknown>(payload: PlantBlacklistInput | ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/plant-blacklist', payload, config)
}

export function deletePlantBlacklist<T = unknown>(seedId: Identifier, config?: ApiRequestConfig) {
  return client.delete<ApiEnvelope<T>>(`/api/plant-blacklist/${pathSegment(seedId)}`, config)
}

export function savePlantBlacklistBatch<T = unknown>(payload: PlantBlacklistInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/plant-blacklist/batch', payload, config)
}

export function clearPlantBlacklist<T = unknown>(config?: ApiRequestConfig) {
  return client.delete<ApiEnvelope<T>>('/api/plant-blacklist', config)
}
