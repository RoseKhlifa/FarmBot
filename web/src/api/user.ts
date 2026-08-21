import type { ApiBody, ApiEnvelope, ApiRequestConfig, Identifier } from './types'
import client from './client'
import { pathSegment } from './types'

export interface LoginInput { username: string, password: string }
export interface RegisterInput { username: string, password: string, cardCode: string }

export function login<T = any>(payload: LoginInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/login', payload, config)
}

export function register<T = any>(payload: RegisterInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/register', payload, config)
}

// Kept for callers during the migration; the Go handler will consume this route when auth teardown is wired.
export function logout<T = any>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/logout', {}, config)
}

export function getCurrentUser<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/user/me', config)
}

export function renewUser<T = any>(cardCode: string, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/user/renew', { cardCode }, config)
}

export function publicRenew<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/public/renew', payload, config)
}

export function renewLegacy<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/renew', payload, config)
}

export function changePassword<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/user/change-password', payload, config)
}

export function verifyPasswordReset<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/public/reset-password/verify', payload, config)
}

export function confirmPasswordReset<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/public/reset-password/confirm', payload, config)
}

export function getWXLoginConfig<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/user/wxlogin-config', config)
}

export function saveWXLoginConfig<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/user/wxlogin-config', payload, config)
}

export function getDeviceProtocol<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/user/device-protocol', config)
}

export function saveDeviceProtocol<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/user/device-protocol', payload, config)
}

export function getAdminUsers<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/admin/users', config)
}

export function getAdminUsersWithPassword<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/admin/users-with-password', config)
}

export function clearExpiredUsers<T = any>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/users/clear-expired', payload, config)
}

export function updateAdminUser<T = any>(username: string, payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>(`/api/admin/users/${pathSegment(username)}`, payload, config)
}

export function editAdminUser<T = any>(username: string, payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>(`/api/admin/users/${pathSegment(username)}/edit`, payload, config)
}

export function renewAdminUser<T = any>(username: string, cardCode: string, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>(`/api/admin/users/${pathSegment(username)}/renew`, { cardCode }, config)
}

export function deleteAdminUser<T = any>(username: string, config?: ApiRequestConfig) {
  return client.delete<ApiEnvelope<T>>(`/api/admin/users/${pathSegment(username)}`, config)
}

export type UserIdentifier = Identifier
