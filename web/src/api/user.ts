import type { ApiBody, ApiEnvelope, ApiRequestConfig, Identifier } from './types'
import client from './client'
import { pathSegment } from './types'

export interface LoginInput { username: string, password: string }
export interface RegisterInput { username: string, password: string, cardCode: string }

export function login<T = unknown>(payload: LoginInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/login', payload, config)
}

export function register<T = unknown>(payload: RegisterInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/register', payload, config)
}

// Kept for callers during the migration; the Go handler will consume this route when auth teardown is wired.
export function logout<T = unknown>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/logout', {}, config)
}

export function getCurrentUser<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/user/me', config)
}

export function renewUser<T = unknown>(cardCode: string, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/user/renew', { cardCode }, config)
}

export function publicRenew<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/public/renew', payload, config)
}

export function renewLegacy<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/renew', payload, config)
}

export function changePassword<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/user/change-password', payload, config)
}

export function verifyPasswordReset<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/public/reset-password/verify', payload, config)
}

export function confirmPasswordReset<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/public/reset-password/confirm', payload, config)
}

export function getWXLoginConfig<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/user/wxlogin-config', config)
}

export function saveWXLoginConfig<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/user/wxlogin-config', payload, config)
}

export function getDeviceProtocol<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/user/device-protocol', config)
}

export function saveDeviceProtocol<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/user/device-protocol', payload, config)
}

export function getAdminUsers<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/admin/users', config)
}

export function getAdminUsersWithPassword<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/admin/users-with-password', config)
}

export function clearExpiredUsers<T = unknown>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/users/clear-expired', payload, config)
}

export function updateAdminUser<T = unknown>(username: string, payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>(`/api/admin/users/${pathSegment(username)}`, payload, config)
}

export function editAdminUser<T = unknown>(username: string, payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>(`/api/admin/users/${pathSegment(username)}/edit`, payload, config)
}

export function renewAdminUser<T = unknown>(username: string, cardCode: string, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>(`/api/admin/users/${pathSegment(username)}/renew`, { cardCode }, config)
}

export function deleteAdminUser<T = unknown>(username: string, config?: ApiRequestConfig) {
  return client.delete<ApiEnvelope<T>>(`/api/admin/users/${pathSegment(username)}`, config)
}

export type UserIdentifier = Identifier
