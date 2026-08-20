import type { ApiBody, ApiEnvelope, ApiRequestConfig, Identifier } from './types'
import client from './client'
import { pathSegment } from './types'

export interface AccountRemarkInput {
  accountId?: string
  remark: string
}

export function getAccounts<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/accounts', config)
}

export function createAccount<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/accounts', payload, config)
}

export function refreshWXCodes<T = unknown>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/accounts/refresh-wx-codes', {}, config)
}

export function setAccountRemark<T = unknown>(payload: AccountRemarkInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/account/remark', payload, config)
}

export function deleteAccount<T = unknown>(id: Identifier, config?: ApiRequestConfig) {
  return client.delete<ApiEnvelope<T>>(`/api/accounts/${pathSegment(id)}`, config)
}

export function startAccount<T = unknown>(id: Identifier, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>(`/api/accounts/${pathSegment(id)}/start`, {}, config)
}

export function stopAccount<T = unknown>(id: Identifier, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>(`/api/accounts/${pathSegment(id)}/stop`, {}, config)
}

export function getAccountLogs<T = unknown>(limit = 100, config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/account-logs', {
    ...config,
    params: { ...config?.params, limit },
  })
}

export function getLogs<T = unknown>(limit = 100, config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/logs', {
    ...config,
    params: { ...config?.params, limit },
  })
}

export function clearLogs<T = unknown>(config?: ApiRequestConfig) {
  return client.delete<ApiEnvelope<T>>('/api/logs', config)
}
