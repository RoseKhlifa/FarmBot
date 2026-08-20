import type { ApiBody, ApiEnvelope, ApiRequestConfig } from './types'
import client from './client'

export function getAccounts<T = unknown>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/yyb/accounts', payload, config)
}

export function getCode<T = unknown>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/yyb/getcode', payload, config)
}

export function getThirdPartyCode<T = unknown>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/yyb/thirdparty-code', payload, config)
}

export function createQR<T = unknown>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/yyb/qr/create', payload, config)
}

export function pollQR<T = unknown>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/yyb/qr/poll', payload, config)
}

export function confirmQR<T = unknown>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/yyb/qr/confirm', payload, config)
}
