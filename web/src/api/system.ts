import type { ApiBody, ApiEnvelope, ApiRequestConfig } from './types'
import client from './client'

export function getLoginLinks<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/admin/login-links', config)
}

export function saveLoginLinks<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/login-links', payload, config)
}

export function resetLoginLinks<T = unknown>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/login-links/reset', {}, config)
}

export function getSystemConfig<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/admin/system-config', config)
}

export function saveSystemConfig<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/system-config', payload, config)
}

export function resetSystemConfig<T = unknown>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/system-config/reset', {}, config)
}

export function getWXConfig<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/admin/wx-config', config)
}

export function saveWXConfig<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/wx-config', payload, config)
}

export function saveAnnouncement<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/announcement', payload, config)
}

export function saveLoginLogo<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/login-logo', payload, config)
}

export function saveSuperAdminAnnouncement<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/super-admin/announcement', payload, config)
}

export function getSuperAdminAnnouncement<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/super-admin-announcement', config)
}

export function saveSuperAdminAnnouncementLegacy<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/super-admin-announcement', payload, config)
}

export function verifySuperAdminAnnouncement<T = unknown>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/super-admin-announcement/verify', payload, config)
}

export function getSuperAdminAntiResaleConfig<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/super-admin/anti-resale-config', config)
}

export function saveSuperAdminAntiResaleConfig<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/super-admin/anti-resale-config', payload, config)
}

export function checkAccountLimit<T = unknown>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/super-admin/check-account-limit', payload, config)
}

export function clearData<T = unknown>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/super-admin/clear-data', payload, config)
}

export function getSystemInfo<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/system/info', config)
}

export function getPublicLoginLinks<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/public/login-links', config)
}

export function getDebugItemConfig<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/debug/item-config', config)
}

export function getPublicValue<T = unknown>(name: 'user-count' | 'anti-resale-config' | 'changelog' | 'scheduler', config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>(`/api/${name}`, config)
}

export function getAnnouncement<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/announcement', config)
}

export function readAnnouncement<T = unknown>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/announcement/read', payload, config)
}

export function clearAnnouncement<T = unknown>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/announcement/clear', {}, config)
}

export function restartSystem<T = unknown>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/system/restart', {}, config)
}

export function saveSystemRuntimeConfig<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/system/config', payload, config)
}

export function getGameVersion<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/game-version', config)
}

export function validateAuth<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/auth/validate', config)
}
