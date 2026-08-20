import type { ApiBody, ApiEnvelope, ApiRequestConfig } from './types'
import client from './client'
import { pathSegment } from './types'

export function getCardInfo<T = unknown>(code: string, config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>(`/api/card/info/${pathSegment(code)}`, config)
}

export function getCards<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/admin/cards', config)
}

export function createCard<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/cards', payload, config)
}

export function deleteCard<T = unknown>(code: string, config?: ApiRequestConfig) {
  return client.delete<ApiEnvelope<T>>(`/api/admin/cards/${pathSegment(code)}`, config)
}

export function updateCard<T = unknown>(code: string, payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>(`/api/admin/cards/${pathSegment(code)}`, payload, config)
}

export function batchDeleteCards<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/cards/batch-delete', payload, config)
}

export function batchRenewCards<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/cards/batch-renew', payload, config)
}

export function getClaimStatus<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/card-claim/status', config)
}

export function claimCard<T = unknown>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/card-claim/claim', payload, config)
}

export function updateClaimStatus<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/card-claim/status', payload, config)
}

export function getAdminClaimStatus<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/admin/card-claim/status', config)
}

export function getClaimRecords<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/admin/card-claim/records', config)
}
